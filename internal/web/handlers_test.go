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
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
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
	r.GET("/media/:id/open-folder", h.OpenFolder)
	r.POST("/media/:id/start", h.StartMedia)
	r.POST("/media/:id/status", h.ChangeStatus)
	r.POST("/media/random-new/start", h.RandomNew)
	r.POST("/media/:id/for-dnd", h.SetForDnd)
	r.POST("/media/:id/for-deletion", h.SetForDeletion)
	r.POST("/media/:id/wrong-subs", h.SetWrongSubs)
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
	assert.Equal(t, 0, statusChangeCount(t, d, id), "only new → started writes a history row")
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

	body := "status=started"
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/media/%d/status", id), nil)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Body = io.NopCloser(strings.NewReader(body))

	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)

	updated, err := q.GetMediaByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "started", updated.CurrentStatus)

	var changeCount int
	require.NoError(t, d.QueryRow("SELECT COUNT(*) FROM status_changes WHERE media_id = ?", id).Scan(&changeCount))
	assert.Equal(t, 1, changeCount)
}

func TestChangeStatus_Returns404_ForUnknownID(t *testing.T) {
	d := openTestDB(t)
	body := "status=started"
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
	assert.Contains(t, body, `value="started"`)
	assert.NotContains(t, body, `value="for_dnd"`, "Bug 30: „За DnD\" is a flag, not a status")
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
	body := "status=new"
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

	body := "status=started"
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
		{Filename: "StartedPick.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
		{Filename: "Other1.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000},
		{Filename: "Other2.mkv", FolderRelativePath: "Films", SizeBytes: 3_000_000_000},
	})
	q := db.New(d)

	startedID := mediaIDByName(t, d, "StartedPick.mkv")
	other1ID := mediaIDByName(t, d, "Other1.mkv")
	other2ID := mediaIDByName(t, d, "Other2.mkv")

	require.NoError(t, q.UpdateMediaStatus(context.Background(), db.UpdateMediaStatusParams{
		CurrentStatus: "started",
		ID:            startedID,
	}))
	// other1 and other2 stay "new"

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/media/random-new/start", strings.NewReader("status=started&disk=on"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	newRouter(t, d).ServeHTTP(w, req)

	// Only the started media is a candidate, so the pick is deterministic.
	assert.GreaterOrEqual(t, startEventCount(t, d, startedID), 1, "filtered (started) media should have a start event")
	assert.Equal(t, 0, startEventCount(t, d, other1ID), "non-matching media should not be started")
	assert.Equal(t, 0, startEventCount(t, d, other2ID), "non-matching media should not be started")
	assert.Equal(t, 0, statusChangeCount(t, d, startedID), "🎲 must not record a status change for an already started media")
}

func TestRandomNew_NewBecomesStarted(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "FreshMovie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})
	q := db.New(d)
	id := mediaIDByName(t, d, "FreshMovie.mkv")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/media/random-new/start", strings.NewReader("status=new&disk=on"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	newRouter(t, d).ServeHTTP(w, req)

	updated, err := q.GetMediaByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "started", updated.CurrentStatus)
	assert.GreaterOrEqual(t, startEventCount(t, d, id), 1)
}

func TestRandomNew_HTMX_ReturnsOOBRow(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "OobMovie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/media/random-new/start", strings.NewReader("status=all&disk=on"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "text/html")
	body := w.Body.String()
	assert.Contains(t, body, `hx-swap-oob="true"`)
	assert.Contains(t, body, `id="media-`)
	assert.Contains(t, body, "OobMovie.mkv")
	assert.Contains(t, body, "🎲 Стартирах: OobMovie.mkv", "Bug 27: the flash names the started film again")
}

