-- Bug 30: the translation feature is removed together with its data — the
-- media's translation type and the remembered translation filter. Neither
-- column has an index, so SQLite can drop them in place.
ALTER TABLE media DROP COLUMN translation_type;
ALTER TABLE ui_prefs DROP COLUMN trans_filter;
