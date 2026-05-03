package web

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/yankogovedarov/movie-tracker/internal/db"
	"github.com/yankogovedarov/movie-tracker/templates"
)

func IndexHandler(database *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := db.New(database)
		media, err := q.ListOnDiskMedia(c.Request.Context())
		if err != nil {
			c.String(http.StatusInternalServerError, "db error: %v", err)
			return
		}
		c.Header("Content-Type", "text/html; charset=utf-8")
		templates.ListPage(media).Render(c.Request.Context(), c.Writer)
	}
}
