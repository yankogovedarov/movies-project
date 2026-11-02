-- name: UpsertMedia :one
INSERT INTO media (filename, folder_relative_path, file_size_bytes, on_disk, file_created_at)
VALUES (?, ?, ?, 1, ?)
ON CONFLICT(filename, file_size_bytes) DO UPDATE SET
    folder_relative_path = excluded.folder_relative_path,
    on_disk = 1,
    file_created_at = COALESCE(media.file_created_at, excluded.file_created_at)
RETURNING id;

-- name: MarkAllOffDisk :exec
UPDATE media SET on_disk = 0;

-- name: CountOnDisk :one
SELECT COUNT(*) FROM media WHERE on_disk = 1;

-- name: ListOnDiskMedia :many
SELECT id, filename, folder_relative_path, file_size_bytes, current_status, on_disk, created_at, file_created_at, for_deletion, for_dnd, wrong_subs
FROM media
WHERE on_disk = 1
ORDER BY folder_relative_path, filename;

-- name: GetMediaByID :one
SELECT id, filename, folder_relative_path, file_size_bytes, current_status, on_disk, created_at, file_created_at, for_deletion, for_dnd, wrong_subs
FROM media WHERE id = ?;

-- name: InsertStartEvent :exec
INSERT INTO start_events (media_id) VALUES (?);

-- name: UpdateMediaStatus :exec
UPDATE media SET current_status = ? WHERE id = ?;

-- name: InsertStatusChange :exec
INSERT INTO status_changes (media_id, from_status, to_status) VALUES (?, ?, ?);

-- name: ListAllMedia :many
SELECT id, filename, folder_relative_path, file_size_bytes, current_status, on_disk, created_at, file_created_at, for_deletion, for_dnd, wrong_subs
FROM media
ORDER BY folder_relative_path, filename;

-- name: UpdateForDeletion :exec
UPDATE media SET for_deletion = ? WHERE id = ?;

-- name: UpdateForDnd :exec
UPDATE media SET for_dnd = ? WHERE id = ?;

-- name: UpdateWrongSubs :exec
UPDATE media SET wrong_subs = ? WHERE id = ?;

-- name: GetUIPrefs :one
SELECT status_filter, disk_filter, sort_filter, dir_filter, q_filter, del_filter, dnd_filter, subs_filter, folder_filter
FROM ui_prefs WHERE id = 1;

-- name: SaveUIPrefs :exec
INSERT INTO ui_prefs (id, status_filter, disk_filter, sort_filter, dir_filter, q_filter, del_filter, dnd_filter, subs_filter, folder_filter, updated_at)
VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT(id) DO UPDATE SET
    status_filter = excluded.status_filter,
    disk_filter = excluded.disk_filter,
    sort_filter = excluded.sort_filter,
    dir_filter = excluded.dir_filter,
    q_filter = excluded.q_filter,
    del_filter = excluded.del_filter,
    dnd_filter = excluded.dnd_filter,
    subs_filter = excluded.subs_filter,
    folder_filter = excluded.folder_filter,
    updated_at = CURRENT_TIMESTAMP;
