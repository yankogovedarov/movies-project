package web

import (
	"database/sql"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/yankogovedarov/movie-tracker/internal/db"
	"github.com/yankogovedarov/movie-tracker/internal/scanner"
	"github.com/yankogovedarov/movie-tracker/internal/tree"
	"github.com/yankogovedarov/movie-tracker/templates"
)

// Launcher starts an external program (VLC, Explorer). It exists so that
// tests never spawn real windows: a nil Launcher launches nothing at all, and
// only main.go opts in to the real one via ExecLauncher.
type Launcher func(name string, args ...string) error

// ExecLauncher is the production Launcher: it starts the program and returns
// immediately, without waiting for it to exit.
func ExecLauncher(name string, args ...string) error {
	return exec.Command(name, args...).Start()
}

// NoopLauncher launches nothing. Used when MOVIETRACKER_NO_LAUNCH=1 (see
// NoLaunch) so that an end-to-end run does not open VLC or Explorer.
func NoopLauncher(name string, args ...string) error { return nil }

// NoLaunch reports whether external programs must not be started. The e2e
// suite runs the real binary and sets MOVIETRACKER_NO_LAUNCH=1 for it.
func NoLaunch() bool {
	return os.Getenv("MOVIETRACKER_NO_LAUNCH") == "1"
}

type Handlers struct {
	DB       *sql.DB
	DiskRoot string
	VLCPath  string
	Log      *slog.Logger
	// Launcher starts VLC and Explorer. A nil Launcher means "launch nothing",
	// which keeps the test suite free of stray windows.
	Launcher Launcher
}

// launch starts an external program through the configured Launcher. Failures
// are logged, never surfaced to the user: the page must render either way.
func (h *Handlers) launch(name string, args ...string) {
	if h.Launcher == nil {
		return
	}
	if err := h.Launcher(name, args...); err != nil {
		h.log().Warn("launch failed", "program", name, "error", err)
	}
}

func (h *Handlers) log() *slog.Logger {
	if h.Log != nil {
		return h.Log
	}
	return slog.Default()
}

func isHTMX(c *gin.Context) bool {
	return c.GetHeader("HX-Request") == "true"
}

// validStatuses are the statuses a media can be set to. Bug 27 removed the
// three completed statuses and added the manual "for_dnd" (За DnD).
var validStatuses = map[string]bool{
	"new":     true,
	"started": true,
	"for_dnd": true,
}

// topFolder returns the main (top-level) folder of a media's folder path: the
// first segment before a `\` or `/`. Real scans store Windows paths with `\`,
// test seeds also use `/`. A file directly in the disk root has no main folder
// and yields "".
func topFolder(folderRelativePath string) string {
	if i := strings.IndexAny(folderRelativePath, `\/`); i >= 0 {
		return folderRelativePath[:i]
	}
	return folderRelativePath
}

// folderOptions returns the distinct, sorted main folders of the given media —
// the choices of the folder dropdown (Bug 26). Root files have no main folder
// and add no entry. A selected folder that no longer matches any media is kept,
// so the dropdown still shows what filters the (now empty) list.
func folderOptions(raw []db.Medium, current string) []string {
	seen := make(map[string]bool)
	var folders []string
	for _, m := range raw {
		f := topFolder(m.FolderRelativePath)
		if f == "" || seen[f] {
			continue
		}
		seen[f] = true
		folders = append(folders, f)
	}
	if current != "all" && !seen[current] {
		folders = append(folders, current)
	}
	sort.Strings(folders)
	return folders
}

