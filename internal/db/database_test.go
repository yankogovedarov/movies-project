package db_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
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
