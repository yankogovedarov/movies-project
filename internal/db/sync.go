package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/yankogovedarov/movie-tracker/internal/scanner"
)

// SyncScanResults persists a scanner result set into the media table.
// It marks all existing rows off-disk first, then upserts each found file
// (which sets on_disk = 1). Files absent from the scan remain off-disk.
func SyncScanResults(database *sql.DB, files []scanner.VideoFile) error {
	q := New(database)
	ctx := context.Background()

	if err := q.MarkAllOffDisk(ctx); err != nil {
		return fmt.Errorf("mark all off disk: %w", err)
	}

	for _, f := range files {
		_, err := q.UpsertMedia(ctx, UpsertMediaParams{
			Filename:           f.Filename,
			FolderRelativePath: f.FolderRelativePath,
			FileSizeBytes:      f.SizeBytes,
		})
		if err != nil {
			return fmt.Errorf("upsert %q: %w", f.Filename, err)
		}
	}

	return nil
}
