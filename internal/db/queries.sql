-- name: UpsertMedia :one
INSERT INTO media (filename, folder_relative_path, file_size_bytes, on_disk)
VALUES (?, ?, ?, 1)
ON CONFLICT(filename, file_size_bytes) DO UPDATE SET
    folder_relative_path = excluded.folder_relative_path,
    on_disk = 1
RETURNING id;

-- name: MarkAllOffDisk :exec
UPDATE media SET on_disk = 0;

-- name: CountOnDisk :one
SELECT COUNT(*) FROM media WHERE on_disk = 1;

-- name: ListOnDiskMedia :many
SELECT id, filename, folder_relative_path, file_size_bytes, current_status, on_disk, created_at
FROM media
WHERE on_disk = 1
ORDER BY folder_relative_path, filename;

-- name: GetMediaByID :one
SELECT id, filename, folder_relative_path, file_size_bytes, current_status, on_disk, created_at
FROM media WHERE id = ?;

-- name: InsertStartEvent :exec
INSERT INTO start_events (media_id) VALUES (?);

-- name: UpdateMediaStatus :exec
UPDATE media SET current_status = ? WHERE id = ?;

-- name: InsertStatusChange :exec
INSERT INTO status_changes (media_id, from_status, to_status) VALUES (?, ?, ?);

-- name: ListAllMedia :many
SELECT id, filename, folder_relative_path, file_size_bytes, current_status, on_disk, created_at
FROM media
ORDER BY folder_relative_path, filename;

-- name: GetStartEvents :many
SELECT id, media_id, started_at
FROM start_events
WHERE media_id = ?
ORDER BY started_at DESC;

-- name: GetStatusChanges :many
SELECT id, media_id, from_status, to_status, changed_at
FROM status_changes
WHERE media_id = ?
ORDER BY changed_at DESC;
