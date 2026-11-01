package web_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	require.Equal(t, 4, forms.Length(), "expected 4 action forms (3 status + 1 за изтриване)")
	var statusPosts, delPosts int
	forms.Each(func(_ int, s *goquery.Selection) {
		hxPost, exists := s.Attr("hx-post")
		assert.True(t, exists, "expected hx-post on action form")
		switch {
		case strings.Contains(hxPost, "/status"):
			statusPosts++
		case strings.Contains(hxPost, "/for-deletion"):
			delPosts++
		default:
			t.Errorf("unexpected hx-post %q in actions column", hxPost)
		}
		hxTarget, _ := s.Attr("hx-target")
		assert.Equal(t, "closest tr", hxTarget)
	})
	assert.Equal(t, 3, statusPosts, "expected 3 status forms")
	assert.Equal(t, 1, delPosts, "expected 1 за-изтриване form")
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

// Bug 27: the row offers ○ ▶ D — the completed statuses (✓ Я Л) are removed.
func TestIndex_Actions_HasThreeStatusButtons(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	statusForms := doc.Find("td.actions form").FilterFunction(func(_ int, s *goquery.Selection) bool {
		return s.Find("input[name=status]").Length() == 1
	})
	assert.Equal(t, 3, statusForms.Length(), "expected 3 status forms in actions column")

	var statusValues []string
	statusForms.Each(func(_ int, s *goquery.Selection) {
		v, _ := s.Find("input[name=status]").Attr("value")
		statusValues = append(statusValues, v)
	})
	assert.Equal(t, []string{"new", "started", "for_dnd"}, statusValues)

	dnd := doc.Find("td.actions button.icon-btn[title='За DnD']")
	require.Equal(t, 1, dnd.Length(), "expected the За DnD row button")
	assert.Equal(t, "D", strings.TrimSpace(dnd.Text()))
}

func TestIndex_FilterButtons_Present(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	btns := doc.Find("a.filter-btn")
	assert.Equal(t, 11, btns.Length(), "expected 11 filter buttons (4 status + 1 disk toggle + 1 за изтриване + 5 translation)")
}

func TestIndex_TransFilter_HasLabelAndNoneButton(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "превод:", "expected a 'превод:' label separating translation filters")

	doc := parseBody(t, body)
	// The translation group is its own container.
	assert.Equal(t, 1, doc.Find(".trans-btns").Length(), "expected a single .trans-btns group")
	// The "неустановено" (unset) filter button points to trans=none.
	none := doc.Find(".trans-btns a.filter-btn[title='Неустановен превод']")
	require.Equal(t, 1, none.Length(), "expected a single 'неустановено' button")
	href, _ := none.Attr("href")
	assert.Contains(t, href, "trans=none")
}

func TestIndex_TransNoneButton_ActiveWhenParam(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?trans=none", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	none := doc.Find(".trans-btns a.filter-btn[title='Неустановен превод']")
	require.Equal(t, 1, none.Length())
	class, _ := none.Attr("class")
	assert.Contains(t, class, "filter-active", "trans=none button should be active when trans=none")
}