func TestRandomNew_HTMX_DoesNotRedirect(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "NoRedirect.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/media/random-new/start", strings.NewReader("status=all&disk=on"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	newRouter(t, d).ServeHTTP(w, req)

	assert.NotEqual(t, http.StatusSeeOther, w.Code)
	assert.Empty(t, w.Header().Get("Location"))
}

func TestRandomNew_NoCandidates_HTMX(t *testing.T) {
	d := openTestDB(t)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/media/random-new/start", strings.NewReader("status=all&disk=on"))
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
	req := httptest.NewRequest(http.MethodPost, "/media/random-new/start", strings.NewReader("status=all&disk=on"))
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

// The flag is independent of the watch status: a started media can also be
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
			CurrentStatus: "started",
			ID:            mediaIDByName(t, d, name),
		}))
	}
	markForDeletion(t, d, "DoneTrash.mkv")
	markForDeletion(t, d, "NewTrash.mkv")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?status=started&del=yes", nil)
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
			strings.NewReader("status=all&disk=on&del=yes"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.ServeHTTP(w, req)
	}

	assert.Equal(t, 10, startEventCount(t, d, trashID), "only the marked media may be picked")
	assert.Equal(t, 0, startEventCount(t, d, mediaIDByName(t, d, "Other1.mkv")))
	assert.Equal(t, 0, startEventCount(t, d, mediaIDByName(t, d, "Other2.mkv")))
}

// --- Bug 23: filters and sorting are remembered in the database ---

func readPrefs(t *testing.T, d *sql.DB) (status, disk, sortF, dir, q, del string) {
	t.Helper()
	row := d.QueryRow(`SELECT status_filter, disk_filter, sort_filter, dir_filter,
		q_filter, del_filter FROM ui_prefs WHERE id = 1`)
	require.NoError(t, row.Scan(&status, &disk, &sortF, &dir, &q, &del))
	return
}

func TestIndex_SavesFiltersToDB(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Alpha.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet,
		"/?status=new&disk=all&sort=size&dir=desc&q=alp&del=yes&dnd=yes&subs=yes", nil)
	newRouter(t, d).ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	status, disk, sortF, dir, q, del := readPrefs(t, d)
	assert.Equal(t, "new", status)
	assert.Equal(t, "all", disk)
	assert.Equal(t, "size", sortF)
	assert.Equal(t, "desc", dir)
	assert.Equal(t, "alp", q)
	assert.Equal(t, "yes", del)

	dnd, subs := readFlagPrefs(t, d)
	assert.Equal(t, "yes", dnd)
	assert.Equal(t, "yes", subs)
}

