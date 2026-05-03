package web_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yankogovedarov/movie-tracker/internal/db"
	"github.com/yankogovedarov/movie-tracker/internal/scanner"
	"github.com/yankogovedarov/movie-tracker/internal/web"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	tmpDir := t.TempDir()
	d, err := db.Open(filepath.Join(tmpDir, "test.db"))
	require.NoError(t, err)
	require.NoError(t, db.Migrate(d))
	t.Cleanup(func() { d.Close() })
	return d
}

func newRouter(d *sql.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/", web.IndexHandler(d))
	return r
}

func TestIndexHandler_ReturnsHTML(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
}

func TestIndexHandler_ShowsMediaFilenames(t *testing.T) {
	d := openTestDB(t)
	files := []scanner.VideoFile{
		{Filename: "Godzilla.mkv", FolderRelativePath: "Action", SizeBytes: 1_200_000_000},
		{Filename: "Dune.Part.Two.mkv", FolderRelativePath: "SciFi", SizeBytes: 2_400_000_000},
	}
	require.NoError(t, db.SyncScanResults(d, files))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Godzilla.mkv")
	assert.Contains(t, body, "Dune.Part.Two.mkv")
}

func TestIndexHandler_EmptyDB_ShowsNoMediaMessage(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Няма намерени медии")
}
