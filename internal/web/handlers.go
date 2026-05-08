package web

import (
	"database/sql"
	"log/slog"
	"net/http"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yankogovedarov/movie-tracker/internal/db"
	"github.com/yankogovedarov/movie-tracker/internal/scanner"
	"github.com/yankogovedarov/movie-tracker/internal/tree"
	"github.com/yankogovedarov/movie-tracker/templates"
)

type Handlers struct {
	DB       *sql.DB
	DiskRoot string
	VLCPath  string
	Log      *slog.Logger
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

func (h *Handlers) Index(c *gin.Context) {
	statusFilter := c.DefaultQuery("status", "all")
	diskFilter := c.DefaultQuery("disk", "on")
	sortFilter := c.DefaultQuery("sort", "name")

	ctx := c.Request.Context()
	q := db.New(h.DB)

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

	if statusFilter != "all" {
		filtered := make([]db.Medium, 0, len(raw))
		for _, m := range raw {
			if m.CurrentStatus == statusFilter {
				filtered = append(filtered, m)
			}
		}
		raw = filtered
	}

	stats, err := db.FetchMediaStats(ctx, h.DB)
	if err != nil {
		h.log().Error("fetch media stats failed", "err", err)
		stats = make(map[int64]db.MediaStats)
	}

	media := make([]db.MediaWithStats, len(raw))
	for i, m := range raw {
		media[i] = db.MediaWithStats{Medium: m, MediaStats: stats[m.ID]}
	}

	switch sortFilter {
	case "last_started":
		sort.Slice(media, func(i, j int) bool {
			vi, vj := media[i].LastStartedAt, media[j].LastStartedAt
			if !vi.Valid && !vj.Valid {
				return media[i].Filename < media[j].Filename
			}
			if !vi.Valid {
				return false
			}
			if !vj.Valid {
				return true
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
			return ti.After(tj)
		})
	case "marked":
		sort.Slice(media, func(i, j int) bool {
			vi, vj := media[i].MarkedAt, media[j].MarkedAt
			if !vi.Valid && !vj.Valid {
				return media[i].Filename < media[j].Filename
			}
			if !vi.Valid {
				return false
			}
			if !vj.Valid {
				return true
			}
			return vi.Time.After(vj.Time)
		})
	default:
		sort.Slice(media, func(i, j int) bool {
			return media[i].Filename < media[j].Filename
		})
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	templates.ListPage(media, statusFilter, diskFilter, sortFilter).Render(ctx, c.Writer)
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
		_ = exec.Command(h.VLCPath, fullPath).Start()
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
	if newStatus == "" {
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
	_ = exec.Command("explorer.exe", folderPath).Start()
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