// Each of the three flag filters (👎 del, Bug 22; 👍 dnd and ⚠ subs, Bug 30)
// is remembered on its own.
func TestIndex_RestoresSavedFilters(t *testing.T) {
	for _, tc := range []struct{ param, column string }{
		{"del", "for_deletion"},
		{"dnd", "for_dnd"},
		{"subs", "wrong_subs"},
	} {
		t.Run(tc.param, func(t *testing.T) {
			d := openTestDB(t)
			seedMedia(t, d, []scanner.VideoFile{
				{Filename: "Keep.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
				{Filename: "Flagged.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000},
			})
			raiseFlag(t, d, tc.column, "Flagged.mkv")

			// The user filters down to the flagged media...
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/?"+tc.param+"=yes", nil)
			newRouter(t, d).ServeHTTP(w, req)
			require.Equal(t, http.StatusOK, w.Code)

			// ...restarts the backend, and lands on a bare "/" again.
			w = httptest.NewRecorder()
			req = httptest.NewRequest(http.MethodGet, "/", nil)
			newRouter(t, d).ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			body := w.Body.String()
			assert.Contains(t, body, "Flagged.mkv")
			assert.NotContains(t, body, "Keep.mkv", "the remembered %s=yes filter must still apply", tc.param)
		})
	}
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

	_, _, _, _, _, del := readPrefs(t, d)
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

	status, disk, sortF, dir, q, del := readPrefs(t, d)
	assert.Equal(t, []string{"all", "on", "name", "asc", "", "all"},
		[]string{status, disk, sortF, dir, q, del})
	assert.Equal(t, "all", readFolderPref(t, d), "the folder filter defaults to all folders")
	dnd, subs := readFlagPrefs(t, d)
	assert.Equal(t, []string{"all", "all"}, []string{dnd, subs}, "the 👍 and ⚠ filters default to all")
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
	req := httptest.NewRequest(http.MethodPost, "/media/random-new/start", strings.NewReader("status=all&disk=on"))
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

// ---------------------------------------------------------------------------
// Bug 26: филтър по основна папка
// ---------------------------------------------------------------------------

// readFolderPref returns the remembered main-folder filter. It is kept apart
// from readPrefs so the Bug 23 tests keep their six-value shape.
func readFolderPref(t *testing.T, d *sql.DB) string {
	t.Helper()
	var folder string
	require.NoError(t, d.QueryRow("SELECT folder_filter FROM ui_prefs WHERE id = 1").Scan(&folder))
	return folder
}

// seedMainFolders seeds one media under each of three main folders, with the
// backslash paths a real Windows scan stores. "MoviesDnD" deliberately starts
// with "Movies" to catch prefix matching. Extra files are seeded in the same
// scan, because every seedMedia call marks the earlier media off disk.
func seedMainFolders(t *testing.T, d *sql.DB, extra ...scanner.VideoFile) {
	t.Helper()
	files := []scanner.VideoFile{
		{Filename: "Tron.mkv", FolderRelativePath: `Movies\Tron Ares (2025)`, SizeBytes: 1_000_000_000},
		{Filename: "Dragons.mkv", FolderRelativePath: `MoviesDnD\Dragons`, SizeBytes: 2_000_000_000},
		{Filename: "Episode01.mkv", FolderRelativePath: `Serials\Show\Season 1`, SizeBytes: 3_000_000_000},
	}
	seedMedia(t, d, append(files, extra...))
}

// rootFile is a media directly in the disk root — it has no main folder.
var rootFile = scanner.VideoFile{Filename: "RootFile.mkv", FolderRelativePath: "", SizeBytes: 9_000_000_000}

func TestIndex_FolderFilter_ShowsOnlyThatFolder(t *testing.T) {
	d := openTestDB(t)
	seedMainFolders(t, d)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?folder=Serials", nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "Episode01.mkv", "media at any depth below the main folder must match")
	assert.NotContains(t, body, "Tron.mkv")
	assert.NotContains(t, body, "Dragons.mkv")
}

func TestIndex_FolderFilter_DoesNotMatchFolderPrefix(t *testing.T) {
	d := openTestDB(t)
	seedMainFolders(t, d)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?folder=Movies", nil)
	newRouter(t, d).ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "Tron.mkv")
	assert.NotContains(t, body, "Dragons.mkv", `"Movies" must not also match "MoviesDnD"`)
	assert.NotContains(t, body, "Episode01.mkv")
}

func TestIndex_FolderFilter_DefaultShowsAll(t *testing.T) {
	d := openTestDB(t)
	seedMainFolders(t, d, rootFile)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	body := w.Body.String()
	for _, name := range []string{"Tron.mkv", "Dragons.mkv", "Episode01.mkv", "RootFile.mkv"} {
		assert.Contains(t, body, name)
	}
}

func TestIndex_FolderFilter_EmptyParamMeansAll(t *testing.T) {
	d := openTestDB(t)
	seedMainFolders(t, d, rootFile)

	// A remembered folder...
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?folder=Serials", nil)
	newRouter(t, d).ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// ...is replaced by an explicit empty "folder=", which means "all folders"
	// and not "only the files in the disk root".
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/?folder=", nil)
	newRouter(t, d).ServeHTTP(w, req)

	body := w.Body.String()
	for _, name := range []string{"Tron.mkv", "Dragons.mkv", "Episode01.mkv", "RootFile.mkv"} {
		assert.Contains(t, body, name)
	}
	assert.Equal(t, "all", readFolderPref(t, d))
}

func TestIndex_FolderFilter_AcceptsForwardSlashPaths(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films/Action", SizeBytes: 1_000_000_000},
		{Filename: "Nature.mkv", FolderRelativePath: "Docs/Nature", SizeBytes: 2_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?folder=Films", nil)
	newRouter(t, d).ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "Movie.mkv")
	assert.NotContains(t, body, "Nature.mkv")
}

func TestIndex_FolderFilter_CombinesWithStatus(t *testing.T) {
	d := openTestDB(t)
	seedMainFolders(t, d, scanner.VideoFile{
		Filename: "Episode02.mkv", FolderRelativePath: `Serials\Show\Season 1`, SizeBytes: 4_000_000_000,
	})
	q := db.New(d)
	for _, name := range []string{"Tron.mkv", "Episode01.mkv"} {
		require.NoError(t, q.UpdateMediaStatus(context.Background(), db.UpdateMediaStatusParams{
			CurrentStatus: "started",
			ID:            mediaIDByName(t, d, name),
		}))
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?folder=Serials&status=started", nil)
	newRouter(t, d).ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "Episode01.mkv")
	assert.NotContains(t, body, "Episode02.mkv", "a new media in the folder is hidden by the status filter")
	assert.NotContains(t, body, "Tron.mkv", "a started media outside the folder is hidden by the folder filter")
}

func TestIndex_SavesFolderFilterToDB(t *testing.T) {
	d := openTestDB(t)
	seedMainFolders(t, d)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?folder=Serials", nil)
	newRouter(t, d).ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	assert.Equal(t, "Serials", readFolderPref(t, d))
}

func TestIndex_RestoresSavedFolderFilter(t *testing.T) {
	d := openTestDB(t)
	seedMainFolders(t, d)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?folder=Serials", nil)
	newRouter(t, d).ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// A restart (new router, same database) lands on a bare "/" again.
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "Episode01.mkv")
	assert.NotContains(t, body, "Tron.mkv", "the remembered folder filter must still apply")
	assert.NotContains(t, body, "Dragons.mkv", "the remembered folder filter must still apply")
}

func TestRandomNew_RespectsFolderFilter(t *testing.T) {
	d := openTestDB(t)
	seedMainFolders(t, d)
	r := newRouter(t, d)

	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/media/random-new/start",
			strings.NewReader("status=all&disk=on&del=all&folder=Serials"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.ServeHTTP(w, req)
	}

	assert.Equal(t, 10, startEventCount(t, d, mediaIDByName(t, d, "Episode01.mkv")),
		"only the media under the chosen folder may be picked")
	assert.Equal(t, 0, startEventCount(t, d, mediaIDByName(t, d, "Tron.mkv")))
	assert.Equal(t, 0, startEventCount(t, d, mediaIDByName(t, d, "Dragons.mkv")))
}

// Both non-HTMX redirects (after a pick and when nothing matches) must lead back
// to the same folder, with the name query-escaped.
func TestRandomNew_RedirectCarriesEscapedFolder(t *testing.T) {
	const folder = "99 Гледани"
	want := "folder=" + url.QueryEscape(folder)

	post := func(t *testing.T, d *sql.DB) *httptest.ResponseRecorder {
		t.Helper()
		form := url.Values{
			"status": {"all"},
			"disk":   {"on"},
			"del":    {"all"},
			"folder": {folder},
		}
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/media/random-new/start", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		newRouter(t, d).ServeHTTP(w, req)
		return w
	}

	t.Run("after a pick", func(t *testing.T) {
		d := openTestDB(t)
		seedMedia(t, d, []scanner.VideoFile{
			{Filename: "Watched.mkv", FolderRelativePath: folder + `\Old Film`, SizeBytes: 1_000_000_000},
		})

		w := post(t, d)

		require.Equal(t, http.StatusSeeOther, w.Code)
		assert.Contains(t, w.Header().Get("Location"), want)
		assert.Equal(t, 1, startEventCount(t, d, mediaIDByName(t, d, "Watched.mkv")),
			"the media under the folder must be the one picked")
	})

	t.Run("when nothing matches", func(t *testing.T) {
		d := openTestDB(t)

		w := post(t, d)

		require.Equal(t, http.StatusSeeOther, w.Code)
		assert.Contains(t, w.Header().Get("Location"), want)
	})
}

// ---------------------------------------------------------------------------
// Bug 27: без завършени статуси + „За DnD"
// ---------------------------------------------------------------------------

// setStatus sets a media's status directly, without a status_changes row.
func setStatus(t *testing.T, d *sql.DB, filename, status string) {
	t.Helper()
	require.NoError(t, db.New(d).UpdateMediaStatus(context.Background(), db.UpdateMediaStatusParams{
		CurrentStatus: status,
		ID:            mediaIDByName(t, d, filename),
	}))
}

func statusChangeCount(t *testing.T, d *sql.DB, id int64) int {
	t.Helper()
	var n int
	require.NoError(t, d.QueryRow("SELECT COUNT(*) FROM status_changes WHERE media_id = ?", id).Scan(&n))
	return n
}

// A removed status (an old form, a stale browser tab or a hand-made request)
// must not get back into the database: the completed statuses (Bug 27) and
// „За DnD", which became the 👍 flag (Bug 30). The allowlist also replaced the
// old empty-status check, so an empty value is rejected too.
func TestChangeStatus_RejectsRemovedStatus(t *testing.T) {
	for _, status := range []string{"completed_both", "completed_yanko", "completed_liza", "for_dnd", "bogus", ""} {
		name := status
		if name == "" {
			name = "empty"
		}
		t.Run(name, func(t *testing.T) {
			d := openTestDB(t)
			id := seedOne(t, d)

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/media/%d/status", id), strings.NewReader("status="+status))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			newRouter(t, d).ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			updated, err := db.New(d).GetMediaByID(context.Background(), id)
			require.NoError(t, err)
			assert.Equal(t, "new", updated.CurrentStatus, "the status must not change")
			assert.Equal(t, 0, statusChangeCount(t, d, id), "no history row may be written")
		})
	}
}

// ---------------------------------------------------------------------------
// Bug 27: 🎲 изписва филма + без „Детайли"
// ---------------------------------------------------------------------------

// postRandomHTMX sends the 🎲 form the way HTMX does and returns the response.
func postRandomHTMX(t *testing.T, d *sql.DB) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/media/random-new/start", strings.NewReader("status=all&disk=on"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	newRouter(t, d).ServeHTTP(w, req)
	return w
}

