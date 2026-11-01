-- Bug 27: no automatic down. The original completed statuses are still kept as
-- history — status_changes.from_status of the rows the up migration wrote — and
-- the backup made at startup, before the migrations run, is the other way back.
SELECT 1;