func TestIndex_SortButtons_Present(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	btns := doc.Find("a.sort-btn")
	assert.Equal(t, 5, btns.Length(), "expected 5 sort buttons (name, path, size, last_started, added)")
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

func TestIndex_DateSortButtons_FirstClickIsDescending(t *testing.T) {
	// Bug #19: hrefs of the date sort buttons must start with dir=desc when the
	// button is inactive (most recent first on first click). Bug 27 removed the
	// "marked" sort, so "added" is the only date sort left.
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	for _, title := range []string{"По дата добавяне"} {
		btn := doc.Find(fmt.Sprintf("a.sort-btn[title='%s']", title))
		require.Equal(t, 1, btn.Length(), "expected sort button %q", title)
		href, _ := btn.Attr("href")
		assert.Contains(t, href, "dir=desc", "inactive %q button should link to dir=desc", title)
	}
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

	for _, name := range []string{"status", "disk", "q", "trans", "del", "folder"} {
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

// Bug 18 part 1: the search control must be plain — same height as the rest of the
// nav bar and without icons (no 🔍 emoji in the placeholder, no native clear button).
func TestIndex_SearchInput_IsPlain(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())

	// Functionality preserved.
	input := doc.Find("input[type=search][name=q]")
	require.Equal(t, 1, input.Length(), "expected search input")

	class, _ := input.Attr("class")
	assert.Contains(t, class, "search-input", "search input must keep the search-input class")

	// No icon in the placeholder.
	placeholder, _ := input.Attr("placeholder")
	assert.NotContains(t, placeholder, "🔍", "placeholder must not contain the magnifier emoji")

	// Native search decorations hidden so no clear (✕) icon is shown.
	body := w.Body.String()
	assert.Contains(t, body, "-webkit-search-cancel-button",
		"expected CSS hiding the native search clear button")
}

// Bug 18 part 2: the filter-group label must be spelled correctly — "филтър:"
// (all Cyrillic), not the typo "філтър:" which used the Ukrainian і (U+0456).
func TestIndex_FilterLabel_Spelling(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	body := w.Body.String()
	assert.Contains(t, body, "филтър:", "expected correctly spelled filter label")
	assert.NotContains(t, body, "філтър:", "filter label must not use the Ukrainian і (U+0456)")
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

func TestIndex_ForDeletionFilter_InFilterBtns(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	// Bug 22: the flag filter lives inside the existing "филтър:" group.
	btn := doc.Find(".filter-btns a.filter-btn[title='За изтриване']")
	require.Equal(t, 1, btn.Length(), "expected a single 'за изтриване' filter button in .filter-btns")

	href, _ := btn.Attr("href")
	assert.Contains(t, href, "del=yes", "inactive button must narrow to the marked media")
	class, _ := btn.Attr("class")
	assert.NotContains(t, class, "filter-active", "button must be inactive by default")
}

func TestIndex_ForDeletionFilter_ActiveWhenParam(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?del=yes", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	btn := doc.Find(".filter-btns a.filter-btn[title='За изтриване']")
	require.Equal(t, 1, btn.Length())

	class, _ := btn.Attr("class")
	assert.Contains(t, class, "filter-active", "button should be active when del=yes")
	href, _ := btn.Attr("href")
	assert.Contains(t, href, "del=all", "active button must link back to the unfiltered list")
}

func TestIndex_ForDeletionFilter_PreservedByOtherFilters(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?del=yes", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	// Every status/translation link and every sort link must carry del=yes,
	// otherwise clicking one of them silently drops the flag filter.
	doc.Find(".filter-btns a.filter-btn, .trans-btns a.filter-btn, .sort-btns a.sort-btn").
		Each(func(_ int, sel *goquery.Selection) {
			title, _ := sel.Attr("title")
			if title == "За изтриване" {
				return // this one intentionally toggles back to del=all
			}
			href, _ := sel.Attr("href")
			assert.Contains(t, href, "del=yes", "link %q must preserve the за-изтриване filter", title)
		})

	// The search and 🎲 forms re-send the filter (hx-include / hidden inputs).
	carrying := doc.Find("form input[type=hidden][name=del]").FilterFunction(
		func(_ int, sel *goquery.Selection) bool {
			v, _ := sel.Attr("value")
			return v == "yes"
		})
	assert.Equal(t, 2, carrying.Length(), "expected the search and random forms to carry del=yes")
}

func TestIndex_RowHasForDeletionButton(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Keep.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
		{Filename: "Trash.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000},
	})
	markForDeletion(t, d, "Trash.mkv")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?sort=name", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	btns := doc.Find("td.actions button.icon-btn[title='За изтриване']")
	require.Equal(t, 2, btns.Length(), "expected one 🗑 button per row")

	byFile := map[string]*goquery.Selection{}
	doc.Find("tbody tr").Each(func(_ int, row *goquery.Selection) {
		name := strings.TrimSpace(row.Find("td.filename button.filename-link").Text())
		byFile[name] = row
	})

	marked := byFile["Trash.mkv"].Find("td.actions button.icon-btn[title='За изтриване']")
	markedClass, _ := marked.Attr("class")
	assert.Contains(t, markedClass, "filter-active", "marked media must show a highlighted 🗑")
	markedValue, _ := byFile["Trash.mkv"].Find("td.actions input[name=value]").Attr("value")
	assert.Equal(t, "0", markedValue, "clicking a marked media must clear the flag")

	plain := byFile["Keep.mkv"].Find("td.actions button.icon-btn[title='За изтриване']")
	plainClass, _ := plain.Attr("class")
	assert.NotContains(t, plainClass, "filter-active")
	plainValue, _ := byFile["Keep.mkv"].Find("td.actions input[name=value]").Attr("value")
	assert.Equal(t, "1", plainValue, "clicking an unmarked media must raise the flag")
}

// Bug 23: after a restart the restored state must also be visible in the nav
// bar — the right buttons are highlighted and the search box is pre-filled.
func TestIndex_RestoredFilters_MarkActiveButtons(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Godzilla.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?status=new&sort=size&dir=desc&q=god&trans=bg&del=yes", nil)
	newRouter(t, d).ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())

	statusClass, _ := doc.Find("a.filter-btn[title='Нова']").Attr("class")
	assert.Contains(t, statusClass, "filter-active")
	transClass, _ := doc.Find("a.filter-btn[title='Български говор']").Attr("class")
	assert.Contains(t, transClass, "filter-active")
	delClass, _ := doc.Find("a.filter-btn[title='За изтриване']").Attr("class")
	assert.Contains(t, delClass, "filter-active")

	sortBtn := doc.Find("a.sort-btn[title='По размер']")
	sortBtnClass, _ := sortBtn.Attr("class")
	assert.Contains(t, sortBtnClass, "filter-active")
	assert.Equal(t, "Рз↓", strings.TrimSpace(sortBtn.Text()), "descending size sort must be restored")

	searchValue, _ := doc.Find("input.search-input").Attr("value")
	assert.Equal(t, "god", searchValue)
}

// ---------------------------------------------------------------------------
// Bug 26: падащо меню за основна папка
// ---------------------------------------------------------------------------

// folderOptionValues returns the option values of the folder dropdown, in page
// order.
func folderOptionValues(doc *goquery.Document) []string {
	var values []string
	doc.Find("select[name=folder] option").Each(func(_ int, opt *goquery.Selection) {
		v, _ := opt.Attr("value")
		values = append(values, v)
	})
	return values
}

func TestIndex_FolderSelect_FirstInFilterBtns(t *testing.T) {
	d := openTestDB(t)
	seedMainFolders(t, d)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	// The dropdown opens the "филтър:" group, before the ◎ button.
	first := doc.Find(".filter-btns").Children().First()
	require.Equal(t, "select", goquery.NodeName(first), "the folder dropdown must be first in .filter-btns")
	name, _ := first.Attr("name")
	assert.Equal(t, "folder", name)

	// It belongs to the search form: a change re-sends the current search text
	// and filters, and the live search carries the folder along.
	formID, ok := first.Attr("form")
	require.True(t, ok, "the dropdown must be associated with a form")
	searchForm := doc.Find("input[type=search][name=q]").Closest("form")
	require.Equal(t, 1, searchForm.Length(), "expected the search form")
	searchID, _ := searchForm.Attr("id")
	require.NotEmpty(t, searchID, "the search form needs an id")
	assert.Equal(t, searchID, formID)

	onchange, _ := first.Attr("onchange")
	assert.Contains(t, onchange, "submit()", "picking a folder must submit the form")

	opt := first.Find("option").First()
	value, _ := opt.Attr("value")
	assert.Equal(t, "all", value)
	assert.Equal(t, "всички папки", strings.TrimSpace(opt.Text()))
}

func TestIndex_FolderSelect_OptionsFromTopFolders(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Ep.mkv", FolderRelativePath: `Serials\X`, SizeBytes: 1_000_000_000},
		{Filename: "Direct.mkv", FolderRelativePath: "Movies", SizeBytes: 2_000_000_000},
		{Filename: "Nested.mkv", FolderRelativePath: `Movies\Sub`, SizeBytes: 3_000_000_000},
		{Filename: "Dnd.mkv", FolderRelativePath: `MoviesDnD\D`, SizeBytes: 4_000_000_000},
		{Filename: "Root.mkv", FolderRelativePath: "", SizeBytes: 5_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	// Distinct, sorted, and no entry for the disk root (a root file has no
	// main folder).
	assert.Equal(t, []string{"all", "Movies", "MoviesDnD", "Serials"},
		folderOptionValues(parseBody(t, w.Body.String())))
}

func TestIndex_FolderSelect_OptionsFollowDiskFilter(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Here.mkv", FolderRelativePath: `Movies\Here`, SizeBytes: 1_000_000_000},
		{Filename: "Gone.mkv", FolderRelativePath: `Archive\Gone`, SizeBytes: 2_000_000_000},
	})
	_, err := d.Exec("UPDATE media SET on_disk = 0 WHERE filename = 'Gone.mkv'")
	require.NoError(t, err)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)
	assert.Equal(t, []string{"all", "Movies"}, folderOptionValues(parseBody(t, w.Body.String())),
		"a folder with only vanished media is not offered in the on-disk view")

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/?disk=all", nil)
	newRouter(t, d).ServeHTTP(w, req)
	assert.Equal(t, []string{"all", "Archive", "Movies"}, folderOptionValues(parseBody(t, w.Body.String())),
		"the ↕ all-media view also offers the folders of vanished media")
}

