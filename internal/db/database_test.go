package db_test

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yankogovedarov/movie-tracker/internal/db"
)

func TestOpen(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	d, err := db.Open(dbPath)
	assert.NoError(t, err)
	assert.NotNil(t, d)
	d.Close()
}

func TestMigrate(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	d, err := db.Open(dbPath)
	assert.NoError(t, err)
	defer d.Close()

	err = db.Migrate(d)
	assert.NoError(t, err)

	var tableName string
	row := d.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='media'")
	err = row.Scan(&tableName)
	assert.NoError(t, err)
	assert.Equal(t, "media", tableName)
}

func TestMigrate_HasForDeletionColumn(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	d, err := db.Open(dbPath)
	require.NoError(t, err)
	defer d.Close()
	require.NoError(t, db.Migrate(d))

	// The column exists and defaults to 0 (not marked for deletion).
	_, err = d.Exec(`INSERT INTO media (filename, folder_relative_path, file_size_bytes)
		VALUES ('Film.mkv', 'Films', 100)`)
	require.NoError(t, err)

	var forDeletion int64
	row := d.QueryRow("SELECT for_deletion FROM media WHERE filename = 'Film.mkv'")
	require.NoError(t, row.Scan(&forDeletion))
	assert.Equal(t, int64(0), forDeletion, "for_deletion must default to 0")
}

// Bug 23: the UI filter/sort state lives in a single-row `ui_prefs` table so it
// survives a restart of the backend.
func TestMigrate_HasUIPrefsRow(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	d, err := db.Open(dbPath)
	require.NoError(t, err)
	defer d.Close()
	require.NoError(t, db.Migrate(d))

	var status, disk, sortF, dir, q, del string
	row := d.QueryRow(`SELECT status_filter, disk_filter, sort_filter, dir_filter,
		q_filter, del_filter FROM ui_prefs WHERE id = 1`)
	require.NoError(t, row.Scan(&status, &disk, &sortF, &dir, &q, &del))

	// The defaults mirror the handler's c.DefaultQuery values.
	assert.Equal(t, "all", status)
	assert.Equal(t, "on", disk)
	assert.Equal(t, "name", sortF)
	assert.Equal(t, "asc", dir)
	assert.Equal(t, "", q)
	assert.Equal(t, "all", del)
}

// Bug 26: the main-folder filter is remembered next to the other filters and
// defaults to "all" (no folder filter).
func TestMigrate_UIPrefsHasFolderFilter(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	d, err := db.Open(dbPath)
	require.NoError(t, err)
	defer d.Close()
	require.NoError(t, db.Migrate(d))

	var folder string
	row := d.QueryRow("SELECT folder_filter FROM ui_prefs WHERE id = 1")
	require.NoError(t, row.Scan(&folder))
	assert.Equal(t, "all", folder, "folder_filter must default to all")
}

func TestMigrate_UIPrefsRejectsSecondRow(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	d, err := db.Open(dbPath)
	require.NoError(t, err)
	defer d.Close()
	require.NoError(t, db.Migrate(d))

	_, err = d.Exec("INSERT INTO ui_prefs (id) VALUES (2)")
	assert.Error(t, err, "ui_prefs must hold a single row (id = 1)")
}

// openAtSchemaVersion opens a fresh database migrated only up to version — the
// state an older release left behind.
func openAtSchemaVersion(t *testing.T, version uint) *sql.DB {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { _ = d.Close() })

	migrateTo(t, d, version)
	return d
}

// migrateTo runs the migrations of d up to version and no further, so a test
// of one migration does not also see what the later ones change.
func migrateTo(t *testing.T, d *sql.DB, version uint) {
	t.Helper()
	m, err := db.NewMigrate(d)
	require.NoError(t, err)
	require.NoError(t, m.Migrate(version))
}

