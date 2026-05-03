package db_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yankogovedarov/movie-tracker/internal/db"
	"github.com/yankogovedarov/movie-tracker/internal/scanner"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	tmpDir := t.TempDir()
	d, err := db.Open(filepath.Join(tmpDir, "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { d.Close() })
	require.NoError(t, db.Migrate(d))
	return d
}

func TestSyncScanResults_InsertsNewFiles(t *testing.T) {
	d := openTestDB(t)
	files := []scanner.VideoFile{
		{Filename: "movie1.mkv", FolderRelativePath: "ActionMovies", SizeBytes: 1024000},
		{Filename: "movie2.avi", FolderRelativePath: "Drama", SizeBytes: 2048000},
	}

	err := db.SyncScanResults(d, files)
	require.NoError(t, err)

	q := db.New(d)
	count, err := q.CountOnDisk(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestSyncScanResults_UpsertUpdatesFolder(t *testing.T) {
	d := openTestDB(t)

	files := []scanner.VideoFile{
		{Filename: "movie.mkv", FolderRelativePath: "ActionMovies", SizeBytes: 1024000},
	}
	err := db.SyncScanResults(d, files)
	require.NoError(t, err)

	files = []scanner.VideoFile{
		{Filename: "movie.mkv", FolderRelativePath: "Drama", SizeBytes: 1024000},
	}
	err = db.SyncScanResults(d, files)
	require.NoError(t, err)

	row := d.QueryRow("SELECT folder_relative_path FROM media WHERE filename = 'movie.mkv' AND file_size_bytes = 1024000")
	var folder string
	require.NoError(t, row.Scan(&folder))
	assert.Equal(t, "Drama", folder)
}

func TestSyncScanResults_MarksRemovedFilesOffDisk(t *testing.T) {
	d := openTestDB(t)

	files := []scanner.VideoFile{
		{Filename: "movie_a.mkv", FolderRelativePath: "ActionMovies", SizeBytes: 1024000},
		{Filename: "movie_b.avi", FolderRelativePath: "Drama", SizeBytes: 2048000},
	}
	err := db.SyncScanResults(d, files)
	require.NoError(t, err)

	files = []scanner.VideoFile{
		{Filename: "movie_a.mkv", FolderRelativePath: "ActionMovies", SizeBytes: 1024000},
	}
	err = db.SyncScanResults(d, files)
	require.NoError(t, err)

	row := d.QueryRow("SELECT on_disk FROM media WHERE filename = 'movie_b.avi'")
	var onDisk int
	require.NoError(t, row.Scan(&onDisk))
	assert.Equal(t, 0, onDisk)

	row = d.QueryRow("SELECT on_disk FROM media WHERE filename = 'movie_a.mkv'")
	require.NoError(t, row.Scan(&onDisk))
	assert.Equal(t, 1, onDisk)
}

func TestSyncScanResults_EmptyScanMarksAllOffDisk(t *testing.T) {
	d := openTestDB(t)

	files := []scanner.VideoFile{
		{Filename: "movie.mkv", FolderRelativePath: "ActionMovies", SizeBytes: 1024000},
	}
	err := db.SyncScanResults(d, files)
	require.NoError(t, err)

	err = db.SyncScanResults(d, []scanner.VideoFile{})
	require.NoError(t, err)

	q := db.New(d)
	count, err := q.CountOnDisk(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestSyncScanResults_Idempotent(t *testing.T) {
	d := openTestDB(t)

	files := []scanner.VideoFile{
		{Filename: "movie1.mkv", FolderRelativePath: "ActionMovies", SizeBytes: 1024000},
		{Filename: "movie2.avi", FolderRelativePath: "Drama", SizeBytes: 2048000},
	}

	err := db.SyncScanResults(d, files)
	require.NoError(t, err)

	err = db.SyncScanResults(d, files)
	require.NoError(t, err)

	q := db.New(d)
	count, err := q.CountOnDisk(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)

	row := d.QueryRow("SELECT COUNT(*) FROM media")
	var totalRows int
	require.NoError(t, row.Scan(&totalRows))
	assert.Equal(t, 2, totalRows)
}