func TestIndex_FolderSelect_OptionsIgnoreOtherFilters(t *testing.T) {
	d := openTestDB(t)
	seedMainFolders(t, d)

	// Neither the chosen folder nor a status that matches nothing may shrink
	// the dropdown — the user must be able to switch straight to another folder.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?folder=Serials&status=for_dnd", nil)
	newRouter(t, d).ServeHTTP(w, req)

	assert.Equal(t, []string{"all", "Movies", "MoviesDnD", "Serials"},
		folderOptionValues(parseBody(t, w.Body.String())))
}

func TestIndex_FolderSelect_MarksSelectedAndActive(t *testing.T) {
	d := openTestDB(t)
	seedMainFolders(t, d)

	// Fresh state first (a later request would be remembered): "всички папки"
	// is selected and the dropdown is not highlighted.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	sel := parseBody(t, w.Body.String()).Find("select[name=folder]")
	require.Equal(t, 1, sel.Length(), "expected the folder dropdown")
	_, allSelected := sel.Find("option[value=all]").Attr("selected")
	assert.True(t, allSelected, "всички папки must be selected by default")
	class, _ := sel.Attr("class")
	assert.NotContains(t, class, "filter-active", "no folder filter, no highlight")

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/?folder=Serials", nil)
	newRouter(t, d).ServeHTTP(w, req)

	sel = parseBody(t, w.Body.String()).Find("select[name=folder]")
	_, serialsSelected := sel.Find("option[value=Serials]").Attr("selected")
	assert.True(t, serialsSelected, "the chosen folder must be selected")
	assert.Equal(t, 1, sel.Find("option[selected]").Length(), "exactly one option is selected")
	class, _ = sel.Attr("class")
	assert.Contains(t, class, "filter-active", "an active folder filter is highlighted")
}

