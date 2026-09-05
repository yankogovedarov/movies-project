package db

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BackupKeep is how many snapshots are retained in the backup folder.
const BackupKeep = 10

const (
	backupDirName    = "backup"
	backupTimeLayout = "2006_01_02_15_04_05"
)

// Backup copies dbPath into <dir of dbPath>/backup/<timestamp>_<basename> and then
// deletes the oldest snapshots until at most keep of them remain. It returns the path
// of the created snapshot.
//
// If dbPath does not exist (first run) it returns ("", nil) without creating anything.
// The copy is a plain file copy: it is called before the database is opened, so no
// connection holds the file and there is no WAL sidecar to worry about.
func Backup(dbPath string, now time.Time, keep int) (string, error) {
	info, err := os.Stat(dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to stat database: %w", err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("database path %q is a directory", dbPath)
	}

	base := filepath.Base(dbPath)
	backupDir := filepath.Join(filepath.Dir(dbPath), backupDirName)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup dir: %w", err)
	}

	dest := filepath.Join(backupDir, now.Format(backupTimeLayout)+"_"+base)
	if err := copyFile(dbPath, dest); err != nil {
		return "", err
	}

	if err := pruneBackups(backupDir, base, keep); err != nil {
		return "", err
	}
	return dest, nil
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open database for backup: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("failed to write backup file: %w", err)
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return fmt.Errorf("failed to flush backup file: %w", err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("failed to close backup file: %w", err)
	}
	return nil
}

// pruneBackups deletes the oldest snapshots of base until at most keep remain.
// Only files this package created are considered — anything else in the folder is
// left alone.
func pruneBackups(backupDir, base string, keep int) error {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return fmt.Errorf("failed to read backup dir: %w", err)
	}

	var managed []string
	for _, e := range entries {
		if !e.IsDir() && isBackupName(e.Name(), base) {
			managed = append(managed, e.Name())
		}
	}
	if keep < 0 {
		keep = 0
	}
	if len(managed) <= keep {
		return nil
	}

	// The zero-padded timestamp layout sorts lexicographically = chronologically.
	sort.Strings(managed)
	for _, name := range managed[:len(managed)-keep] {
		if err := os.Remove(filepath.Join(backupDir, name)); err != nil {
			return fmt.Errorf("failed to prune old backup %s: %w", name, err)
		}
	}
	return nil
}

// isBackupName reports whether name is "<timestamp>_<base>" with a valid timestamp.
func isBackupName(name, base string) bool {
	suffix := "_" + base
	if !strings.HasSuffix(name, suffix) {
		return false
	}
	stamp := strings.TrimSuffix(name, suffix)
	if _, err := time.Parse(backupTimeLayout, stamp); err != nil {
		return false
	}
	return true
}
