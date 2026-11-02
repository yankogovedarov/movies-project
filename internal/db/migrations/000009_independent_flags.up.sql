-- Bug 30: „За DnD" stops being a status and becomes a flag (👍), next to the
-- „за изтриване" flag (👎, 000004) and the new „грешни субтитри" flag (⚠).
-- The three flags are independent of the status and of each other; each one
-- has its own remembered filter.
ALTER TABLE media ADD COLUMN for_dnd INTEGER NOT NULL DEFAULT 0;
ALTER TABLE media ADD COLUMN wrong_subs INTEGER NOT NULL DEFAULT 0;
ALTER TABLE ui_prefs ADD COLUMN dnd_filter TEXT NOT NULL DEFAULT 'all';
ALTER TABLE ui_prefs ADD COLUMN subs_filter TEXT NOT NULL DEFAULT 'all';

-- A former „За DnD" media keeps the mark as the flag and gets the status the
-- two automatic statuses stand for: "started" once it has been started, else
-- "new". A history row is written first, so status_changes stays consistent
-- with media.current_status (as in 000007).
UPDATE media SET for_dnd = 1 WHERE current_status = 'for_dnd';

INSERT INTO status_changes (media_id, from_status, to_status)
SELECT id, 'for_dnd',
       CASE WHEN EXISTS (SELECT 1 FROM start_events se WHERE se.media_id = media.id)
            THEN 'started' ELSE 'new' END
FROM media WHERE current_status = 'for_dnd';

UPDATE media SET current_status =
       CASE WHEN EXISTS (SELECT 1 FROM start_events se WHERE se.media_id = media.id)
            THEN 'started' ELSE 'new' END
WHERE current_status = 'for_dnd';

-- A remembered „За DnD" status filter would leave the list empty after the
-- restart; the same media are now shown through the flag filter.
UPDATE ui_prefs SET status_filter = 'all', dnd_filter = 'yes' WHERE status_filter = 'for_dnd';