// htmx parses a partial response by its first tag: only with the OOB row first
// (and useTemplateFragments, see ListPage) does the browser swap the row and
// keep the flash. With the flash first the <tr> tags are dropped and the row's
// text lands in #flash.
func TestRandomNew_HTMX_RowComesBeforeFlash(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "OrderMovie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})

	body := postRandomHTMX(t, d).Body.String()

	flashAt := strings.Index(body, "🎲 Стартирах: OrderMovie.mkv")
	require.NotEqual(t, -1, flashAt, "expected the flash in the HTMX response")
	assert.True(t, strings.HasPrefix(strings.TrimSpace(body), `<tr id="media-`),
		"the response must start with the OOB row, got %.80q", body)
	rowAt := strings.Index(body, `hx-swap-oob="true"`)
	require.NotEqual(t, -1, rowAt, "expected the OOB row in the HTMX response")
	assert.Less(t, rowAt, flashAt, "the OOB row must come before the flash")
}

func TestRandomNew_HTMX_FlashEscapesFilename(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Evil<b>Bold</b>.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})

	body := postRandomHTMX(t, d).Body.String()

	assert.Contains(t, body, "🎲 Стартирах: Evil&lt;b&gt;Bold&lt;/b&gt;.mkv", "the filename must be HTML-escaped in the flash")
	assert.NotContains(t, body, "<b>", "no markup from the filename may reach the page")
}

