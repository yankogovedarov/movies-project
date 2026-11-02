-- Bug 30: the columns come back with their original defaults ('' and 'all', as
-- 000003 and 000005 created them), not with their old values; those are only
-- kept in the backup made at startup, before the migrations run.
ALTER TABLE media ADD COLUMN translation_type TEXT NOT NULL DEFAULT '';
ALTER TABLE ui_prefs ADD COLUMN trans_filter TEXT NOT NULL DEFAULT 'all';
