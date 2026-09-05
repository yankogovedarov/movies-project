-- Bug 23: запомняне на избраните филтри и сортиране между пусканията.
-- Един ред (id = 1) с точно колоните, които регистърът държи в URL-а.
CREATE TABLE ui_prefs (
    id            INTEGER PRIMARY KEY CHECK (id = 1),
    status_filter TEXT NOT NULL DEFAULT 'all',
    disk_filter   TEXT NOT NULL DEFAULT 'on',
    sort_filter   TEXT NOT NULL DEFAULT 'name',
    dir_filter    TEXT NOT NULL DEFAULT 'asc',
    q_filter      TEXT NOT NULL DEFAULT '',
    trans_filter  TEXT NOT NULL DEFAULT 'all',
    del_filter    TEXT NOT NULL DEFAULT 'all',
    updated_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO ui_prefs (id) VALUES (1);