// The details view is removed: GET /media/:id answers 404.
func TestMediaDetail_RouteRemoved(t *testing.T) {
	d := openTestDB(t)
	id := seedOne(t, d)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/media/%d", id), nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// ---------------------------------------------------------------------------
// Bug 30: без превод
// ---------------------------------------------------------------------------

// The translation filter is removed: an old bookmark or a hand-made request
// with trans=… must not narrow the list, and a bare "/" afterwards (as after a
// restart) must not bring the old value back either.
func TestIndex_IgnoresTransParam(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Alpha.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
		{Filename: "Beta.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000},
	})

	for _, target := range []string{"/?trans=bg", "/"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, target, nil)
		newRouter(t, d).ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()
		assert.Contains(t, body, "Alpha.mkv", "GET %s must show every media", target)
		assert.Contains(t, body, "Beta.mkv", "GET %s must show every media", target)
	}
}

// trans=… must not narrow the 🎲 pool either, and the redirect back to the list
// (without HTMX) must not carry it on.
func TestRandomNew_IgnoresTransParam(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Pick.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})
	r := newRouter(t, d)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/media/random-new/start", strings.NewReader("status=all&disk=on&trans=bg"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	r.ServeHTTP(w, req)
	assert.Contains(t, w.Body.String(), "🎲 Стартирах: Pick.mkv", "trans=bg must not empty the pool")

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/media/random-new/start", strings.NewReader("status=all&disk=on&trans=bg"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusSeeOther, w.Code)
	assert.NotContains(t, w.Header().Get("Location"), "trans=", "the redirect must not carry the removed parameter")

	assert.Equal(t, 2, startEventCount(t, d, mediaIDByName(t, d, "Pick.mkv")), "both requests must start the media")
}

