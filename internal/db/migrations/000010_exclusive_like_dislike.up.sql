-- Bug 33: 👍 (for_dnd) and 👎 (for_deletion) exclude each other from now on. By
-- the user's decision a media that carries both keeps 👍. ⚠ (wrong_subs) stays
-- independent. A remembered dnd_filter + del_filter pair is repaired by the list
-- handler (exclusiveLikeFilters), so ui_prefs is left alone here.
UPDATE media SET for_deletion = 0 WHERE for_dnd = 1 AND for_deletion = 1;
