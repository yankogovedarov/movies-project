package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/yankogovedarov/movie-tracker/internal/scanner"
)

// syncProgressEvery is how many files apart SyncScanResults logs its progress.
// A variable, so a test can lower it.
var syncProgressEvery = 500

// SyncScanResults persists a scanner result set into the media table.
// It marks all existing rows off-disk first, then upserts each found file
// (which sets on_disk = 1). Files absent from the scan remain off-disk.
//
// Everything runs in one transaction (Bug 34, Bug 28): SQLite then writes one
// journal and flushes the disk once instead of once per file, and a reader sees
// either the previous or the new state, never a half-synced one. On error
// nothing is written. A nil log means slog.Default().
func SyncScanResults(database *sql.DB, files []scanner.VideoFile, log *slog.Logger) error {
	if log == nil {
		log = slog.Default()
	}
	ctx := context.Background()
	start := time.Now()
	log.Info("db sync started", "files", len(files))

	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin sync transaction: %w", err)
	}
	// A no-op once the transaction is committed.
	defer func() { _ = tx.Rollback() }()

	q := New(database).WithTx(tx)
	if err := q.MarkAllOffDisk(ctx); err != nil {
		return fmt.Errorf("mark all off disk: %w", err)
	}

	every := syncProgressEvery
	for i, f := range files {
		_, err := q.UpsertMedia(ctx, UpsertMediaParams{
			Filename:           f.Filename,
			FolderRelativePath: f.FolderRelativePath,
			FileSizeBytes:      f.SizeBytes,
			FileCreatedAt:      sql.NullTime{Time: f.FileCreatedAt, Valid: !f.FileCreatedAt.IsZero()},
		})
		if err != nil {
			return fmt.Errorf("upsert %q: %w", f.Filename, err)
		}
		if done := i + 1; every > 0 && done%every == 0 {
			log.Info("db sync progress", "done", done, "total", len(files), "elapsed", time.Since(start))
		}
	}

	// On a slow disk the single COMMIT (journal write + flushes) is the
	// expensive step; this line tells a hang there from one in the upserts.
	log.Info("db sync committing", "files", len(files), "elapsed", time.Since(start))
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit sync transaction: %w", err)
	}
	log.Info("db sync committed", "files", len(files), "took", time.Since(start))
	return nil
}
