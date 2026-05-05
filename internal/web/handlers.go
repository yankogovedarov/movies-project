package web

import (
	"database/sql"
	"log/slog"
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yankogovedarov/movie-tracker/internal/db"
	"github.com/yankogovedarov/movie-tracker/internal/scanner"
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

func (h *Handlers) Index(c *gin.Context) {
	q := db.New(h.DB)
	media, err := q.ListOnDiskMedia(c.Request.Context())
	if err != nil {
		h.log().Error("list media failed", "err", err)
		c.String(http.StatusInternalServerError, "db error: %v", err)
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	templates.ListPage(media).Render(c.Request.Context(), c.Writer)
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

	c.Redirect(http.StatusSeeOther, "/")
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
