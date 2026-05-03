package web

import (
	"database/sql"
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/yankogovedarov/movie-tracker/internal/db"
	"github.com/yankogovedarov/movie-tracker/templates"
)

type Handlers struct {
	DB       *sql.DB
	DiskRoot string
	VLCPath  string
}

func (h *Handlers) Index(c *gin.Context) {
	q := db.New(h.DB)
	media, err := q.ListOnDiskMedia(c.Request.Context())
	if err != nil {
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

	if h.VLCPath != "" {
		fullPath := filepath.Join(h.DiskRoot, media.FolderRelativePath, media.Filename)
		_ = exec.Command(h.VLCPath, fullPath).Start()
	}

	c.Redirect(http.StatusSeeOther, "/")
}
