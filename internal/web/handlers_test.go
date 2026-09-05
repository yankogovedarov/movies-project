package web_test

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strconv"
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
	r.POST("/media/random-new/start", h.RandomNew)
	r.POST("/media/:id/translation-type", h.SetTranslationType)
	r.POST("/media/:id/for-deletion", h.SetForDeletion)
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
	assert.Contains(t, w.Body.String(), `<tr id="media-`)
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
	assert.Contains(t, w.Body.String(), `<tr id="media-`)
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

func TestIndex_FilterTransNone_ShowsOnlyUnset(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Unset.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000, TranslationType: ""},
		{Filename: "Subbed.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000, TranslationType: "sub"},
		{Filename: "Dubbed.mkv", FolderRelativePath: "Films", SizeBytes: 3_000_000_000, TranslationType: "bg"},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?trans=none", nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Unset.mkv")
	assert.NotContains(t, body, "Subbed.mkv")
	assert.NotContains(t, body, "Dubbed.mkv")
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

func startEventCount(t *testing.T, d *sql.DB, id int64) int {
	t.Helper()
	var n int
	require.NoError(t, d.QueryRow("SELECT COUNT(*) FROM start_events WHERE media_id = ?", id).Scan(&n))
	return n
}

func mediaIDByName(t *testing.T, d *sql.DB, name string) int64 {
	t.Helper()
	q := db.New(d)
	media, err := q.ListAllMedia(context.Background())
	require.NoError(t, err)
	for _, m := range media {
		if m.Filename == name {
			return m.ID
		}
	}
	t.Fatalf("media %q not found", name)
	return 0
}

func TestRandomNew_PicksFromFilteredStatus(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "YankoPick.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
		{Filename: "Other1.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000},
		{Filename: "Other2.mkv", FolderRelativePath: "Films", SizeBytes: 3_000_000_000},
	})
	q := db.New(d)

	yankoID := mediaIDByName(t, d, "YankoPick.mkv")
	other1ID := mediaIDByName(t, d, "Other1.mkv")
	other2ID := mediaIDByName(t, d, "Other2.mkv")

	require.NoError(t, q.UpdateMediaStatus(context.Background(), db.UpdateMediaStatusParams{
		CurrentStatus: "completed_yanko",
		ID:            yankoID,
	}))
	require.NoError(t, q.UpdateMediaStatus(context.Background(), db.UpdateMediaStatusParams{
		CurrentStatus: "started",
		ID:            other1ID,
	}))
	// other2 stays "new"

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/media/random-new/start", strings.NewReader("status=completed_yanko&disk=on&trans=all"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	newRouter(t, d).ServeHTTP(w, req)

	// Only the yanko-status media is a candidate, so the pick is deterministic.
	assert.GreaterOrEqual(t, startEventCount(t, d, yankoID), 1, "filtered (yanko) media should have a start event")
	assert.Equal(t, 0, startEventCount(t, d, other1ID), "non-matching media should not be started")
	assert.Equal(t, 0, startEventCount(t, d, other2ID), "non-matching media should not be started")
}

func TestRandomNew_NewBecomesStarted(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "FreshMovie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})
	q := db.New(d)
	id := mediaIDByName(t, d, "FreshMovie.mkv")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/media/random-new/start", strings.NewReader("status=new&disk=on&trans=all"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	newRouter(t, d).ServeHTTP(w, req)

	updated, err := q.GetMediaByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "started", updated.CurrentStatus)
	assert.GreaterOrEqual(t, startEventCount(t, d, id), 1)
}

func TestRandomNew_KeepsCompletedStatus(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "DoneMovie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})
	q := db.New(d)
	id := mediaIDByName(t, d, "DoneMovie.mkv")
	require.NoError(t, q.UpdateMediaStatus(context.Background(), db.UpdateMediaStatusParams{
		CurrentStatus: "completed_yanko",
		ID:            id,
	}))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/media/random-new/start", strings.NewReader("status=completed_yanko&disk=on&trans=all"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	newRouter(t, d).ServeHTTP(w, req)

	updated, err := q.GetMediaByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "completed_yanko", updated.CurrentStatus, "completed status must be preserved")
	assert.GreaterOrEqual(t, startEventCount(t, d, id), 1, "a start event must still be recorded")
}

