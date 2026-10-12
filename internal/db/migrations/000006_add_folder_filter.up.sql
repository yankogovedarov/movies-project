-- Bug 26: remember the chosen main folder (the folder dropdown on the list page).
ALTER TABLE ui_prefs ADD COLUMN folder_filter TEXT NOT NULL DEFAULT 'all';
