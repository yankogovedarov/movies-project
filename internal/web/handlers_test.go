package web_test

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func newRouter(t *testing.T, d *sql.DB) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := &web.Handlers{DB: d, DiskRoot: t.TempDir(), VLCPath: ""}
	r := gin.New()
	r.GET("/", h.Index)
	r.GET("/tree", h.Tree)
	r.GET("/media/:id", h.MediaDetail)
	r.POST("/media/:id/start", h.StartMedia)
	r.POST("/media/:id/status", h.ChangeStatus)
	r.POST("/scan", h.Scan)
	return r
}

func seedMedia(t *testing.T, d *sql.DB, files []scanner.VideoFile) {
	t.Helper()
	require.NoError(t, db.SyncScanResults(d, files))
}

func TestIndexHandler_ReturnsHTML(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
}

func TestIndexHandler_ShowsMediaFilenames(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Godzilla.mkv", FolderRelativePath: "Action", SizeBytes: 1_200_000_000},
		{Filename: "Dune.Part.Two.mkv", FolderRelativePath: "SciFi", SizeBytes: 2_400_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Godzilla.mkv")
	assert.Contains(t, body, "Dune.Part.Two.mkv")
}

func TestIndexHandler_EmptyDB_ShowsNoMediaMessage(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "Няма намерени медии")
}

func TestIndex_ShowsFilenameLink(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Contains(t, w.Body.String(), "/media/")
}

func TestStartMedia_RecordsEventAndSetsStarted(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})
	q := db.New(d)
	media, err := q.ListOnDiskMedia(context.Background())
	require.NoError(t, err)
	require.Len(t, media, 1)
	id := media[0].ID

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/media/%d/start", id), nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)

	updated, err := q.GetMediaByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "started", updated.CurrentStatus)

	var eventCount int
	require.NoError(t, d.QueryRow("SELECT COUNT(*) FROM start_events WHERE media_id = ?", id).Scan(&eventCount))
	assert.Equal(t, 1, eventCount)
}

func TestStartMedia_RecordsEvent_WhenAlreadyStarted(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})
	q := db.New(d)
	media, err := q.ListOnDiskMedia(context.Background())
	require.NoError(t, err)
	id := media[0].ID

	require.NoError(t, q.UpdateMediaStatus(context.Background(), db.UpdateMediaStatusParams{
		CurrentStatus: "started",
		ID:            id,
	}))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/media/%d/start", id), nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)

	updated, err := q.GetMediaByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "started", updated.CurrentStatus)

	var eventCount int
	require.NoError(t, d.QueryRow("SELECT COUNT(*) FROM start_events WHERE media_id = ?", id).Scan(&eventCount))
	assert.Equal(t, 1, eventCount)
}

func TestStartMedia_Returns404_ForUnknownID(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/media/999999/start", nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestChangeStatus_UpdatesAndRecordsChange(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})
	q := db.New(d)
	media, err := q.ListOnDiskMedia(context.Background())
	require.NoError(t, err)
	id := media[0].ID

	body := "status=completed_both"
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/media/%d/status", id), nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Body = io.NopCloser(strings.NewReader(body))

	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)

	updated, err := q.GetMediaByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "completed_both", updated.CurrentStatus)

	var changeCount int
	require.NoError(t, d.QueryRow("SELECT COUNT(*) FROM status_changes WHERE media_id = ?", id).Scan(&changeCount))
	assert.Equal(t, 1, changeCount)
}

func TestChangeStatus_Returns404_ForUnknownID(t *testing.T) {
	d := openTestDB(t)
	body := "status=completed_both"
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/media/999999/status", nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Body = io.NopCloser(strings.NewReader(body))

	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestChangeStatus_NoOp_WhenSameStatus(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})
	q := db.New(d)
	media, err := q.ListOnDiskMedia(context.Background())
	require.NoError(t, err)
	id := media[0].ID
	currentStatus := media[0].CurrentStatus

	body := fmt.Sprintf("status=%s", currentStatus)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/media/%d/status", id), nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Body = io.NopCloser(strings.NewReader(body))

	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)

	var changeCount int
	require.NoError(t, d.QueryRow("SELECT COUNT(*) FROM status_changes WHERE media_id = ?", id).Scan(&changeCount))
	assert.Equal(t, 0, changeCount)
}

func TestIndex_ShowsStatusDropdown(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, `<select name="status"`)
	assert.Contains(t, body, `value="completed_both"`)
}

func TestIndex_HasResponsiveTableClasses(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "VeryLongMovieTitle.mkv", FolderRelativePath: "Films/Action", SizeBytes: 2_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, `class="filename"`)
	assert.Contains(t, body, `class="folder"`)
	assert.Contains(t, body, `class="size"`)
	assert.Contains(t, body, `class="status"`)
	assert.Contains(t, body, `class="actions"`)
	assert.Contains(t, body, `title="VeryLongMovieTitle.mkv"`)
	assert.Contains(t, body, `title="Films/Action"`)
}

func TestScan_ReturnsRedirect(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/scan", nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/", w.Header().Get("Location"))
}

func TestScan_DoesNotBlockRequest(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/scan", nil)

	start := time.Now()
	newRouter(t, d).ServeHTTP(w, req)
	duration := time.Since(start)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Less(t, duration, 100*time.Millisecond, "scan handler should return quickly")
}

func TestIndex_ShowsScanButton(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, `action="/scan"`)
	assert.Contains(t, body, "Сканирай")
}

func TestHandlers_WorkWithNilLogger(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})
	q := db.New(d)
	media, err := q.ListOnDiskMedia(context.Background())
	require.NoError(t, err)
	id := media[0].ID

	router := newRouter(t, d)

	// Index
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusOK, w.Code)

	// StartMedia
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, fmt.Sprintf("/media/%d/start", id), nil))
	assert.Equal(t, http.StatusSeeOther, w.Code)

	// ChangeStatus
	body := fmt.Sprintf("status=completed_both")
	w = httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/media/%d/status", id), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusSeeOther, w.Code)

	// Scan
	w = httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/scan", nil))
	assert.Equal(t, http.StatusSeeOther, w.Code)
}

func TestMediaDetail_Returns200(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Inception.mkv", FolderRelativePath: "SciFi", SizeBytes: 1_500_000_000},
	})
	q := db.New(d)
	media, err := q.ListOnDiskMedia(context.Background())
	require.NoError(t, err)
	id := media[0].ID

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/media/%d", id), nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
}

func TestMediaDetail_NotFound(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/media/999999", nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMediaDetail_ShowsFilename(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Dune.Part.Two.mkv", FolderRelativePath: "SciFi", SizeBytes: 2_400_000_000},
	})
	q := db.New(d)
	media, err := q.ListOnDiskMedia(context.Background())
	require.NoError(t, err)
	id := media[0].ID

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/media/%d", id), nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Contains(t, w.Body.String(), "Dune.Part.Two.mkv")
}

func TestTreeHandler_ReturnsHTML(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tree", nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
}

func TestTreeHandler_ShowsFolderNames(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "A.mkv", FolderRelativePath: "Action", SizeBytes: 1_000_000_000},
		{Filename: "B.mkv", FolderRelativePath: "Drama", SizeBytes: 1_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tree", nil)
	newRouter(t, d).ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "Action")
	assert.Contains(t, body, "Drama")
}