// ---------------------------------------------------------------------------
// Bug 30: три независими флага 👍 👎 ⚠
// ---------------------------------------------------------------------------

// raiseFlag raises one of the three independent flags (for_dnd, for_deletion,
// wrong_subs) directly in the DB, like markForDeletion.
func raiseFlag(t *testing.T, d *sql.DB, column, filename string) {
	t.Helper()
	_, err := d.Exec("UPDATE media SET "+column+" = 1 WHERE filename = ?", filename)
	require.NoError(t, err)
}

// flagValue returns one of the three flags of a media straight from the DB.
func flagValue(t *testing.T, d *sql.DB, column string, id int64) int64 {
	t.Helper()
	var v int64
	require.NoError(t, d.QueryRow("SELECT "+column+" FROM media WHERE id = ?", id).Scan(&v))
	return v
}

// readFlagPrefs returns the remembered 👍 and ⚠ filters. Like readFolderPref it
// is kept apart from readPrefs, so the Bug 23 tests keep their six-value shape.
func readFlagPrefs(t *testing.T, d *sql.DB) (dnd, subs string) {
	t.Helper()
	require.NoError(t, d.QueryRow("SELECT dnd_filter, subs_filter FROM ui_prefs WHERE id = 1").Scan(&dnd, &subs))
	return
}