func TestIndex_FolderSelect_KeepsUnknownFolderSelected(t *testing.T) {
	d := openTestDB(t)
	seedMainFolders(t, d)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?folder=Old", nil)
	newRouter(t, d).ServeHTTP(w, req)

	body := w.Body.String()
	// A remembered folder that no longer exists stays visibly selected, so the
	// dropdown explains the empty list.
	old := parseBody(t, body).Find("select[name=folder] option[value=Old]")
	require.Equal(t, 1, old.Length(), "the unknown folder must still be offered")
	_, selected := old.Attr("selected")
	assert.True(t, selected, "the unknown folder must stay selected")
	assert.Contains(t, body, "Няма намерени медии.")
}

func TestIndex_FolderFilter_PreservedByOtherFilters(t *testing.T) {
	const folder = "99 Гледани"
	want := "folder=" + url.QueryEscape(folder)

	// The ↕ link has two variants (on disk / all media); each request renders
	// one of them, so both views are checked.
	for _, tc := range []struct{ name, target string }{
		{"on disk", "/?folder=" + url.QueryEscape(folder)},
		{"all media", "/?disk=all&folder=" + url.QueryEscape(folder)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := openTestDB(t)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.target, nil)
			newRouter(t, d).ServeHTTP(w, req)

			doc := parseBody(t, w.Body.String())
			// Every status/↕/🗑 link, translation link and sort link carries the
			// folder (escaped), otherwise clicking one of them silently drops the
			// filter.
			links := doc.Find(".filter-btns a.filter-btn, .trans-btns a.filter-btn, .sort-btns a.sort-btn")
			require.Equal(t, 16, links.Length(), "expected 6 filter + 5 translation + 5 sort links")
			links.Each(func(_ int, sel *goquery.Selection) {
				title, _ := sel.Attr("title")
				href, _ := sel.Attr("href")
				assert.Contains(t, href, want, "link %q must preserve the folder filter", title)
			})

			// The 🎲 form re-sends the folder as a hidden field.
			random := doc.Find("form[action='/media/random-new/start']")
			require.Equal(t, 1, random.Length(), "expected the 🎲 form")
			value, ok := random.Find("input[type=hidden][name=folder]").Attr("value")
			require.True(t, ok, "expected a hidden folder input in the 🎲 form")
			assert.Equal(t, folder, value)
		})
	}
}

