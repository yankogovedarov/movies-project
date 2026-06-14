-- name: UpsertMedia :one
INSERT INTO media (filename, folder_relative_path, file_size_bytes, on_disk, file_created_at, translation_type)
VALUES (?, ?, ?, 1, ?, ?)
ON CONFLICT(filename, file_size_bytes) DO UPDATE SET
    folder_relative_path = excluded.folder_relative_path,
    on_disk = 1,
    file_created_at = COALESCE(media.file_created_at, excluded.file_created_at),
    -- Keep an existing (manually set or previously detected) translation type;
    -- only fill in the detected hint when the current value is still empty.
    translation_type = CASE WHEN media.translation_type != '' THEN media.translation_type ELSE excluded.translation_type END
RETURNING id;

-- name: MarkAllOffDisk :exec
UPDATE media SET on_disk = 0;

-- name: CountOnDisk :one
SELECT COUNT(*) FROM media WHERE on_disk = 1;

-- name: ListOnDiskMedia :many
SELECT id, filename, folder_relative_path, file_size_bytes, current_status, on_disk, created_at, file_created_at, translation_type
FROM media
WHERE on_disk = 1
ORDER BY folder_relative_path, filename;

-- name: GetMediaByID :one
SELECT id, filename, folder_relative_path, file_size_bytes, current_status, on_disk, created_at, file_created_at, translation_type
FROM media WHERE id = ?;

-- name: InsertStartEvent :exec
INSERT INTO start_events (media_id) VALUES (?);

-- name: UpdateMediaStatus :exec
UPDATE media SET current_status = ? WHERE id = ?;

-- name: InsertStatusChange :exec
INSERT INTO status_changes (media_id, from_status, to_status) VALUES (?, ?, ?);

-- name: ListAllMedia :many
SELECT id, filename, folder_relative_path, file_size_bytes, current_status, on_disk, created_at, file_created_at, translation_type
FROM media
ORDER BY folder_relative_path, filename;

-- name: UpdateTranslationType :exec
UPDATE media SET translation_type = ? WHERE id = ?;

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