func filterMedia(raw []db.Medium, statusFilter, qFilter, transFilter, delFilter, folderFilter string) []db.Medium {
	result := raw
	if statusFilter != "all" {
		f := make([]db.Medium, 0, len(result))
		for _, m := range result {
			if m.CurrentStatus == statusFilter {
				f = append(f, m)
			}
		}
		result = f
	}
	if qFilter != "" {
		q := strings.ToLower(qFilter)
		f := make([]db.Medium, 0, len(result))
		for _, m := range result {
			if strings.Contains(strings.ToLower(m.Filename), q) ||
				strings.Contains(strings.ToLower(m.FolderRelativePath), q) {
				f = append(f, m)
			}
		}
		result = f
	}
	if transFilter != "all" {
		// "none" matches media with no translation type set yet.
		want := transFilter
		if transFilter == "none" {
			want = ""
		}
		f := make([]db.Medium, 0, len(result))
		for _, m := range result {
			if m.TranslationType == want {
				f = append(f, m)
			}
		}
		result = f
	}
	if delFilter == "yes" {
		// The "за изтриване" flag is independent of the watch status, so this
		// narrows whatever the other filters already selected.
		f := make([]db.Medium, 0, len(result))
		for _, m := range result {
			if m.ForDeletion == 1 {
				f = append(f, m)
			}
		}
		result = f
	}
	if folderFilter != "all" {
		// Exact match on the main folder, so "Movies" does not also catch
		// "MoviesDnD"; root files only show under "all".
		f := make([]db.Medium, 0, len(result))
		for _, m := range result {
			if topFolder(m.FolderRelativePath) == folderFilter {
				f = append(f, m)
			}
		}
		result = f
	}
	return result
}

// defaultPrefs mirrors the ui_prefs row created by migration 000005, plus the
// folder_filter column added by 000006 — the fallback whenever the remembered
// state cannot be read.
func defaultPrefs() db.GetUIPrefsRow {
	return db.GetUIPrefsRow{
		StatusFilter: "all",
		DiskFilter:   "on",
		SortFilter:   "name",
		DirFilter:    "asc",
		QFilter:      "",
		TransFilter:  "all",
		DelFilter:    "all",
		FolderFilter: "all",
	}
}

// loadPrefs returns the filter/sort state remembered from the previous run
// (Bug 23). A database problem must never break the list, so it degrades to the
// built-in defaults.
func (h *Handlers) loadPrefs(c *gin.Context) db.GetUIPrefsRow {
	prefs, err := db.New(h.DB).GetUIPrefs(c.Request.Context())
	if err != nil {
		h.log().Error("load ui prefs failed", "err", err)
		return defaultPrefs()
	}
	return prefs
}