func TestRandomNew_HTMX_ReturnsOOBRow(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "OobMovie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/media/random-new/start", strings.NewReader("status=all&disk=on&trans=all"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	body := w.Body.String()
	assert.Contains(t, body, `hx-swap-oob="true"`)
	assert.Contains(t, body, `id="media-`)
	assert.Contains(t, body, "OobMovie.mkv")
	assert.NotContains(t, body, "Стартирах", "flash success message must be removed (Bug 16)")
}

func TestRandomNew_HTMX_DoesNotRedirect(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "NoRedirect.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/media/random-new/start", strings.NewReader("status=all&disk=on&trans=all"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	newRouter(t, d).ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusSeeOther, w.Code)
	assert.Empty(t, w.Header().Get("Location"))
}

func TestRandomNew_NoCandidates_HTMX(t *testing.T) {
	d := openTestDB(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/media/random-new/start", strings.NewReader("status=all&disk=on&trans=all"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, w.Header().Get("Location"))
	assert.Contains(t, w.Body.String(), "Няма филми")
}

func TestRandomNew_LogsIndexAndPoolSize(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "LogA.mkv", FolderRelativePath: "Alpha", SizeBytes: 1_000_000_000},
		{Filename: "LogB.mkv", FolderRelativePath: "Beta", SizeBytes: 2_000_000_000},
		{Filename: "LogC.mkv", FolderRelativePath: "Gamma", SizeBytes: 3_000_000_000},
	})

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	h := &web.Handlers{DB: d, DiskRoot: t.TempDir(), VLCPath: "", Log: logger}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/media/random-new/start", h.RandomNew)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/media/random-new/start", strings.NewReader("status=all&disk=on&trans=all"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)

	logOutput := buf.String()
	assert.Contains(t, logOutput, "pool=3", "log must report the size of the random pool")

	m := regexp.MustCompile(`index=([0-9]+)`).FindStringSubmatch(logOutput)
	require.Len(t, m, 2, "log must report the chosen random index")
	idx, err := strconv.Atoi(m[1])
	require.NoError(t, err)
	assert.GreaterOrEqual(t, idx, 0, "index must be >= 0")
	assert.LessOrEqual(t, idx, 2, "index must be <= pool-1")
}

// markForDeletion raises the "за изтриване" flag directly in the DB, mirroring
// the raw-SQL setup used for on_disk in the disk-filter tests.
func markForDeletion(t *testing.T, d *sql.DB, filename string) {
	t.Helper()
	_, err := d.Exec("UPDATE media SET for_deletion = 1 WHERE filename = ?", filename)
	require.NoError(t, err)
}

func forDeletionFlag(t *testing.T, d *sql.DB, id int64) int64 {
	t.Helper()
	var v int64
	require.NoError(t, d.QueryRow("SELECT for_deletion FROM media WHERE id = ?", id).Scan(&v))
	return v
}

func TestIndex_FilterForDeletion_ShowsOnlyMarked(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Keep.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
		{Filename: "Trash.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000},
	})
	markForDeletion(t, d, "Trash.mkv")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?del=yes", nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Trash.mkv")
	assert.NotContains(t, body, "Keep.mkv")
}

func TestIndex_DefaultDelFilter_ShowsBoth(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Keep.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
		{Filename: "Trash.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000},
	})
	markForDeletion(t, d, "Trash.mkv")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "Keep.mkv")
	assert.Contains(t, body, "Trash.mkv")
}

// The flag is independent of the watch status: a completed media can also be
// marked for deletion, so the two filters combine instead of excluding.
func TestIndex_ForDeletionCombinesWithStatus(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "DoneTrash.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
		{Filename: "DoneKeep.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000},
		{Filename: "NewTrash.mkv", FolderRelativePath: "Films", SizeBytes: 3_000_000_000},
	})
	q := db.New(d)
	for _, name := range []string{"DoneTrash.mkv", "DoneKeep.mkv"} {
		require.NoError(t, q.UpdateMediaStatus(context.Background(), db.UpdateMediaStatusParams{
			CurrentStatus: "completed_both",
			ID:            mediaIDByName(t, d, name),
		}))
	}
	markForDeletion(t, d, "DoneTrash.mkv")
	markForDeletion(t, d, "NewTrash.mkv")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?status=completed_both&del=yes", nil)
	newRouter(t, d).ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "DoneTrash.mkv")
	assert.NotContains(t, body, "DoneKeep.mkv")
	assert.NotContains(t, body, "NewTrash.mkv")
}

func TestSetForDeletion_TogglesFlag(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Film.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})
	id := mediaIDByName(t, d, "Film.mkv")
	r := newRouter(t, d)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/media/%d/for-deletion", id), strings.NewReader("value=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, int64(1), forDeletionFlag(t, d, id))

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, fmt.Sprintf("/media/%d/for-deletion", id), strings.NewReader("value=0"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)
	assert.Equal(t, int64(0), forDeletionFlag(t, d, id))
}