// Bug 27: the three completed statuses are removed. Migration 000007 turns every
// completed media into "started", records that change in the media's history
// and resets a remembered filter/sort that pointed at the removed statuses.
func TestMigrate_CompletedStatusesBecomeStarted(t *testing.T) {
	// The database as the previous version left it (schema version 6).
	d := openAtSchemaVersion(t, 6)

	_, err := d.Exec(`INSERT INTO media (filename, folder_relative_path, file_size_bytes, current_status) VALUES
		('Both.mkv', 'Films', 100, 'completed_both'),
		('Yanko.mkv', 'Films', 200, 'completed_yanko'),
		('Liza.mkv', 'Films', 300, 'completed_liza'),
		('Fresh.mkv', 'Films', 400, 'new')`)
	require.NoError(t, err)
	_, err = d.Exec(`UPDATE ui_prefs SET status_filter = 'completed_yanko',
		sort_filter = 'marked', dir_filter = 'desc' WHERE id = 1`)
	require.NoError(t, err)

	require.NoError(t, db.Migrate(d))

	statuses := map[string]string{}
	rows, err := d.Query("SELECT filename, current_status FROM media")
	require.NoError(t, err)
	for rows.Next() {
		var name, status string
		require.NoError(t, rows.Scan(&name, &status))
		statuses[name] = status
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
	assert.Equal(t, map[string]string{
		"Both.mkv":  "started",
		"Yanko.mkv": "started",
		"Liza.mkv":  "started",
		"Fresh.mkv": "new",
	}, statuses, "completed media become started, a new one is left alone")

	// The history stays consistent with current_status: one completed_* → started
	// row per formerly completed media, none for the untouched one.
	var changes []string
	rows, err = d.Query(`SELECT m.filename, sc.from_status, sc.to_status
		FROM status_changes sc JOIN media m ON m.id = sc.media_id
		ORDER BY m.filename`)
	require.NoError(t, err)
	for rows.Next() {
		var name, from, to string
		require.NoError(t, rows.Scan(&name, &from, &to))
		changes = append(changes, name+": "+from+" → "+to)
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
	assert.Equal(t, []string{
		"Both.mkv: completed_both → started",
		"Liza.mkv: completed_liza → started",
		"Yanko.mkv: completed_yanko → started",
	}, changes)

	// A remembered removed status or the removed "marked" sort would leave the
	// list empty / without an active sort button after the restart.
	var status, sortF, dir string
	row := d.QueryRow("SELECT status_filter, sort_filter, dir_filter FROM ui_prefs WHERE id = 1")
	require.NoError(t, row.Scan(&status, &sortF, &dir))
	assert.Equal(t, "all", status)
	assert.Equal(t, "name", sortF)
	assert.Equal(t, "asc", dir)
}

// Bug 27: migration 000007 only resets a remembered filter/sort that points at
// a removed status or at the removed "marked" sort; any other choice survives.
func TestMigrate_CompletedStatuses_KeepsValidUIPrefs(t *testing.T) {
	d := openAtSchemaVersion(t, 6)
	_, err := d.Exec(`UPDATE ui_prefs SET status_filter = 'started', sort_filter = 'added',
		dir_filter = 'desc', folder_filter = 'Serials' WHERE id = 1`)
	require.NoError(t, err)

	require.NoError(t, db.Migrate(d))

	var status, sortF, dir, folder string
	row := d.QueryRow("SELECT status_filter, sort_filter, dir_filter, folder_filter FROM ui_prefs WHERE id = 1")
	require.NoError(t, row.Scan(&status, &sortF, &dir, &folder))
	assert.Equal(t, []string{"started", "added", "desc", "Serials"},
		[]string{status, sortF, dir, folder}, "valid remembered choices must not be reset")
}

// tableColumns returns the column names of table, in table order (PRAGMA
// table_info). A missing table yields no columns.
func tableColumns(t *testing.T, d *sql.DB, table string) []string {
	t.Helper()
	rows, err := d.Query("SELECT name FROM pragma_table_info(?) ORDER BY cid", table)
	require.NoError(t, err)

	var cols []string
	for rows.Next() {
		var name string
		require.NoError(t, rows.Scan(&name))
		cols = append(cols, name)
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
	return cols
}

// Bug 30: the translation type is removed together with its data — migration
// 000008 drops the media's translation type and the remembered translation
// filter.
func TestMigrate_DropsTranslationColumns(t *testing.T) {
	d := openTestDB(t)

	for table, column := range map[string]string{
		"media":    "translation_type",
		"ui_prefs": "trans_filter",
	} {
		cols := tableColumns(t, d, table)
		require.NotEmpty(t, cols, "table %s must exist", table)
		assert.NotContains(t, cols, column, "%s.%s must be dropped", table, column)
	}
}

// Bug 30: SQLite rewrites both tables to drop a column; nothing else may change.
// A characterization test — it passes before migration 000008 too — that
// guards the rewrite: the status, the „за изтриване" flag, the history and the
// other remembered filters survive.
func TestMigrate_DropTranslation_KeepsOtherData(t *testing.T) {
	// The database as the previous version left it (schema version 7).
	d := openAtSchemaVersion(t, 7)

	res, err := d.Exec(`INSERT INTO media (filename, folder_relative_path, file_size_bytes,
		current_status, file_created_at, translation_type, for_deletion)
		VALUES ('Dragons.mkv', 'MoviesDnD\Dragons', 4000000000, 'for_dnd', '2025-03-14 15:09:26', 'bg', 1)`)
	require.NoError(t, err)
	id, err := res.LastInsertId()
	require.NoError(t, err)
	_, err = d.Exec("INSERT INTO start_events (media_id) VALUES (?), (?)", id, id)
	require.NoError(t, err)
	_, err = d.Exec("INSERT INTO status_changes (media_id, from_status, to_status) VALUES (?, 'new', 'for_dnd')", id)
	require.NoError(t, err)
	_, err = d.Exec(`UPDATE ui_prefs SET status_filter = 'for_dnd', disk_filter = 'all',
		sort_filter = 'size', dir_filter = 'desc', q_filter = 'drag', trans_filter = 'bg',
		del_filter = 'yes', folder_filter = 'MoviesDnD' WHERE id = 1`)
	require.NoError(t, err)

	// Only 000008: 000009 turns the for_dnd status into a flag (see
	// TestMigrate_ForDnDStatusBecomesFlag).
	migrateTo(t, d, 8)

	var (
		gotID, size, onDisk, forDeletion int64
		name, folder, status             string
		fileCreated                      sql.NullTime
	)
	require.NoError(t, d.QueryRow(`SELECT id, filename, folder_relative_path, file_size_bytes,
		current_status, on_disk, file_created_at, for_deletion FROM media`).
		Scan(&gotID, &name, &folder, &size, &status, &onDisk, &fileCreated, &forDeletion))
	assert.Equal(t, id, gotID, "the id must survive — the history refers to it")
	assert.Equal(t, "Dragons.mkv", name)
	assert.Equal(t, `MoviesDnD\Dragons`, folder)
	assert.Equal(t, int64(4_000_000_000), size)
	assert.Equal(t, "for_dnd", status, "the status must survive")
	assert.Equal(t, int64(1), onDisk)
	assert.True(t, fileCreated.Valid, "the file date must survive")
	assert.True(t, fileCreated.Time.Equal(time.Date(2025, 3, 14, 15, 9, 26, 0, time.UTC)),
		"the file date must survive, got %v", fileCreated.Time)
	assert.Equal(t, int64(1), forDeletion, "the raised „за изтриване\" flag must survive")

	var starts int
	require.NoError(t, d.QueryRow("SELECT COUNT(*) FROM start_events WHERE media_id = ?", id).Scan(&starts))
	assert.Equal(t, 2, starts, "the start history must survive")
	var from, to string
	require.NoError(t, d.QueryRow("SELECT from_status, to_status FROM status_changes WHERE media_id = ?", id).
		Scan(&from, &to))
	assert.Equal(t, []string{"new", "for_dnd"}, []string{from, to}, "the status history must survive")

	var prefs [7]string
	require.NoError(t, d.QueryRow(`SELECT status_filter, disk_filter, sort_filter, dir_filter,
		q_filter, del_filter, folder_filter FROM ui_prefs WHERE id = 1`).
		Scan(&prefs[0], &prefs[1], &prefs[2], &prefs[3], &prefs[4], &prefs[5], &prefs[6]))
	assert.Equal(t, [7]string{"for_dnd", "all", "size", "desc", "drag", "yes", "MoviesDnD"}, prefs,
		"the other remembered filters must survive")
}

// Bug 30: „За DnD" stops being a status and becomes the 👍 flag. Migration
// 000009 raises the flag on every former DnD media and gives it the automatic
// status it would have had — "started" once it has been started, else "new" —,
// records that change in the media's history and turns a remembered DnD status
// filter into the flag filter. Every other media keeps its status.
func TestMigrate_ForDnDStatusBecomesFlag(t *testing.T) {
	// The database as the previous version left it (schema version 8).
	d := openAtSchemaVersion(t, 8)

	_, err := d.Exec(`INSERT INTO media (filename, folder_relative_path, file_size_bytes, current_status) VALUES
		('DnDWatched.mkv', 'MoviesDnD', 100, 'for_dnd'),
		('DnDUnseen.mkv', 'MoviesDnD', 200, 'for_dnd'),
		('StartedByHand.mkv', 'Movies', 300, 'started'),
		('BackToNew.mkv', 'Movies', 400, 'new')`)
	require.NoError(t, err)
	// ▶ sets „Стартирана" without a start and ○ sets „Нова" after one, so only
	// the former DnD media may take their status from the starts.
	_, err = d.Exec(`INSERT INTO start_events (media_id) SELECT id FROM media
		WHERE filename IN ('DnDWatched.mkv', 'BackToNew.mkv')`)
	require.NoError(t, err)
	_, err = d.Exec("UPDATE ui_prefs SET status_filter = 'for_dnd' WHERE id = 1")
	require.NoError(t, err)

	require.NoError(t, db.Migrate(d))

	media := map[string]string{}
	rows, err := d.Query("SELECT filename, current_status, for_dnd FROM media")
	require.NoError(t, err)
	for rows.Next() {
		var name, status string
		var forDnd int64
		require.NoError(t, rows.Scan(&name, &status, &forDnd))
		media[name] = fmt.Sprintf("%s, for_dnd=%d", status, forDnd)
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
	assert.Equal(t, map[string]string{
		"DnDWatched.mkv":    "started, for_dnd=1",
		"DnDUnseen.mkv":     "new, for_dnd=1",
		"StartedByHand.mkv": "started, for_dnd=0",
		"BackToNew.mkv":     "new, for_dnd=0",
	}, media, "a former DnD media keeps the mark as the flag and takes the status of its starts")

	// The history stays consistent with current_status: one for_dnd → … row per
	// former DnD media, none for the others.
	var changes []string
	rows, err = d.Query(`SELECT m.filename, sc.from_status, sc.to_status
		FROM status_changes sc JOIN media m ON m.id = sc.media_id
		ORDER BY m.filename`)
	require.NoError(t, err)
	for rows.Next() {
		var name, from, to string
		require.NoError(t, rows.Scan(&name, &from, &to))
		changes = append(changes, name+": "+from+" → "+to)
	}
	require.NoError(t, rows.Err())
	require.NoError(t, rows.Close())
	assert.Equal(t, []string{
		"DnDUnseen.mkv: for_dnd → new",
		"DnDWatched.mkv: for_dnd → started",
	}, changes)

	// A remembered DnD status filter shows the same media through the flag.
	var status, dnd string
	require.NoError(t, d.QueryRow("SELECT status_filter, dnd_filter FROM ui_prefs WHERE id = 1").
		Scan(&status, &dnd))
	assert.Equal(t, "all", status)
	assert.Equal(t, "yes", dnd)
}

// Bug 30: the 👍 „За DnD" and ⚠ „Грешни субтитри" flags start lowered, and
// their filters are remembered next to the others, showing every media by
// default.
func TestMigrate_HasFlagColumns(t *testing.T) {
	d := openTestDB(t)
	_, err := d.Exec(`INSERT INTO media (filename, folder_relative_path, file_size_bytes)
		VALUES ('Film.mkv', 'Films', 100)`)
	require.NoError(t, err)

	var forDnd, wrongSubs int64
	require.NoError(t, d.QueryRow("SELECT for_dnd, wrong_subs FROM media WHERE filename = 'Film.mkv'").
		Scan(&forDnd, &wrongSubs))
	assert.Equal(t, int64(0), forDnd, "for_dnd must default to 0")
	assert.Equal(t, int64(0), wrongSubs, "wrong_subs must default to 0")

	var dnd, subs string
	require.NoError(t, d.QueryRow("SELECT dnd_filter, subs_filter FROM ui_prefs WHERE id = 1").Scan(&dnd, &subs))
	assert.Equal(t, "all", dnd, "dnd_filter must default to all")
	assert.Equal(t, "all", subs, "subs_filter must default to all")
}
