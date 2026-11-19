package db_test

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

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
	require.NoError(t, db.Migrate(d))
	t.Cleanup(func() { d.Close() })
	return d
}

func TestSyncScanResults_InsertsNewFiles(t *testing.T) {
	d := openTestDB(t)
	files := []scanner.VideoFile{
		{Filename: "movie1.mkv", FolderRelativePath: "ActionMovies", SizeBytes: 1024000},
		{Filename: "movie2.avi", FolderRelativePath: "Drama", SizeBytes: 2048000},
	}

	err := db.SyncScanResults(d, files, nil)
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
	err := db.SyncScanResults(d, files, nil)
	require.NoError(t, err)

	files = []scanner.VideoFile{
		{Filename: "movie.mkv", FolderRelativePath: "Drama", SizeBytes: 1024000},
	}
	err = db.SyncScanResults(d, files, nil)
	require.NoError(t, err)

	row := d.QueryRow("SELECT folder_relative_path FROM media WHERE filename = 'movie.mkv' AND file_size_bytes = 1024000")
	var folder string
	require.NoError(t, row.Scan(&folder))
	assert.Equal(t, "Drama", folder)
}

// Bug 30: a rescan keeps the three manual flags — UpsertMedia does not write
// them — also when the file has moved to another folder.
func TestSyncScanResults_KeepsFlags(t *testing.T) {
	d := openTestDB(t)

	files := []scanner.VideoFile{
		{Filename: "movie.mkv", FolderRelativePath: "ActionMovies", SizeBytes: 1024000},
	}
	require.NoError(t, db.SyncScanResults(d, files, nil))
	_, err := d.Exec("UPDATE media SET for_dnd = 1, for_deletion = 1, wrong_subs = 1 WHERE filename = 'movie.mkv'")
	require.NoError(t, err)

	files[0].FolderRelativePath = "Drama"
	require.NoError(t, db.SyncScanResults(d, files, nil))

	var forDnd, forDeletion, wrongSubs int
	row := d.QueryRow("SELECT for_dnd, for_deletion, wrong_subs FROM media WHERE filename = 'movie.mkv'")
	require.NoError(t, row.Scan(&forDnd, &forDeletion, &wrongSubs))
	assert.Equal(t, []int{1, 1, 1}, []int{forDnd, forDeletion, wrongSubs}, "a rescan must keep the manual flags")
}

func TestSyncScanResults_MarksRemovedFilesOffDisk(t *testing.T) {
	d := openTestDB(t)

	files := []scanner.VideoFile{
		{Filename: "movie_a.mkv", FolderRelativePath: "ActionMovies", SizeBytes: 1024000},
		{Filename: "movie_b.avi", FolderRelativePath: "Drama", SizeBytes: 2048000},
	}
	err := db.SyncScanResults(d, files, nil)
	require.NoError(t, err)

	files = []scanner.VideoFile{
		{Filename: "movie_a.mkv", FolderRelativePath: "ActionMovies", SizeBytes: 1024000},
	}
	err = db.SyncScanResults(d, files, nil)
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
	err := db.SyncScanResults(d, files, nil)
	require.NoError(t, err)

	err = db.SyncScanResults(d, []scanner.VideoFile{}, nil)
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

	err := db.SyncScanResults(d, files, nil)
	require.NoError(t, err)

	err = db.SyncScanResults(d, files, nil)
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

// Bug 34 + Bug 28: the whole sync is one transaction. A failure part-way leaves
// the previous state intact instead of a half-written one: the earlier files
// stay on disk and none of the new rows is written.
func TestSyncScanResults_FailureRollsBack(t *testing.T) {
	d := openTestDB(t)
	require.NoError(t, db.SyncScanResults(d, []scanner.VideoFile{
		{Filename: "old.mkv", FolderRelativePath: "Films", SizeBytes: 100},
	}, nil))

	// A test-only trigger makes the upsert of boom.mkv fail after new.mkv has
	// already been written by the same sync.
	_, err := d.Exec(`CREATE TRIGGER boom BEFORE INSERT ON media
		WHEN NEW.filename = 'boom.mkv'
		BEGIN SELECT RAISE(ABORT, 'boom'); END;`)
	require.NoError(t, err)

	err = db.SyncScanResults(d, []scanner.VideoFile{
		{Filename: "new.mkv", FolderRelativePath: "Films", SizeBytes: 200},
		{Filename: "boom.mkv", FolderRelativePath: "Films", SizeBytes: 300},
	}, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "boom.mkv")

	var onDisk int
	require.NoError(t, d.QueryRow("SELECT on_disk FROM media WHERE filename = 'old.mkv'").Scan(&onDisk))
	assert.Equal(t, 1, onDisk, "a failed sync must not mark the earlier files off-disk")

	var newRows int
	require.NoError(t, d.QueryRow("SELECT COUNT(*) FROM media WHERE filename = 'new.mkv'").Scan(&newRows))
	assert.Equal(t, 0, newRows, "a failed sync must not write any of its files")
}

// Bug 34: a page request may be reading the database while a rescan commits.
// The commit waits for the reader (busy_timeout) instead of failing at once
// with "database is locked".
func TestSyncScanResults_WaitsForOpenReader(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	d, err := db.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })
	require.NoError(t, db.Migrate(d))
	require.NoError(t, db.SyncScanResults(d, []scanner.VideoFile{
		{Filename: "old.mkv", FolderRelativePath: "Films", SizeBytes: 100},
	}, nil))

	// A second pool on the same file, as another request would read it. An open
	// read transaction in the middle of its SELECT holds a SHARED lock, which
	// keeps any writer from committing.
	reader, err := db.Open(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = reader.Close() })
	ctx := context.Background()
	tx, err := reader.BeginTx(ctx, nil)
	require.NoError(t, err)
	rows, err := tx.QueryContext(ctx, "SELECT filename FROM media")
	require.NoError(t, err)
	require.True(t, rows.Next(), "the reader must be in the middle of its SELECT")

	releasing := make(chan struct{})
	released := make(chan struct{})
	go func() {
		defer close(released)
		time.Sleep(500 * time.Millisecond)
		// Signalled before the lock goes away, so the sync cannot finish
		// before it.
		close(releasing)
		_ = rows.Close()
		_ = tx.Rollback()
	}()
	t.Cleanup(func() { <-released })

	err = db.SyncScanResults(d, []scanner.VideoFile{
		{Filename: "new.mkv", FolderRelativePath: "Films", SizeBytes: 200},
	}, nil)
	require.NoError(t, err, "the sync must wait for the reader, not fail")

	select {
	case <-releasing:
	default:
		t.Fatal("the sync finished while the reader still held its lock: the test does not exercise the contention")
	}

	var oldOnDisk, newOnDisk int
	require.NoError(t, d.QueryRow("SELECT on_disk FROM media WHERE filename = 'old.mkv'").Scan(&oldOnDisk))
	require.NoError(t, d.QueryRow("SELECT on_disk FROM media WHERE filename = 'new.mkv'").Scan(&newOnDisk))
	assert.Equal(t, []int{0, 1}, []int{oldOnDisk, newOnDisk}, "the sync must be committed")
}

