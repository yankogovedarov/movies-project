package web_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yankogovedarov/movie-tracker/internal/db"
	"github.com/yankogovedarov/movie-tracker/internal/scanner"
)

func parseBody(t *testing.T, body string) *goquery.Document {
	t.Helper()
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	require.NoError(t, err)
	return doc
}

func TestIndex_TableRowCount(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Alpha.mkv", FolderRelativePath: "Action", SizeBytes: 1_000_000_000},
		{Filename: "Beta.mkv", FolderRelativePath: "Drama", SizeBytes: 2_000_000_000},
		{Filename: "Gamma.mkv", FolderRelativePath: "Comedy", SizeBytes: 500_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	rows := doc.Find("tbody tr")
	assert.Equal(t, 3, rows.Length(), "expected 3 table rows")
}

func TestIndex_FilenameCell_IsLinkButton(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Inception.mkv", FolderRelativePath: "SciFi", SizeBytes: 1_500_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	link := doc.Find("td.filename a.filename-link")
	require.Equal(t, 1, link.Length(), "expected filename-link anchor")
	assert.Equal(t, "Inception.mkv", strings.TrimSpace(link.Text()))
}

func TestIndex_FolderCell_ShowsPath(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films/Action", SizeBytes: 1_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	folder := doc.Find("td.folder")
	require.Equal(t, 1, folder.Length(), "expected folder cell")
	assert.Equal(t, "Films/Action", strings.TrimSpace(folder.Text()))
}

func TestIndex_StartFormAction(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	href, exists := doc.Find("td.filename a.filename-link").Attr("href")
	require.True(t, exists, "expected filename-link anchor in filename cell")
	assert.Contains(t, href, "/media/", "filename link should point to detail page")
}

func TestIndex_StatusFormAction(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	action, exists := doc.Find("td.actions form").Attr("action")
	require.True(t, exists, "expected form in actions cell")
	assert.Contains(t, action, "/status", "status form action should contain /status")
}

func TestDetailPage_ShowsMediaInfo(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Arrival.mkv", FolderRelativePath: "SciFi", SizeBytes: 1_800_000_000},
	})
	q := db.New(d)
	media, err := q.ListOnDiskMedia(context.Background())
	require.NoError(t, err)
	id := media[0].ID

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/media/%d", id), nil)
	newRouter(t, d).ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	doc := parseBody(t, w.Body.String())
	assert.Contains(t, doc.Find("h1").Text(), "Arrival.mkv")
	assert.Equal(t, 1, doc.Find("select[name=status]").Length(), "expected status dropdown")
}

func TestDetailPage_ShowsHistoryTables(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Interstellar.mkv", FolderRelativePath: "SciFi", SizeBytes: 2_000_000_000},
	})
	q := db.New(d)
	media, err := q.ListOnDiskMedia(context.Background())
	require.NoError(t, err)
	id := media[0].ID

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/media/%d", id), nil)
	newRouter(t, d).ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "История на стартиранията")
	assert.Contains(t, body, "История на статусите")
}

func TestIndex_StatusSelect_HasFiveOptions(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	options := doc.Find("select[name=status] option")
	assert.Equal(t, 5, options.Length(), "expected 5 status options")

	var values []string
	options.Each(func(_ int, s *goquery.Selection) {
		v, _ := s.Attr("value")
		values = append(values, v)
	})
	assert.Contains(t, values, "new")
	assert.Contains(t, values, "started")
	assert.Contains(t, values, "completed_both")
	assert.Contains(t, values, "completed_yanko")
	assert.Contains(t, values, fmt.Sprintf("%s", "completed_liza"))
}
