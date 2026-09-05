package db_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yankogovedarov/movie-tracker/internal/db"
)

func TestBackup_CreatesTimestampedCopy(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "movietracker.db")
	content := []byte("SQLite format 3\x00 payload")
	require.NoError(t, os.WriteFile(dbPath, content, 0644))

	now := time.Date(2026, 8, 24, 19, 7, 0, 0, time.UTC)
	got, err := db.Backup(dbPath, now, db.BackupKeep)
	require.NoError(t, err)

	want := filepath.Join(tmpDir, "backup", "2026_08_24_19_07_00_movietracker.db")
	assert.Equal(t, want, got)

	copied, err := os.ReadFile(want)
	require.NoError(t, err)
	assert.Equal(t, content, copied)

	// The source database must stay untouched.
	original, err := os.ReadFile(dbPath)
	require.NoError(t, err)
	assert.Equal(t, content, original)
}

func TestBackup_NoSourceFile(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "movietracker.db")

	got, err := db.Backup(dbPath, time.Now(), db.BackupKeep)
	assert.NoError(t, err)
	assert.Empty(t, got)

	_, statErr := os.Stat(filepath.Join(tmpDir, "backup"))
	assert.True(t, os.IsNotExist(statErr), "backup dir must not be created on first run")
}

func TestBackup_PrunesOldest(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "movietracker.db")
	require.NoError(t, os.WriteFile(dbPath, []byte("current"), 0644))

	backupDir := filepath.Join(tmpDir, "backup")
	require.NoError(t, os.MkdirAll(backupDir, 0755))

	// 12 existing snapshots: 2026_08_01_.. through 2026_08_12_..
	for day := 1; day <= 12; day++ {
		ts := time.Date(2026, 8, day, 10, 0, 0, 0, time.UTC).Format("2006_01_02_15_04_05")
		name := ts + "_movietracker.db"
		require.NoError(t, os.WriteFile(filepath.Join(backupDir, name), []byte("old"), 0644))
	}

	now := time.Date(2026, 8, 24, 19, 7, 0, 0, time.UTC)
	_, err := db.Backup(dbPath, now, 10)
	require.NoError(t, err)

	entries, err := os.ReadDir(backupDir)
	require.NoError(t, err)

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	assert.Len(t, names, 10)
	assert.Contains(t, names, "2026_08_24_19_07_00_movietracker.db")
	// The three oldest (01, 02, 03) are gone; 04 is the oldest survivor.
	assert.NotContains(t, names, "2026_08_01_10_00_00_movietracker.db")
	assert.NotContains(t, names, "2026_08_02_10_00_00_movietracker.db")
	assert.NotContains(t, names, "2026_08_03_10_00_00_movietracker.db")
	assert.Contains(t, names, "2026_08_04_10_00_00_movietracker.db")
	assert.Contains(t, names, "2026_08_12_10_00_00_movietracker.db")
}

func TestBackup_IgnoresUnrelatedFiles(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "movietracker.db")
	require.NoError(t, os.WriteFile(dbPath, []byte("current"), 0644))

	backupDir := filepath.Join(tmpDir, "backup")
	require.NoError(t, os.MkdirAll(backupDir, 0755))

	unrelated := []string{
		"notes.txt",
		"2020_01_01_00_00_00_other.db",
		"movietracker.db",
		"not_a_timestamp_movietracker.db",
	}
	for _, name := range unrelated {
		require.NoError(t, os.WriteFile(filepath.Join(backupDir, name), []byte("keep me"), 0644))
	}
	for day := 1; day <= 3; day++ {
		ts := time.Date(2026, 8, day, 10, 0, 0, 0, time.UTC).Format("2006_01_02_15_04_05")
		require.NoError(t, os.WriteFile(filepath.Join(backupDir, ts+"_movietracker.db"), []byte("old"), 0644))
	}

	now := time.Date(2026, 8, 24, 19, 7, 0, 0, time.UTC)
	// keep=1 → only the fresh snapshot may survive among managed files.
	_, err := db.Backup(dbPath, now, 1)
	require.NoError(t, err)

	for _, name := range unrelated {
		_, statErr := os.Stat(filepath.Join(backupDir, name))
		assert.NoError(t, statErr, "unrelated file %q must not be deleted", name)
	}
	_, statErr := os.Stat(filepath.Join(backupDir, "2026_08_24_19_07_00_movietracker.db"))
	assert.NoError(t, statErr)
	for day := 1; day <= 3; day++ {
		ts := time.Date(2026, 8, day, 10, 0, 0, 0, time.UTC).Format("2006_01_02_15_04_05")
		_, statErr := os.Stat(filepath.Join(backupDir, ts+"_movietracker.db"))
		assert.True(t, os.IsNotExist(statErr), "managed snapshot %s must be pruned", ts)
	}
}

func TestBackup_RealDatabaseIsUsable(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "movietracker.db")

	d, err := db.Open(dbPath)
	require.NoError(t, err)
	require.NoError(t, db.Migrate(d))
	require.NoError(t, d.Close())

	backupPath, err := db.Backup(dbPath, time.Now(), db.BackupKeep)
	require.NoError(t, err)
	require.NotEmpty(t, backupPath)

	restored, err := db.Open(backupPath)
	require.NoError(t, err)
	defer restored.Close()

	var tableName string
	row := restored.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='media'")
	require.NoError(t, row.Scan(&tableName))
	assert.Equal(t, "media", tableName)
}