func (h *Handlers) Index(c *gin.Context) {
	// The remembered state acts as the per-parameter default, so an explicit
	// query parameter always wins and a bare "/" restores the previous run.
	prefs := h.loadPrefs(c)
	statusFilter := c.DefaultQuery("status", prefs.StatusFilter)
	diskFilter := c.DefaultQuery("disk", prefs.DiskFilter)
	sortFilter := c.DefaultQuery("sort", prefs.SortFilter)
	dirFilter := c.DefaultQuery("dir", prefs.DirFilter)
	qFilter := c.DefaultQuery("q", prefs.QFilter)
	transFilter := c.DefaultQuery("trans", prefs.TransFilter)
	delFilter := c.DefaultQuery("del", prefs.DelFilter)
	folderFilter := c.DefaultQuery("folder", prefs.FolderFilter)
	if folderFilter == "" {
		// An explicit empty "folder=" means all folders, not "only the files in
		// the disk root".
		folderFilter = "all"
	}

	ctx := c.Request.Context()
	q := db.New(h.DB)

	if err := q.SaveUIPrefs(ctx, db.SaveUIPrefsParams{
		StatusFilter: statusFilter,
		DiskFilter:   diskFilter,
		SortFilter:   sortFilter,
		DirFilter:    dirFilter,
		QFilter:      qFilter,
		TransFilter:  transFilter,
		DelFilter:    delFilter,
		FolderFilter: folderFilter,
	}); err != nil {
		h.log().Error("save ui prefs failed", "err", err)
	}

	var raw []db.Medium
	var err error
	if diskFilter == "all" {
		raw, err = q.ListAllMedia(ctx)
	} else {
		raw, err = q.ListOnDiskMedia(ctx)
	}
	if err != nil {
		h.log().Error("list media failed", "err", err)
		c.String(http.StatusInternalServerError, "db error: %v", err)
		return
	}

	// The dropdown offers every main folder of the current ↕ mode. It is built
	// before the other filters narrow the list, so picking a status, a
	// translation or a search text never hides folders from it.
	folders := folderOptions(raw, folderFilter)

	raw = filterMedia(raw, statusFilter, qFilter, transFilter, delFilter, folderFilter)

	stats, err := db.FetchMediaStats(ctx, h.DB)
	if err != nil {
		h.log().Error("fetch media stats failed", "err", err)
		stats = make(map[int64]db.MediaStats)
	}

	media := make([]db.MediaWithStats, len(raw))
	for i, m := range raw {
		media[i] = db.MediaWithStats{Medium: m, MediaStats: stats[m.ID]}
	}

	asc := dirFilter != "desc"

	switch sortFilter {
	case "last_started":
		sort.Slice(media, func(i, j int) bool {
			vi, vj := media[i].LastStartedAt, media[j].LastStartedAt
			if !vi.Valid && !vj.Valid {
				return media[i].Filename < media[j].Filename
			}
			if !vi.Valid {
				return !asc
			}
			if !vj.Valid {
				return asc
			}
			if asc {
				return vi.Time.Before(vj.Time)
			}
			return vi.Time.After(vj.Time)
		})
	case "added":
		sort.Slice(media, func(i, j int) bool {
			ti := media[i].CreatedAt
			if media[i].FileCreatedAt.Valid {
				ti = media[i].FileCreatedAt.Time
			}
			tj := media[j].CreatedAt
			if media[j].FileCreatedAt.Valid {
				tj = media[j].FileCreatedAt.Time
			}
			if asc {
				return ti.Before(tj)
			}
			return ti.After(tj)
		})
	case "path":
		sort.Slice(media, func(i, j int) bool {
			if asc {
				return media[i].FolderRelativePath < media[j].FolderRelativePath
			}
			return media[i].FolderRelativePath > media[j].FolderRelativePath
		})
	case "size":
		sort.Slice(media, func(i, j int) bool {
			if asc {
				return media[i].FileSizeBytes < media[j].FileSizeBytes
			}
			return media[i].FileSizeBytes > media[j].FileSizeBytes
		})
	default: // "name"
		sort.Slice(media, func(i, j int) bool {
			if asc {
				return media[i].Filename < media[j].Filename
			}
			return media[i].Filename > media[j].Filename
		})
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	templates.ListPage(media, statusFilter, diskFilter, sortFilter, dirFilter, qFilter, transFilter, delFilter, folderFilter, folders).Render(ctx, c.Writer)
}

func (h *Handlers) RandomNew(c *gin.Context) {
	ctx := c.Request.Context()
	q := db.New(h.DB)

	statusFilter := c.PostForm("status")
	diskFilter := c.PostForm("disk")
	qFilter := c.PostForm("q")
	transFilter := c.PostForm("trans")
	delFilter := c.PostForm("del")
	folderFilter := c.PostForm("folder")
	if statusFilter == "" {
		statusFilter = "all"
	}
	if diskFilter == "" {
		diskFilter = "on"
	}
	if transFilter == "" {
		transFilter = "all"
	}
	if delFilter == "" {
		delFilter = "all"
	}
	if folderFilter == "" {
		folderFilter = "all"
	}

	var all []db.Medium
	var err error
	if diskFilter == "all" {
		all, err = q.ListAllMedia(ctx)
	} else {
		all, err = q.ListOnDiskMedia(ctx)
	}
	if err != nil {
		h.log().Error("list media failed", "err", err)
		c.Redirect(http.StatusSeeOther, "/")
		return
	}

	candidates := filterMedia(all, statusFilter, qFilter, transFilter, delFilter, folderFilter)

	// Without HTMX the browser is sent back to the list with the same filters.
	// Folder names may contain spaces, Cyrillic or "&", hence the escaping.
	listURL := fmt.Sprintf("/?status=%s&disk=%s&q=%s&trans=%s&del=%s&folder=%s",
		statusFilter, diskFilter, qFilter, transFilter, delFilter, url.QueryEscape(folderFilter))

	if len(candidates) == 0 {
		if isHTMX(c) {
			c.Header("Content-Type", "text/html; charset=utf-8")
			templates.Flash("Няма филми за избор").Render(ctx, c.Writer)
			return
		}
		c.Redirect(http.StatusSeeOther, listURL)
		return
	}

	idx := rand.Intn(len(candidates))
	chosen := candidates[idx]

	_ = q.InsertStartEvent(ctx, chosen.ID)

	// Only a brand-new media transitions to "started"; a "started" or "for_dnd"
	// media keeps its status so it stays inside the active filter.
	if chosen.CurrentStatus == "new" {
		_ = q.InsertStatusChange(ctx, db.InsertStatusChangeParams{
			MediaID:    chosen.ID,
			FromStatus: sql.NullString{String: "new", Valid: true},
			ToStatus:   "started",
		})
		_ = q.UpdateMediaStatus(ctx, db.UpdateMediaStatusParams{
			CurrentStatus: "started",
			ID:            chosen.ID,
		})
	}

	h.log().Info("random started", "id", chosen.ID, "file", chosen.Filename, "index", idx, "pool", len(candidates))

	if h.VLCPath != "" {
		fullPath := filepath.Join(h.DiskRoot, chosen.FolderRelativePath, chosen.Filename)
		h.launch(h.VLCPath, fullPath)
	}

	if isHTMX(c) {
		updated, err := q.GetMediaByID(ctx, chosen.ID)
		if err != nil {
			updated = chosen
		}
		s, _ := db.FetchSingleMediaStats(ctx, h.DB, chosen.ID)
		c.Header("Content-Type", "text/html; charset=utf-8")
		// No flash message (Bug 16): the response is only the OOB row, which
		// HTMX swaps by id. The hx-target (#flash) receives the empty remainder
		// and is therefore cleared.
		templates.MediaRowOOB(db.MediaWithStats{Medium: updated, MediaStats: s}).Render(ctx, c.Writer)
		return
	}

	c.Redirect(http.StatusSeeOther, listURL)
}

func (h *Handlers) SetTranslationType(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	ctx := c.Request.Context()
	q := db.New(h.DB)

	translationType := c.PostForm("type")
	_ = q.UpdateTranslationType(ctx, db.UpdateTranslationTypeParams{
		TranslationType: translationType,
		ID:              id,
	})

	if isHTMX(c) {
		media, err := q.GetMediaByID(ctx, id)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		stats, _ := db.FetchMediaStats(ctx, h.DB)
		c.Header("Content-Type", "text/html; charset=utf-8")
		templates.MediaRow(db.MediaWithStats{Medium: media, MediaStats: stats[id]}).Render(ctx, c.Writer)
		return
	}
	c.Redirect(http.StatusSeeOther, "/")
}

// SetForDeletion toggles the "за изтриване" flag (Bug 22). The submitted value
// is "1" to raise it and "0" to clear it; the template decides which one to send.
func (h *Handlers) SetForDeletion(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	ctx := c.Request.Context()
	q := db.New(h.DB)

	var forDeletion int64
	if c.PostForm("value") == "1" {
		forDeletion = 1
	}
	_ = q.UpdateForDeletion(ctx, db.UpdateForDeletionParams{
		ForDeletion: forDeletion,
		ID:          id,
	})

	if isHTMX(c) {
		media, err := q.GetMediaByID(ctx, id)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		s, _ := db.FetchSingleMediaStats(ctx, h.DB, id)
		c.Header("Content-Type", "text/html; charset=utf-8")
		templates.MediaRow(db.MediaWithStats{Medium: media, MediaStats: s}).Render(ctx, c.Writer)
		return
	}
	c.Redirect(http.StatusSeeOther, "/")
}

func (h *Handlers) StartMedia(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	ctx := c.Request.Context()
	q := db.New(h.DB)

	media, err := q.GetMediaByID(ctx, id)
	if err == sql.ErrNoRows {
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		h.log().Error("get media failed", "id", id, "err", err)
		c.String(http.StatusInternalServerError, "db error: %v", err)
		return
	}

	_ = q.InsertStartEvent(ctx, id)

	if media.CurrentStatus == "new" {
		_ = q.InsertStatusChange(ctx, db.InsertStatusChangeParams{
			MediaID:    id,
			FromStatus: sql.NullString{String: "new", Valid: true},
			ToStatus:   "started",
		})
		_ = q.UpdateMediaStatus(ctx, db.UpdateMediaStatusParams{
			CurrentStatus: "started",
			ID:            id,
		})
	}

	h.log().Info("start media", "id", id, "file", media.Filename)

	if h.VLCPath != "" {
		fullPath := filepath.Join(h.DiskRoot, media.FolderRelativePath, media.Filename)
		h.launch(h.VLCPath, fullPath)
	}

	if isHTMX(c) {
		updated, err := q.GetMediaByID(ctx, id)
		if err != nil {
			updated = media
		}
		s, _ := db.FetchSingleMediaStats(ctx, h.DB, id)
		c.Header("Content-Type", "text/html; charset=utf-8")
		templates.MediaRow(db.MediaWithStats{Medium: updated, MediaStats: s}).Render(ctx, c.Writer)
		return
	}
	c.Redirect(http.StatusSeeOther, "/")
}

func (h *Handlers) ChangeStatus(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}

	newStatus := c.PostForm("status")
	if !validStatuses[newStatus] {
		// Bug 27: a removed status (an old form, a stale tab or a hand-made
		// request) must not get back into the database.
		c.Status(http.StatusBadRequest)
		return
	}

	ctx := c.Request.Context()
	q := db.New(h.DB)

	media, err := q.GetMediaByID(ctx, id)
	if err == sql.ErrNoRows {
		c.Status(http.StatusNotFound)
		return
	}
	if err != nil {
		h.log().Error("get media failed", "id", id, "err", err)
		c.String(http.StatusInternalServerError, "db error: %v", err)
		return
	}

	if media.CurrentStatus != newStatus {
		_ = q.InsertStatusChange(ctx, db.InsertStatusChangeParams{
			MediaID:    id,
			FromStatus: sql.NullString{String: media.CurrentStatus, Valid: true},
			ToStatus:   newStatus,
		})
		_ = q.UpdateMediaStatus(ctx, db.UpdateMediaStatusParams{
			CurrentStatus: newStatus,
			ID:            id,
		})
		h.log().Info("status change", "id", id, "from", media.CurrentStatus, "to", newStatus)
	}

	if isHTMX(c) {
		updated, err := q.GetMediaByID(ctx, id)
		if err != nil {
			updated = media
			updated.CurrentStatus = newStatus
		}
		s, _ := db.FetchSingleMediaStats(ctx, h.DB, id)
		c.Header("Content-Type", "text/html; charset=utf-8")
		templates.MediaRow(db.MediaWithStats{Medium: updated, MediaStats: s}).Render(ctx, c.Writer)
		return
	}
	c.Redirect(http.StatusSeeOther, "/")
}