// dnd=yes and subs=yes narrow the list to the media with the 👍 / ⚠ flag
// raised, like del=yes does for 👎; "all" shows every media again.
func TestIndex_FlagFilters(t *testing.T) {
	for _, tc := range []struct{ param, column string }{
		{"dnd", "for_dnd"},
		{"subs", "wrong_subs"},
	} {
		t.Run(tc.param, func(t *testing.T) {
			d := openTestDB(t)
			seedMedia(t, d, []scanner.VideoFile{
				{Filename: "Flagged.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
				{Filename: "Plain.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000},
			})
			raiseFlag(t, d, tc.column, "Flagged.mkv")

			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/?"+tc.param+"=yes", nil)
			newRouter(t, d).ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)
			body := w.Body.String()
			assert.Contains(t, body, "Flagged.mkv")
			assert.NotContains(t, body, "Plain.mkv", "%s=yes must hide the media without the flag", tc.param)

			w = httptest.NewRecorder()
			req = httptest.NewRequest(http.MethodGet, "/?"+tc.param+"=all", nil)
			newRouter(t, d).ServeHTTP(w, req)

			body = w.Body.String()
			assert.Contains(t, body, "Flagged.mkv")
			assert.Contains(t, body, "Plain.mkv", "%s=all must show every media", tc.param)
		})
	}
}

// The flags are independent: a media can carry 👍 and 👎 at once and shows up
// under either filter; flag filters combine with each other and with the status.
func TestIndex_FlagsAreIndependent(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Both.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
		{Filename: "OnlyDnd.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000},
		{Filename: "OnlyDel.mkv", FolderRelativePath: "Films", SizeBytes: 3_000_000_000},
	})
	raiseFlag(t, d, "for_dnd", "Both.mkv")
	raiseFlag(t, d, "for_deletion", "Both.mkv")
	raiseFlag(t, d, "for_dnd", "OnlyDnd.mkv")
	raiseFlag(t, d, "for_deletion", "OnlyDel.mkv")
	setStatus(t, d, "OnlyDnd.mkv", "started")

	// Every request names status, dnd and del: a missing one would fall back to
	// the value the previous request remembered.
	for _, tc := range []struct {
		target string
		want   []string
	}{
		{"/?status=all&dnd=yes&del=all", []string{"Both.mkv", "OnlyDnd.mkv"}},
		{"/?status=all&dnd=all&del=yes", []string{"Both.mkv", "OnlyDel.mkv"}},
		{"/?status=all&dnd=yes&del=yes", []string{"Both.mkv"}},
		{"/?status=started&dnd=yes&del=all", []string{"OnlyDnd.mkv"}},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, tc.target, nil)
		newRouter(t, d).ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		body := w.Body.String()
		for _, name := range []string{"Both.mkv", "OnlyDnd.mkv", "OnlyDel.mkv"} {
			if slices.Contains(tc.want, name) {
				assert.Contains(t, body, name, "GET %s", tc.target)
			} else {
				assert.NotContains(t, body, name, "GET %s", tc.target)
			}
		}
	}
}

// The three flags share one handler: value=1 raises the flag, value=0 clears
// it; with HTMX only the updated row comes back, with the button highlighted;
// a bad id is a 404.
func TestSetFlag_TogglesAndReturnsRow(t *testing.T) {
	for _, tc := range []struct{ route, column, title string }{
		{"for-dnd", "for_dnd", "За DnD"},
		{"for-deletion", "for_deletion", "За изтриване"},
		{"wrong-subs", "wrong_subs", "Грешни субтитри"},
	} {
		t.Run(tc.route, func(t *testing.T) {
			d := openTestDB(t)
			id := seedOne(t, d)
			r := newRouter(t, d)
			post := func(target, value string, htmx bool) *httptest.ResponseRecorder {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, target, strings.NewReader("value="+value))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				if htmx {
					req.Header.Set("HX-Request", "true")
				}
				r.ServeHTTP(w, req)
				return w
			}
			target := fmt.Sprintf("/media/%d/%s", id, tc.route)

			w := post(target, "1", false)
			assert.Equal(t, http.StatusSeeOther, w.Code)
			assert.Equal(t, int64(1), flagValue(t, d, tc.column, id), "value=1 must raise the flag")

			w = post(target, "0", false)
			assert.Equal(t, http.StatusSeeOther, w.Code)
			assert.Equal(t, int64(0), flagValue(t, d, tc.column, id), "value=0 must clear the flag")

			w = post(target, "1", true)
			assert.Equal(t, http.StatusOK, w.Code)
			body := w.Body.String()
			assert.True(t, strings.HasPrefix(strings.TrimSpace(body), fmt.Sprintf(`<tr id="media-%d"`, id)),
				"the HTMX response must be the media's row, got %.80q", body)
			assert.NotContains(t, body, "<html", "HTMX response must be the row only")
			assert.Equal(t, int64(1), flagValue(t, d, tc.column, id))
			// The HTML parser drops a bare <tr> outside a table, hence the wrapper.
			class := parseBody(t, "<table>"+body+"</table>").
				Find(fmt.Sprintf("td.actions button[title='%s']", tc.title)).AttrOr("class", "")
			assert.Contains(t, class, "filter-active", "the returned row must show the raised flag")

			w = post("/media/abc/"+tc.route, "1", false)
			assert.Equal(t, http.StatusNotFound, w.Code, "a bad id must be a 404")
		})
	}
}

