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
	r.GET("/media/:id/open-folder", h.OpenFolder)
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

func TestIndex_ShowsStartButton(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Contains(t, w.Body.String(), "/start")
}

func TestOpenFolder_Returns303(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})
	q := db.New(d)
	media, err := q.ListOnDiskMedia(context.Background())
	require.NoError(t, err)
	id := media[0].ID

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/media/%d/open-folder", id), nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/", w.Header().Get("Location"))
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

func TestIndex_ShowsStatusIconButtons(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, `value="completed_both"`)
	assert.Contains(t, body, `class="icon-btn"`)
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

func TestStartMedia_HTMX_ReturnsRow(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})
	q := db.New(d)
	media, err := q.ListOnDiskMedia(context.Background())
	require.NoError(t, err)
	id := media[0].ID

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/media/%d/start", id), nil)
	req.Header.Set("HX-Request", "true")
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, w.Body.String(), "<tr>")
	assert.Contains(t, w.Body.String(), "Movie.mkv")
}

func TestChangeStatus_HTMX_ReturnsRow(t *testing.T) {
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
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/media/%d/status", id), strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, w.Body.String(), "<tr>")
}

func TestOpenFolder_HTMX_Returns200(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})
	q := db.New(d)
	media, err := q.ListOnDiskMedia(context.Background())
	require.NoError(t, err)
	id := media[0].ID

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/media/%d/open-folder", id), nil)
	req.Header.Set("HX-Request", "true")
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
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

func TestIndex_FilterByStatus_ReturnsOnlyMatching(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "New.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
		{Filename: "Started.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000},
	})
	q := db.New(d)
	media, err := q.ListOnDiskMedia(context.Background())
	require.NoError(t, err)
	var startedID int64
	for _, m := range media {
		if m.Filename == "Started.mkv" {
			startedID = m.ID
		}
	}
	require.NoError(t, q.UpdateMediaStatus(context.Background(), db.UpdateMediaStatusParams{
		CurrentStatus: "started",
		ID:            startedID,
	}))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?status=new", nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "New.mkv")
	assert.NotContains(t, w.Body.String(), "Started.mkv")
}

func TestIndex_FilterDisk_All_IncludesOffDisk(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "OnDisk.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
		{Filename: "OffDisk.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000},
	})
	_, err := d.Exec("UPDATE media SET on_disk = 0 WHERE filename = 'OffDisk.mkv'")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?disk=all", nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "OnDisk.mkv")
	assert.Contains(t, w.Body.String(), "OffDisk.mkv")
}

func TestIndex_DefaultFilter_ExcludesOffDisk(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "OnDisk.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
		{Filename: "OffDisk.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000},
	})
	_, err := d.Exec("UPDATE media SET on_disk = 0 WHERE filename = 'OffDisk.mkv'")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "OnDisk.mkv")
	assert.NotContains(t, w.Body.String(), "OffDisk.mkv")
}

func TestIndex_SortDefault_AlphabeticalByFilename(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Zebra.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
		{Filename: "Alpha.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	body := w.Body.String()
	alphaIdx := strings.Index(body, "Alpha.mkv")
	zebraIdx := strings.Index(body, "Zebra.mkv")
	assert.Greater(t, alphaIdx, -1, "Alpha.mkv should be in body")
	assert.Less(t, alphaIdx, zebraIdx, "Alpha.mkv should appear before Zebra.mkv in default sort")
}

func TestIndex_SortByLastStarted_StartedMediaFirst(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "NotStarted.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
		{Filename: "Started.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000},
	})
	q := db.New(d)
	media, err := q.ListOnDiskMedia(context.Background())
	require.NoError(t, err)
	var startedID int64
	for _, m := range media {
		if m.Filename == "Started.mkv" {
			startedID = m.ID
		}
	}
	require.NotZero(t, startedID)
	require.NoError(t, q.InsertStartEvent(context.Background(), startedID))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?sort=last_started", nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	startedIdx := strings.Index(body, "Started.mkv")
	notStartedIdx := strings.Index(body, "NotStarted.mkv")
	assert.Less(t, startedIdx, notStartedIdx, "Started.mkv should appear before NotStarted.mkv when sorting by last_started")
}

func TestIndex_SortByAdded_Returns200WithAllMedia(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "MovieA.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
		{Filename: "MovieB.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?sort=added", nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "MovieA.mkv")
	assert.Contains(t, body, "MovieB.mkv")
}

func TestIndex_SortByAdded_OrdersByFileCreatedAt(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "OlderFile.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
		{Filename: "NewerFile.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000},
	})

	older := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	_, err := d.Exec("UPDATE media SET file_created_at = ? WHERE filename = 'OlderFile.mkv'", older)
	require.NoError(t, err)
	_, err = d.Exec("UPDATE media SET file_created_at = ? WHERE filename = 'NewerFile.mkv'", newer)
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?sort=added&dir=desc", nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	newerIdx := strings.Index(body, "NewerFile.mkv")
	olderIdx := strings.Index(body, "OlderFile.mkv")
	assert.Greater(t, newerIdx, -1, "NewerFile.mkv should be in body")
	assert.Less(t, newerIdx, olderIdx, "NewerFile.mkv (2024) should appear before OlderFile.mkv (2023) with dir=desc")
}

func TestIndex_SortByMarked_Returns200WithAllMedia(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "MovieC.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?sort=marked", nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "MovieC.mkv")
}