// Picking a folder changes the dropdown before the form submit leaves the page,
// so a back/forward-cache restore would show the new folder over the old list.
// On such a restore the page resets the dropdown to the server-rendered option.
// It must not reload: reloading a bare "/" would render the newer remembered
// state and break Back for every filter.
func TestIndex_ResetsFolderSelectOnBackForwardCacheRestore(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	var script string
	parseBody(t, w.Body.String()).Find("head script:not([src])").Each(func(_ int, s *goquery.Selection) {
		if strings.Contains(s.Text(), "pageshow") {
			script = s.Text()
		}
	})
	require.NotEmpty(t, script, "expected an inline pageshow listener in <head>")
	assert.Contains(t, script, "persisted", "only a back/forward-cache restore may touch the dropdown")
	assert.Contains(t, script, "option[selected]", "the dropdown must be reset to the server-rendered option")
	assert.NotContains(t, script, "location.reload",
		"a reload would replace the restored view with the newer remembered state")
}

// ---------------------------------------------------------------------------
// Bug 27: без завършени статуси + „За DnD"
// ---------------------------------------------------------------------------

func TestIndex_StatusColumn_ShowsDnD(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Dragons.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})
	setStatus(t, d, "Dragons.mkv", "for_dnd")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	span := doc.Find("td.status span.status-dnd")
	require.Equal(t, 1, span.Length(), "expected a span.status-dnd in the status column")
	assert.Equal(t, "DnD", strings.TrimSpace(span.Text()))
}

func TestIndex_NoMarkedSortButton(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	body := w.Body.String()
	doc := parseBody(t, body)
	assert.Equal(t, 0, doc.Find("a[href*='sort=marked']").Length(), "the sort by completion date is removed")
	assert.NotContains(t, body, "По дата завършване")
}

func TestIndex_ForDnDFilterButton(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	// The status buttons of the "филтър:" group are ◎ ○ ▶ D, before ↕ and 🗑.
	var titles []string
	doc.Find(".filter-btns a.filter-btn").Each(func(_ int, sel *goquery.Selection) {
		title, _ := sel.Attr("title")
		titles = append(titles, title)
	})
	require.GreaterOrEqual(t, len(titles), 4)
	assert.Equal(t, []string{"Всички статуси", "Нова", "Стартирана", "За DnD"}, titles[:4])

	btn := doc.Find(".filter-btns a.filter-btn[title='За DnD']")
	require.Equal(t, 1, btn.Length(), "expected a single За DnD filter button in .filter-btns")
	assert.Equal(t, "D", strings.TrimSpace(btn.Text()))
	href, _ := btn.Attr("href")
	assert.Contains(t, href, "status=for_dnd")
	class, _ := btn.Attr("class")
	assert.NotContains(t, class, "filter-active", "button must be inactive by default")

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/?status=for_dnd", nil)
	newRouter(t, d).ServeHTTP(w, req)

	class, _ = parseBody(t, w.Body.String()).Find(".filter-btns a.filter-btn[title='За DnD']").Attr("class")
	assert.Contains(t, class, "filter-active", "button should be active when status=for_dnd")
}

// Bug 27: the tree's status dropdown offers the same three statuses. The e2e
// check skips when the scanned disk shows no tree files, so it is pinned here.
func TestTree_StatusOptions_AreNewStartedForDnD(t *testing.T) {
	d := openTestDB(t)
	seedOne(t, d)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tree", nil)
	newRouter(t, d).ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	sel := parseBody(t, w.Body.String()).Find(".tree-file select[name=status]")
	require.Equal(t, 1, sel.Length(), "expected one status dropdown for the seeded media")

	var values, labels []string
	sel.Find("option").Each(func(_ int, opt *goquery.Selection) {
		v, _ := opt.Attr("value")
		values = append(values, v)
		labels = append(labels, strings.TrimSpace(opt.Text()))
	})
	assert.Equal(t, []string{"new", "started", "for_dnd"}, values)
	assert.Equal(t, []string{"Нова", "Стартирана", "За DnD"}, labels)
}
