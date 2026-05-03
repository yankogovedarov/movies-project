-- Таблица media: всяка медия (филм или епизод) е един запис
CREATE TABLE media (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    filename TEXT NOT NULL,
    folder_relative_path TEXT NOT NULL,
    file_size_bytes INTEGER NOT NULL,
    current_status TEXT NOT NULL DEFAULT 'new',
    on_disk INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(filename, file_size_bytes)
);

CREATE INDEX idx_media_status ON media(current_status);
CREATE INDEX idx_media_on_disk ON media(on_disk);
CREATE INDEX idx_media_folder ON media(folder_relative_path);

-- Event log: всяко стартиране на медия
CREATE TABLE start_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_start_events_media ON start_events(media_id);
CREATE INDEX idx_start_events_started_at ON start_events(started_at);

-- Event log: всяка промяна на статуса
CREATE TABLE status_changes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    from_status TEXT,
    to_status TEXT NOT NULL,
    changed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_status_changes_media ON status_changes(media_id);

-- Състояние на дървовидния изглед
CREATE TABLE tree_state (
    folder_path TEXT PRIMARY KEY,
    expanded INTEGER NOT NULL DEFAULT 0
);
