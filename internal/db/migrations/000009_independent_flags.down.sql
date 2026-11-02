-- Bug 30: no automatic down. Dropping the new columns would erase the 👍 and ⚠
-- flags (the 👍 of every former „За DnD" media included). The former status is
-- still kept as history — status_changes.from_status of the rows the up
-- migration wrote — and the backup made at startup, before the migrations run,
-- is the way back. Unlike 000007, the up migration cannot run again after this
-- down: it starts with ADD COLUMN and the columns stay, so restore the backup.
SELECT 1;
