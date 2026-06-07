package web

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed static
var staticFiles embed.FS

func (h *Handlers) StaticFile(c *gin.Context) {
	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	http.StripPrefix("/static/", http.FileServer(http.FS(sub))).ServeHTTP(c.Writer, c.Request)
}