// Starting a media only moves new → started; the flags stay as they are.
func TestStartMedia_KeepsFlags(t *testing.T) {
	d := openTestDB(t)
	id := seedOne(t, d)
	raiseFlag(t, d, "for_dnd", "Movie.mkv")
	raiseFlag(t, d, "wrong_subs", "Movie.mkv")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/media/%d/start", id), nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	updated, err := db.New(d).GetMediaByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "started", updated.CurrentStatus)
	assert.Equal(t, 1, startEventCount(t, d, id), "a start event must be recorded")
	assert.Equal(t, int64(1), flagValue(t, d, "for_dnd", id), "👍 must stay raised")
	assert.Equal(t, int64(0), flagValue(t, d, "for_deletion", id), "👎 must stay lowered")
	assert.Equal(t, int64(1), flagValue(t, d, "wrong_subs", id), "⚠ must stay raised")
}

// 🎲 follows the same rule as "Стартирай": new → started, the flags untouched.
func TestRandomNew_KeepsFlags(t *testing.T) {
	d := openTestDB(t)
	id := seedOne(t, d)
	raiseFlag(t, d, "for_dnd", "Movie.mkv")
	raiseFlag(t, d, "wrong_subs", "Movie.mkv")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/media/random-new/start", strings.NewReader("status=all&disk=on"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	newRouter(t, d).ServeHTTP(w, req)

	updated, err := db.New(d).GetMediaByID(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, "started", updated.CurrentStatus)
	assert.Equal(t, 1, startEventCount(t, d, id), "a start event must be recorded")
	assert.Equal(t, int64(1), flagValue(t, d, "for_dnd", id), "👍 must stay raised")
	assert.Equal(t, int64(0), flagValue(t, d, "for_deletion", id), "👎 must stay lowered")
	assert.Equal(t, int64(1), flagValue(t, d, "wrong_subs", id), "⚠ must stay raised")
}

// 🎲 picks from the list the 👍 / ⚠ filters show, and without HTMX it leads
// back to that same list.
func TestRandomNew_RespectsFlagFilters(t *testing.T) {
	for _, tc := range []struct{ param, column string }{
		{"dnd", "for_dnd"},
		{"subs", "wrong_subs"},
	} {
		t.Run(tc.param, func(t *testing.T) {
			d := openTestDB(t)
			seedMedia(t, d, []scanner.VideoFile{
				{Filename: "Flagged.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
				{Filename: "Other1.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000},
				{Filename: "Other2.mkv", FolderRelativePath: "Films", SizeBytes: 3_000_000_000},
			})
			raiseFlag(t, d, tc.column, "Flagged.mkv")
			r := newRouter(t, d)

			var w *httptest.ResponseRecorder
			for i := 0; i < 10; i++ {
				w = httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/media/random-new/start",
					strings.NewReader("status=all&disk=on&"+tc.param+"=yes"))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				r.ServeHTTP(w, req)
			}

			assert.Equal(t, 10, startEventCount(t, d, mediaIDByName(t, d, "Flagged.mkv")),
				"only the flagged media may be picked")
			assert.Equal(t, 0, startEventCount(t, d, mediaIDByName(t, d, "Other1.mkv")))
			assert.Equal(t, 0, startEventCount(t, d, mediaIDByName(t, d, "Other2.mkv")))

			require.Equal(t, http.StatusSeeOther, w.Code)
			assert.Contains(t, w.Header().Get("Location"), tc.param+"=yes",
				"the redirect must lead back to the same flag filter")
		})
	}
}