// Bug 34: the sync logs its start, its progress every syncProgressEvery files,
// the start of its commit and the commit itself, so a slow disk shows up in the
// log instead of a silent hang.
func TestSyncScanResults_LogsProgress(t *testing.T) {
	saved := *db.SyncProgressEvery
	*db.SyncProgressEvery = 2
	t.Cleanup(func() { *db.SyncProgressEvery = saved })

	d := openTestDB(t)
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))

	files := make([]scanner.VideoFile, 5)
	for i := range files {
		files[i] = scanner.VideoFile{
			Filename:           fmt.Sprintf("movie%d.mkv", i),
			FolderRelativePath: "Films",
			SizeBytes:          int64(1000 + i),
		}
	}
	require.NoError(t, db.SyncScanResults(d, files, logger))
	out := buf.String()

	started := logLines(out, "db sync started")
	require.Len(t, started, 1, out)
	assert.Equal(t, "5", logAttr(started[0], "files"))

	progress := logLines(out, "db sync progress")
	require.Len(t, progress, 2, "a progress line after the 2nd and the 4th file only:\n%s", out)
	assert.Equal(t, "2", logAttr(progress[0], "done"))
	assert.Equal(t, "4", logAttr(progress[1], "done"))
	for _, line := range progress {
		assert.Equal(t, "5", logAttr(line, "total"))
		assert.NotEmpty(t, logAttr(line, "elapsed"))
	}

	// On a slow disk the single COMMIT is the expensive step: a line right
	// before it tells "stuck in the upserts" from "stuck in the commit".
	committing := logLines(out, "db sync committing")
	require.Len(t, committing, 1, out)
	assert.Equal(t, "5", logAttr(committing[0], "files"))
	assert.NotEmpty(t, logAttr(committing[0], "elapsed"))
	assert.Less(t, strings.Index(out, progress[1]), strings.Index(out, committing[0]),
		"committing must follow the upserts:\n%s", out)

	committed := logLines(out, "db sync committed")
	require.Len(t, committed, 1, out)
	assert.Less(t, strings.Index(out, committing[0]), strings.Index(out, committed[0]),
		"committed must follow committing:\n%s", out)
	assert.Equal(t, "5", logAttr(committed[0], "files"))
	assert.NotEmpty(t, logAttr(committed[0], "took"))
}

// logLines returns the lines of a slog text output whose message is msg.
func logLines(out, msg string) []string {
	var lines []string
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, `msg="`+msg+`"`) {
			lines = append(lines, line)
		}
	}
	return lines
}

// logAttr returns the value of the key attribute of a slog text line, or ""
// when the line has no such attribute.
func logAttr(line, key string) string {
	m := regexp.MustCompile(`(?:^| )` + regexp.QuoteMeta(key) + `=(\S+)`).FindStringSubmatch(line)
	if m == nil {
		return ""
	}
	return m[1]
}
