package db_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yankogovedarov/movie-tracker/internal/db"
	"github.com/yankogovedarov/movie-tracker/internal/scanner"
)

// Bug 27: the stats queries used to LEFT JOIN start_events and status_changes
// together (for the removed marked_at), so the start count was multiplied by
// the number of status changes — "Стартирана 4 пъти" for two starts. With the
// join gone the count must stay exact.
func TestMediaStats_StartCountIgnoresStatusChanges(t *testing.T) {
	d := openTestDB(t)
	require.NoError(t, db.SyncScanResults(d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	}))
	ctx := context.Background()
	q := db.New(d)
	media, err := q.ListOnDiskMedia(ctx)
	require.NoError(t, err)
	require.Len(t, media, 1)
	id := media[0].ID

	for i := 0; i < 2; i++ {
		require.NoError(t, q.InsertStartEvent(ctx, id))
	}
	for _, ch := range []struct{ from, to string }{{"new", "started"}, {"started", "new"}} {
		require.NoError(t, q.InsertStatusChange(ctx, db.InsertStatusChangeParams{
			MediaID:    id,
			FromStatus: sql.NullString{String: ch.from, Valid: true},
			ToStatus:   ch.to,
		}))
	}

	all, err := db.FetchMediaStats(ctx, d)
	require.NoError(t, err)
	assert.Equal(t, int64(2), all[id].StartCount, "FetchMediaStats")

	single, err := db.FetchSingleMediaStats(ctx, d, id)
	require.NoError(t, err)
	assert.Equal(t, int64(2), single.StartCount, "FetchSingleMediaStats")
}

// Bug 32: the row above the list shows the media of the newest start event.
// started_at only has whole seconds, so two quick starts tie on it; the start
// event id still tells them apart.
func TestGetLastStartedMediaID(t *testing.T) {
	d := openTestDB(t)
	require.NoError(t, db.SyncScanResults(d, []scanner.VideoFile{
		{Filename: "First.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
		{Filename: "Second.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000},
	}))
	ctx := context.Background()
	q := db.New(d)

	_, err := q.GetLastStartedMediaID(ctx)
	assert.ErrorIs(t, err, sql.ErrNoRows, "nothing started yet")

	ids := map[string]int64{}
	media, err := q.ListOnDiskMedia(ctx)
	require.NoError(t, err)
	for _, m := range media {
		ids[m.Filename] = m.ID
	}
	require.Len(t, ids, 2)

	// Second.mkv first, then First.mkv in the same second: the answer must be
	// the later start, not the higher media id.
	const sameSecond = "2026-11-03 20:15:00"
	for _, name := range []string{"Second.mkv", "First.mkv"} {
		_, err = d.Exec("INSERT INTO start_events (media_id, started_at) VALUES (?, ?)", ids[name], sameSecond)
		require.NoError(t, err)
	}

	got, err := q.GetLastStartedMediaID(ctx)
	require.NoError(t, err)
	assert.Equal(t, ids["First.mkv"], got, "the newest start event wins a tie on started_at")
}
