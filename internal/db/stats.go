package db

import (
	"context"
	"database/sql"
	"time"
)

type MediaStats struct {
	LastStartedAt sql.NullTime
	StartCount    int64
}

type MediaWithStats struct {
	Medium
	MediaStats
}

func FetchMediaStats(ctx context.Context, d *sql.DB) (map[int64]MediaStats, error) {
	const q = `
	SELECT
		m.id,
		MAX(se.started_at) AS last_started_at,
		COUNT(se.id) AS start_count
	FROM media m
	LEFT JOIN start_events se ON se.media_id = m.id
	GROUP BY m.id
	`
	rows, err := d.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int64]MediaStats)
	for rows.Next() {
		var mediaID int64
		var lastStarted interface{}
		var startCount int64
		if err := rows.Scan(&mediaID, &lastStarted, &startCount); err != nil {
			return nil, err
		}
		result[mediaID] = MediaStats{
			LastStartedAt: toNullTime(lastStarted),
			StartCount:    startCount,
		}
	}
	return result, rows.Err()
}

func FetchSingleMediaStats(ctx context.Context, d *sql.DB, id int64) (MediaStats, error) {
	const q = `
	SELECT
		MAX(se.started_at) AS last_started_at,
		COUNT(se.id) AS start_count
	FROM media m
	LEFT JOIN start_events se ON se.media_id = m.id
	WHERE m.id = ?
	GROUP BY m.id
	`
	var lastStarted interface{}
	var startCount int64
	err := d.QueryRowContext(ctx, q, id).Scan(&lastStarted, &startCount)
	if err == sql.ErrNoRows {
		return MediaStats{}, nil
	}
	if err != nil {
		return MediaStats{}, err
	}
	return MediaStats{
		LastStartedAt: toNullTime(lastStarted),
		StartCount:    startCount,
	}, nil
}

func toNullTime(v interface{}) sql.NullTime {
	if v == nil {
		return sql.NullTime{}
	}
	switch t := v.(type) {
	case time.Time:
		return sql.NullTime{Time: t, Valid: true}
	case string:
		parsed, err := time.Parse("2006-01-02 15:04:05", t)
		if err != nil {
			return sql.NullTime{}
		}
		return sql.NullTime{Time: parsed, Valid: true}
	}
	return sql.NullTime{}
}
