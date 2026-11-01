-- Bug 27: the completed statuses (completed_both / completed_yanko /
-- completed_liza) are removed; such media become "started".
-- A history row is written first, so status_changes stays consistent with
-- media.current_status.
INSERT INTO status_changes (media_id, from_status, to_status)
SELECT id, current_status, 'started' FROM media
WHERE current_status IN ('completed_both', 'completed_yanko', 'completed_liza');

UPDATE media SET current_status = 'started'
WHERE current_status IN ('completed_both', 'completed_yanko', 'completed_liza');

-- A remembered removed status or the removed "marked" sort would leave the
-- list empty / without an active sort button after the restart.
UPDATE ui_prefs SET status_filter = 'all'
WHERE status_filter IN ('completed_both', 'completed_yanko', 'completed_liza');

UPDATE ui_prefs SET sort_filter = 'name', dir_filter = 'asc' WHERE sort_filter = 'marked';
