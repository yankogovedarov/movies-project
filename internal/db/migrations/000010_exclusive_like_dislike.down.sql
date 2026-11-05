-- Bug 33: no automatic down. The up migration clears a 👎 without writing any
-- history, so nothing tells which media had it. The backup made at startup, before
-- the migrations run, is the way back.
SELECT 1;
