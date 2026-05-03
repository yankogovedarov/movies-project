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