func (h *Handlers) OpenFolder(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	ctx := c.Request.Context()
	q := db.New(h.DB)
	media, err := q.GetMediaByID(ctx, id)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	folderPath := filepath.Join(h.DiskRoot, media.FolderRelativePath)
	h.launch("explorer.exe", folderPath)
	if isHTMX(c) {
		c.Status(http.StatusOK)
		return
	}
	c.Redirect(http.StatusSeeOther, "/")
}

func (h *Handlers) MediaDetail(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	ctx := c.Request.Context()
	q := db.New(h.DB)
	medium, err := q.GetMediaByID(ctx, id)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	starts, _ := q.GetStartEvents(ctx, id)
	changes, _ := q.GetStatusChanges(ctx, id)
	c.Header("Content-Type", "text/html; charset=utf-8")
	templates.DetailPage(medium, starts, changes).Render(ctx, c.Writer)
}

func (h *Handlers) Tree(c *gin.Context) {
	q := db.New(h.DB)
	media, err := q.ListOnDiskMedia(c.Request.Context())
	if err != nil {
		h.log().Error("list media for tree failed", "err", err)
		c.String(http.StatusInternalServerError, "db error: %v", err)
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	templates.TreePage(tree.Build(media)).Render(c.Request.Context(), c.Writer)
}

func (h *Handlers) Scan(c *gin.Context) {
	excludedDirs := []string{
		"$RECYCLE.BIN",
		"System Volume Information",
		"found.000",
		"Books",
		"Download",
		"Games",
		"Install",
		"LizaWork",
		"Music",
		"Sub",
		"Tatko",
		"Temp",
		"zzz",
	}

	go func() {
		files, err := scanner.Scan(h.DiskRoot, excludedDirs)
		if err != nil {
			h.log().Error("scan failed", "err", err)
			return
		}
		if err := db.SyncScanResults(h.DB, files); err != nil {
			h.log().Error("sync scan results failed", "err", err)
			return
		}
		h.log().Info("scan complete", "files", len(files))
	}()

	c.Redirect(http.StatusSeeOther, "/")
}
