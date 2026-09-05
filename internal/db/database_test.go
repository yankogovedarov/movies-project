package db_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yankogovedarov/movie-tracker/internal/db"
)

func TestOpen(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	d, err := db.Open(dbPath)
	assert.NoError(t, err)
	assert.NotNil(t, d)
	d.Close()
}

func TestMigrate(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	d, err := db.Open(dbPath)
	assert.NoError(t, err)
	defer d.Close()

	err = db.Migrate(d)
	assert.NoError(t, err)

	var tableName string
	row := d.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='media'")
	err = row.Scan(&tableName)
	assert.NoError(t, err)
	assert.Equal(t, "media", tableName)
}

func TestMigrate_HasForDeletionColumn(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	d, err := db.Open(dbPath)
	require.NoError(t, err)
	defer d.Close()
	require.NoError(t, db.Migrate(d))

	// The column exists and defaults to 0 (not marked for deletion).
	_, err = d.Exec(`INSERT INTO media (filename, folder_relative_path, file_size_bytes)
		VALUES ('Film.mkv', 'Films', 100)`)
	require.NoError(t, err)

	var forDeletion int64
	row := d.QueryRow("SELECT for_deletion FROM media WHERE filename = 'Film.mkv'")
	require.NoError(t, row.Scan(&forDeletion))
	assert.Equal(t, int64(0), forDeletion, "for_deletion must default to 0")
}