func TestSetForDeletion_HTMX_ReturnsRowOnly(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Film.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})
	id := mediaIDByName(t, d, "Film.mkv")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/media/%d/for-deletion", id), strings.NewReader("value=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "<tr")
	assert.NotContains(t, body, "<html", "HTMX response must be the row only")
	assert.Equal(t, int64(1), forDeletionFlag(t, d, id))
}

func TestSetForDeletion_InvalidID_Returns404(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/media/abc/for-deletion", strings.NewReader("value=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	newRouter(t, d).ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRandomNew_RespectsForDeletionFilter(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Trash.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
		{Filename: "Other1.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000},
		{Filename: "Other2.mkv", FolderRelativePath: "Films", SizeBytes: 3_000_000_000},
	})
	markForDeletion(t, d, "Trash.mkv")
	trashID := mediaIDByName(t, d, "Trash.mkv")
	r := newRouter(t, d)

	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/media/random-new/start",
			strings.NewReader("status=all&disk=on&trans=all&del=yes"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.ServeHTTP(w, req)
	}

	assert.Equal(t, 10, startEventCount(t, d, trashID), "only the marked media may be picked")
	assert.Equal(t, 0, startEventCount(t, d, mediaIDByName(t, d, "Other1.mkv")))
	assert.Equal(t, 0, startEventCount(t, d, mediaIDByName(t, d, "Other2.mkv")))
}

// --- Bug 23: filters and sorting are remembered in the database ---

func readPrefs(t *testing.T, d *sql.DB) (status, disk, sortF, dir, q, trans, del string) {
	t.Helper()
	row := d.QueryRow(`SELECT status_filter, disk_filter, sort_filter, dir_filter,
		q_filter, trans_filter, del_filter FROM ui_prefs WHERE id = 1`)
	require.NoError(t, row.Scan(&status, &disk, &sortF, &dir, &q, &trans, &del))
	return
}

func TestIndex_SavesFiltersToDB(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Alpha.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/?status=new&disk=all&sort=size&dir=desc&q=alp&trans=bg&del=yes", nil)
	newRouter(t, d).ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	status, disk, sortF, dir, q, trans, del := readPrefs(t, d)
	assert.Equal(t, "new", status)
	assert.Equal(t, "all", disk)
	assert.Equal(t, "size", sortF)
	assert.Equal(t, "desc", dir)
	assert.Equal(t, "alp", q)
	assert.Equal(t, "bg", trans)
	assert.Equal(t, "yes", del)
}

func TestIndex_RestoresSavedFilters(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Keep.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
		{Filename: "Trash.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000},
	})
	markForDeletion(t, d, "Trash.mkv")

	// The user filters down to the media marked for deletion...
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?del=yes", nil)
	newRouter(t, d).ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// ...restarts the backend, and lands on a bare "/" again.
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Trash.mkv")
	assert.NotContains(t, body, "Keep.mkv", "the remembered del=yes filter must still apply")
}

func TestIndex_RestoresSavedSortAndDirection(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Small.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
		{Filename: "Big.mkv", FolderRelativePath: "Films", SizeBytes: 5_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?sort=size&dir=desc", nil)
	newRouter(t, d).ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	body := w.Body.String()
	assert.Less(t, strings.Index(body, "Big.mkv"), strings.Index(body, "Small.mkv"),
		"the remembered sort=size&dir=desc must put the biggest file first")
}

func TestIndex_RestoresSavedSearchQuery(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Godzilla.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
		{Filename: "Dune.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?q=dune", nil)
	newRouter(t, d).ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "Dune.mkv")
	assert.NotContains(t, body, "Godzilla.mkv", "the remembered search text must still apply")
}

func TestIndex_ExplicitParamOverridesSavedPref(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Keep.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
		{Filename: "Trash.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000},
	})
	markForDeletion(t, d, "Trash.mkv")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?del=yes", nil)
	newRouter(t, d).ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// An explicit parameter wins over the remembered one and replaces it.
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/?del=all", nil)
	newRouter(t, d).ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "Keep.mkv")
	assert.Contains(t, body, "Trash.mkv")

	_, _, _, _, _, _, del := readPrefs(t, d)
	assert.Equal(t, "all", del, "the explicit value must be the new remembered one")
}

func TestIndex_FreshDB_UsesDefaultFilters(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Keep.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
		{Filename: "Trash.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000},
	})
	markForDeletion(t, d, "Trash.mkv")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "Keep.mkv")
	assert.Contains(t, body, "Trash.mkv")

	status, disk, sortF, dir, q, trans, del := readPrefs(t, d)
	assert.Equal(t, []string{"all", "on", "name", "asc", "", "all", "all"},
		[]string{status, disk, sortF, dir, q, trans, del})
}

