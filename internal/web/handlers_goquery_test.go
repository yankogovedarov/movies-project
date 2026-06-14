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
	btn := doc.Find("td.filename form button.filename-link")
	require.Equal(t, 1, btn.Length(), "expected filename-link button")
	assert.Equal(t, "Inception.mkv", strings.TrimSpace(btn.Text()))
}

func TestIndex_FilenameForm_HasHxPost(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Inception.mkv", FolderRelativePath: "SciFi", SizeBytes: 1_500_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	form := doc.Find("td.filename form")
	hxPost, exists := form.Attr("hx-post")
	require.True(t, exists, "expected hx-post on filename form")
	assert.Contains(t, hxPost, "/start")
	hxTarget, _ := form.Attr("hx-target")
	assert.Equal(t, "closest tr", hxTarget)
}

func TestIndex_ActionForms_HaveHxPost(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	forms := doc.Find("td.actions form")
	require.Equal(t, 5, forms.Length(), "expected 5 action forms")
	forms.Each(func(_ int, s *goquery.Selection) {
		hxPost, exists := s.Attr("hx-post")
		assert.True(t, exists, "expected hx-post on action form")
		assert.Contains(t, hxPost, "/status")
		hxTarget, _ := s.Attr("hx-target")
		assert.Equal(t, "closest tr", hxTarget)
	})
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
	action, exists := doc.Find("td.filename form").Attr("action")
	require.True(t, exists, "expected form in filename cell")
	assert.Contains(t, action, "/start", "start form action should contain /start")
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
	action, exists := doc.Find("td.actions form").First().Attr("action")
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

func TestIndex_Actions_HasFiveStatusButtons(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	statusForms := doc.Find("td.actions form")
	assert.Equal(t, 5, statusForms.Length(), "expected 5 status forms in actions column")

	var statusValues []string
	statusForms.Each(func(_ int, s *goquery.Selection) {
		v, _ := s.Find("input[name=status]").Attr("value")
		statusValues = append(statusValues, v)
	})
	assert.Contains(t, statusValues, "new")
	assert.Contains(t, statusValues, "started")
	assert.Contains(t, statusValues, "completed_both")
	assert.Contains(t, statusValues, "completed_yanko")
	assert.Contains(t, statusValues, "completed_liza")
}

func TestIndex_FilterButtons_Present(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	btns := doc.Find("a.filter-btn")
	assert.Equal(t, 11, btns.Length(), "expected 11 filter buttons (6 status + 1 disk toggle + 4 translation)")
}

func TestIndex_SortButtons_Present(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	btns := doc.Find("a.sort-btn")
	assert.Equal(t, 6, btns.Length(), "expected 6 sort buttons (name, path, size, last_started, added, marked)")
}

func TestIndex_SortButton_NameIsActiveByDefault(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	active := doc.Find("a.sort-btn.filter-active")
	require.Equal(t, 1, active.Length(), "expected exactly 1 active sort button by default")
	href, _ := active.First().Attr("href")
	assert.Contains(t, href, "sort=name", "default active sort button href should contain sort=name")
}

func TestIndex_SortButton_LastStartedIsActive_WhenParam(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?sort=last_started", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	active := doc.Find("a.sort-btn.filter-active")
	require.Equal(t, 1, active.Length(), "expected exactly 1 active sort button")
	href, _ := active.First().Attr("href")
	assert.Contains(t, href, "sort=last_started", "active sort button href should contain sort=last_started")
}

func TestIndex_Row_ShowsStartCount_WhenStarted(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Watched.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})
	q := db.New(d)
	media, err := q.ListOnDiskMedia(context.Background())
	require.NoError(t, err)
	id := media[0].ID
	require.NoError(t, q.InsertStartEvent(context.Background(), id))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	startInfo := doc.Find(".start-info")
	require.Equal(t, 1, startInfo.Length(), "expected .start-info element for started media")
	assert.Contains(t, startInfo.Text(), "1")
}

func TestIndex_Row_NoStartInfo_WhenNeverStarted(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Fresh.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	startInfo := doc.Find(".start-info")
	assert.Equal(t, 0, startInfo.Length(), "expected no .start-info for never-started media")
}

func TestIndex_RandomForm_HasHxPost(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())

	var randomForm *goquery.Selection
	doc.Find("form").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		action, _ := s.Attr("action")
		hxPost, _ := s.Attr("hx-post")
		if strings.Contains(action, "random-new/start") || strings.Contains(hxPost, "random-new/start") {
			randomForm = s
			return false
		}
		return true
	})
	require.NotNil(t, randomForm, "expected 🎲 random form")

	hxPost, exists := randomForm.Attr("hx-post")
	require.True(t, exists, "expected hx-post on random form")
	assert.Contains(t, hxPost, "random-new/start")

	for _, name := range []string{"status", "disk", "q", "trans"} {
		assert.Equal(t, 1, randomForm.Find(fmt.Sprintf("input[name=%s]", name)).Length(),
			"expected hidden input %q in random form", name)
	}
}

func TestIndex_StickyNavAndTranslationHighlight_CSS(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	body := w.Body.String()
	// Bug 16 part 5: filter bar stays visible while scrolling.
	assert.Contains(t, body, "position: sticky", "expected sticky .top-nav CSS")
	// Bug 16 part 3: active translation type is visibly highlighted.
	assert.Contains(t, body, "td.translation .icon-btn.filter-active",
		"expected highlight CSS for active translation button")
}

func TestIndex_SearchInput_IsLive(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())

	// Results region is wrapped so HTMX can hx-select/hx-swap it independently.
	require.Equal(t, 1, doc.Find("#media-list").Length(), "expected #media-list wrapper")

	input := doc.Find("input[type=search][name=q]")
	require.Equal(t, 1, input.Length(), "expected search input")

	hxGet, ok := input.Attr("hx-get")
	require.True(t, ok, "search input must have hx-get for live search")
	assert.Equal(t, "/", hxGet)

	hxTrigger, ok := input.Attr("hx-trigger")
	require.True(t, ok, "search input must have hx-trigger for live search")
	assert.Contains(t, hxTrigger, "keyup", "search must trigger on keyup (no Enter needed)")

	hxTarget, _ := input.Attr("hx-target")
	assert.Equal(t, "#media-list", hxTarget)
	hxSelect, _ := input.Attr("hx-select")
	assert.Equal(t, "#media-list", hxSelect)
}

func TestIndex_FilterButtons_ReflectsActiveParam(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})
	q := db.New(d)
	media, err := q.ListOnDiskMedia(context.Background())
	require.NoError(t, err)
	require.NoError(t, q.UpdateMediaStatus(context.Background(), db.UpdateMediaStatusParams{
		CurrentStatus: "started",
		ID:            media[0].ID,
	}))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?status=started", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	active := doc.Find("a.filter-btn.filter-active")
	require.GreaterOrEqual(t, active.Length(), 1, "expected at least one active filter button")
	href, _ := active.First().Attr("href")
	assert.Contains(t, href, "status=started", "active button href should contain status=started")
}