// ---------------------------------------------------------------------------
// Bug 24: тестовете не трябва да пускат VLC или Explorer
// ---------------------------------------------------------------------------

// launchRecorder заменя реалното стартиране на външна програма и запомня какво
// е щяло да бъде пуснато.
type launchRecorder struct {
	calls [][]string
}

func (r *launchRecorder) launch(name string, args ...string) error {
	r.calls = append(r.calls, append([]string{name}, args...))
	return nil
}

func newRouterWithHandlers(t *testing.T, h *web.Handlers) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/media/:id/open-folder", h.OpenFolder)
	r.POST("/media/:id/start", h.StartMedia)
	r.POST("/media/random-new/start", h.RandomNew)
	return r
}

func seedOne(t *testing.T, d *sql.DB) int64 {
	t.Helper()
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})
	media, err := db.New(d).ListOnDiskMedia(context.Background())
	require.NoError(t, err)
	return media[0].ID
}

func TestOpenFolder_UsesLauncherInsteadOfSpawningExplorer(t *testing.T) {
	d := openTestDB(t)
	id := seedOne(t, d)
	root := t.TempDir()
	rec := &launchRecorder{}
	h := &web.Handlers{DB: d, DiskRoot: root, Launcher: rec.launch}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/media/%d/open-folder", id), nil)
	newRouterWithHandlers(t, h).ServeHTTP(w, req)

	require.Len(t, rec.calls, 1)
	assert.Equal(t, []string{"explorer.exe", filepath.Join(root, "Films")}, rec.calls[0])
}

func TestOpenFolder_NilLauncher_DoesNotLaunch(t *testing.T) {
	d := openTestDB(t)
	id := seedOne(t, d)
	h := &web.Handlers{DB: d, DiskRoot: t.TempDir()}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/media/%d/open-folder", id), nil)
	newRouterWithHandlers(t, h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
}

func TestStartMedia_LaunchesVLCThroughLauncher(t *testing.T) {
	d := openTestDB(t)
	id := seedOne(t, d)
	root := t.TempDir()
	rec := &launchRecorder{}
	h := &web.Handlers{DB: d, DiskRoot: root, VLCPath: `C:\fake\vlc.exe`, Launcher: rec.launch}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/media/%d/start", id), nil)
	newRouterWithHandlers(t, h).ServeHTTP(w, req)

	require.Len(t, rec.calls, 1)
	assert.Equal(t, []string{`C:\fake\vlc.exe`, filepath.Join(root, "Films", "Movie.mkv")}, rec.calls[0])
}

func TestStartMedia_WithoutVLCPath_DoesNotLaunch(t *testing.T) {
	d := openTestDB(t)
	id := seedOne(t, d)
	rec := &launchRecorder{}
	h := &web.Handlers{DB: d, DiskRoot: t.TempDir(), VLCPath: "", Launcher: rec.launch}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/media/%d/start", id), nil)
	newRouterWithHandlers(t, h).ServeHTTP(w, req)

	assert.Empty(t, rec.calls)
}

func TestRandomNew_LaunchesVLCThroughLauncher(t *testing.T) {
	d := openTestDB(t)
	seedOne(t, d)
	root := t.TempDir()
	rec := &launchRecorder{}
	h := &web.Handlers{DB: d, DiskRoot: root, VLCPath: `C:\fake\vlc.exe`, Launcher: rec.launch}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/media/random-new/start", strings.NewReader("status=all&disk=on&trans=all"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	newRouterWithHandlers(t, h).ServeHTTP(w, req)

	require.Len(t, rec.calls, 1)
	assert.Equal(t, []string{`C:\fake\vlc.exe`, filepath.Join(root, "Films", "Movie.mkv")}, rec.calls[0])
}

func TestNoLaunch_TrueWhenEnvIsOne(t *testing.T) {
	t.Setenv("MOVIETRACKER_NO_LAUNCH", "1")
	assert.True(t, web.NoLaunch())
}

func TestNoLaunch_FalseWhenEnvIsUnset(t *testing.T) {
	t.Setenv("MOVIETRACKER_NO_LAUNCH", "")
	assert.False(t, web.NoLaunch())
}

func TestNoLaunch_FalseWhenEnvIsZero(t *testing.T) {
	t.Setenv("MOVIETRACKER_NO_LAUNCH", "0")
	assert.False(t, web.NoLaunch())
}

func TestNoopLauncher_DoesNothing(t *testing.T) {
	assert.NoError(t, web.NoopLauncher("explorer.exe", `C:\Windows`))
}
