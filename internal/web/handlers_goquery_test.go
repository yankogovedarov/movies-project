package web_test

import (
	"context"
	"database/sql"
	"encoding/json"
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
	rows := doc.Find("#media-list tbody tr")
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
	btn := doc.Find("#media-list td.filename form button.filename-link")
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
	form := doc.Find("#media-list td.filename form")
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
	forms := doc.Find("#media-list td.actions form")
	require.Equal(t, 5, forms.Length(), "expected 5 action forms (2 status + 3 flags)")
	posts := map[string]int{}
	forms.Each(func(_ int, s *goquery.Selection) {
		hxPost, exists := s.Attr("hx-post")
		assert.True(t, exists, "expected hx-post on action form")
		switch {
		case strings.Contains(hxPost, "/status"):
			posts["status"]++
		case strings.Contains(hxPost, "/for-dnd"):
			posts["for-dnd"]++
		case strings.Contains(hxPost, "/for-deletion"):
			posts["for-deletion"]++
		case strings.Contains(hxPost, "/wrong-subs"):
			posts["wrong-subs"]++
		default:
			t.Errorf("unexpected hx-post %q in actions column", hxPost)
		}
		hxTarget, _ := s.Attr("hx-target")
		assert.Equal(t, "closest tr", hxTarget)
	})
	assert.Equal(t, map[string]int{"status": 2, "for-dnd": 1, "for-deletion": 1, "wrong-subs": 1}, posts,
		"expected 2 status forms and one form per flag")
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
	folder := doc.Find("#media-list td.folder")
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
	action, exists := doc.Find("#media-list td.filename form").Attr("action")
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
	action, exists := doc.Find("#media-list td.actions form").First().Attr("action")
	require.True(t, exists, "expected form in actions cell")
	assert.Contains(t, action, "/status", "status form action should contain /status")
}

// Bug 30: the row offers the statuses ○ ▶ and the three flags 👍 👎 ⚠ — „За DnD"
// (the D status of Bug 27) is a flag now.
func TestIndex_Actions_HasTwoStatusAndThreeFlagForms(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Movie.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	statusForms := doc.Find("#media-list td.actions form").FilterFunction(func(_ int, s *goquery.Selection) bool {
		return s.Find("input[name=status]").Length() == 1
	})
	assert.Equal(t, 2, statusForms.Length(), "expected 2 status forms in actions column")

	var statusValues []string
	statusForms.Each(func(_ int, s *goquery.Selection) {
		v, _ := s.Find("input[name=status]").Attr("value")
		statusValues = append(statusValues, v)
	})
	assert.Equal(t, []string{"new", "started"}, statusValues)

	flagForms := doc.Find("#media-list td.actions form").FilterFunction(func(_ int, s *goquery.Selection) bool {
		return s.Find("input[name=value]").Length() == 1
	})
	assert.Equal(t, 3, flagForms.Length(), "expected 3 flag forms in actions column")
}

func TestIndex_FilterButtons_Present(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	btns := doc.Find("a.filter-btn")
	assert.Equal(t, 7, btns.Length(), "expected 7 filter buttons (3 status + 1 disk toggle + 3 flags)")
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
	startInfo := doc.Find("#media-list .start-info")
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
	startInfo := doc.Find("#media-list .start-info")
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

	for _, name := range []string{"status", "disk", "q", "del", "folder"} {
		assert.Equal(t, 1, randomForm.Find(fmt.Sprintf("input[name=%s]", name)).Length(),
			"expected hidden input %q in random form", name)
	}
}

func TestIndex_StickyNav_CSS(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	body := w.Body.String()
	// Bug 16 part 5: filter bar stays visible while scrolling.
	assert.Contains(t, body, "position: sticky", "expected sticky .top-nav CSS")
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
	// Every status/↕ link and every sort link must carry del=yes, otherwise
	// clicking one of them silently drops the flag filter.
	doc.Find(".filter-btns a.filter-btn, .sort-btns a.sort-btn").
		Each(func(_ int, sel *goquery.Selection) {
			title, _ := sel.Attr("title")
			href, _ := sel.Attr("href")
			switch title {
			case "За изтриване":
				// This one intentionally toggles back to del=all.
			case "За DnD":
				assert.Contains(t, href, "del=all", "Bug 33: 👍 and 👎 exclude each other, so the 👍 link clears 👎")
			default:
				assert.Contains(t, href, "del=yes", "link %q must preserve the за-изтриване filter", title)
			}
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
	btns := doc.Find("#media-list td.actions button.icon-btn[title='За изтриване']")
	require.Equal(t, 2, btns.Length(), "expected one 👎 button per row")

	byFile := map[string]*goquery.Selection{}
	doc.Find("#media-list tbody tr").Each(func(_ int, row *goquery.Selection) {
		name := strings.TrimSpace(row.Find("td.filename button.filename-link").Text())
		byFile[name] = row
	})

	// Every flag has its own form in the row (Bug 30), so the value is read
	// from the 👎 button's form.
	marked := byFile["Trash.mkv"].Find("td.actions button.icon-btn[title='За изтриване']")
	markedClass, _ := marked.Attr("class")
	assert.Contains(t, markedClass, "filter-active", "marked media must show a highlighted 👎")
	markedValue, _ := marked.Closest("form").Find("input[name=value]").Attr("value")
	assert.Equal(t, "0", markedValue, "clicking a marked media must clear the flag")

	plain := byFile["Keep.mkv"].Find("td.actions button.icon-btn[title='За изтриване']")
	plainClass, _ := plain.Attr("class")
	assert.NotContains(t, plainClass, "filter-active")
	plainValue, _ := plain.Closest("form").Find("input[name=value]").Attr("value")
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
	req := httptest.NewRequest(http.MethodGet, "/?status=new&sort=size&dir=desc&q=god&del=yes", nil)
	newRouter(t, d).ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())

	statusClass, _ := doc.Find("a.filter-btn[title='Нова']").Attr("class")
	assert.Contains(t, statusClass, "filter-active")
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

	// Neither the chosen folder nor a status that matches nothing (every seeded
	// media is new) may shrink the dropdown — the user must be able to switch
	// straight to another folder.
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/?folder=Serials&status=started", nil)
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
			// Every status/↕/flag link and every sort link carries the folder
			// (escaped), otherwise clicking one of them silently drops the filter.
			links := doc.Find(".filter-btns a.filter-btn, .sort-btns a.sort-btn")
			require.Equal(t, 12, links.Length(), "expected 7 filter + 5 sort links")
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

// Bug 30: the tree's status dropdown offers the same two statuses — „За DnD"
// is a flag now. The e2e check skips when the scanned disk shows no tree
// files, so it is pinned here.
func TestTree_StatusOptions_AreNewStarted(t *testing.T) {
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
	assert.Equal(t, []string{"new", "started"}, values)
	assert.Equal(t, []string{"Нова", "Стартирана"}, labels)
}

// ---------------------------------------------------------------------------
// Bug 27: 🎲 изписва филма + без „Детайли"
// ---------------------------------------------------------------------------

// The row responses (a row plus out-of-band rows, Bug 32) only keep their table
// context when htmx parses partial responses in a <template>. htmx reads its
// config from a JSON meta tag.
func TestIndex_HtmxConfigUsesTemplateFragments(t *testing.T) {
	d := openTestDB(t)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	meta := parseBody(t, w.Body.String()).Find("head meta[name=htmx-config]")
	require.Equal(t, 1, meta.Length(), "expected one htmx-config meta tag in <head>")
	content, _ := meta.Attr("content")
	var cfg map[string]any
	require.NoError(t, json.Unmarshal([]byte(content), &cfg), "htmx-config must be valid JSON, got %q", content)
	assert.Equal(t, true, cfg["useTemplateFragments"])
}

func TestIndex_NoDetailsButton(t *testing.T) {
	d := openTestDB(t)
	id := seedOne(t, d)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	doc := parseBody(t, w.Body.String())
	require.Equal(t, 1, doc.Find("#media-list tbody tr").Length(), "expected the seeded media row")
	assert.Equal(t, 0, doc.Find("[title='Детайли']").Length(), "the ℹ „Детайли\" button must be gone")
	assert.Equal(t, 0, doc.Find(fmt.Sprintf("[href='/media/%d']", id)).Length(),
		"no link may lead to the removed details view")
}

// ---------------------------------------------------------------------------
// Bug 30: без превод
// ---------------------------------------------------------------------------

// The translation feature is removed: no „Пр" column with СУБ/БГ/ОРГ in the
// row, no „превод:" group in the nav bar and no trans parameter in any link or
// form.
func TestIndex_NoTranslationUI(t *testing.T) {
	d := openTestDB(t)
	seedOne(t, d)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	body := w.Body.String()
	doc := parseBody(t, body)
	require.Equal(t, 1, doc.Find("#media-list tbody tr").Length(), "expected the seeded media row")
	assert.Equal(t, 0, doc.Find("th.translation, td.translation").Length(), "the „Пр\" column must be gone")
	assert.NotContains(t, body, "превод:", "the „превод:\" label must be gone")
	assert.Equal(t, 0, doc.Find(".trans-btns").Length(), "the translation filter group must be gone")
	assert.NotContains(t, body, "/translation-type", "nothing may post to the removed route")
	assert.Equal(t, 0, doc.Find("a[href*='trans=']").Length(), "no link may carry the removed trans parameter")
	assert.Equal(t, 0, doc.Find("input[name=trans]").Length(), "no form may re-send the removed trans parameter")
	assert.NotContains(t, body, "td.translation", "no CSS may be left for the removed column")
	assert.NotContains(t, body, ".trans-btns", "no CSS may be left for the removed group")
}

// ---------------------------------------------------------------------------
// Bug 30: the three flags 👍 👎 ⚠ (👍 and 👎 exclude each other since Bug 33, see below)
// ---------------------------------------------------------------------------

// Next to ○ ▶ the row offers the three flags 👍 👎 ⚠, each a form that posts to
// its own route and swaps the row in place. A raised flag is highlighted and its
// button clears it (value 0); a lowered one raises it. Each button renders its
// own column, so the seed writes all three straight into the DB, although the
// routes never leave 👍 and 👎 raised together (Bug 33).
func TestIndex_RowFlagButtons(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Flagged.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
		{Filename: "Plain.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000},
	})
	for _, column := range []string{"for_dnd", "for_deletion", "wrong_subs"} {
		raiseFlag(t, d, column, "Flagged.mkv")
	}

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	rows := map[string]*goquery.Selection{}
	parseBody(t, w.Body.String()).Find("#media-list tbody tr").Each(func(_ int, row *goquery.Selection) {
		rows[strings.TrimSpace(row.Find("td.filename button.filename-link").Text())] = row
	})
	require.Len(t, rows, 2, "expected both seeded rows")

	var icons []string
	rows["Plain.mkv"].Find("td.actions button.icon-btn").Each(func(_ int, btn *goquery.Selection) {
		icons = append(icons, strings.TrimSpace(btn.Text()))
	})
	assert.Equal(t, []string{"○", "▶", "👍", "👎", "⚠️"}, icons, "the row reads ○ ▶ 👍 👎 ⚠️")

	for _, flag := range []struct{ title, icon, route string }{
		{"За DnD", "👍", "for-dnd"},
		{"За изтриване", "👎", "for-deletion"},
		{"Грешни субтитри", "⚠️", "wrong-subs"},
	} {
		for _, tc := range []struct {
			file, value string
			raised      bool
		}{
			{"Flagged.mkv", "0", true},
			{"Plain.mkv", "1", false},
		} {
			btn := rows[tc.file].Find(fmt.Sprintf("td.actions button.icon-btn[title='%s']", flag.title))
			require.Equal(t, 1, btn.Length(), "%s: expected one %s button", tc.file, flag.title)
			assert.Equal(t, flag.icon, strings.TrimSpace(btn.Text()))

			form := btn.Closest("form")
			route := fmt.Sprintf("/media/%d/%s", mediaIDByName(t, d, tc.file), flag.route)
			assert.Equal(t, route, form.AttrOr("hx-post", ""), "%s: %s", tc.file, flag.title)
			assert.Equal(t, route, form.AttrOr("action", ""), "the form must also work without HTMX")
			assert.Equal(t, "closest tr", form.AttrOr("hx-target", ""))
			assert.Equal(t, "outerHTML", form.AttrOr("hx-swap", ""))

			value, _ := form.Find("input[type=hidden][name=value]").Attr("value")
			assert.Equal(t, tc.value, value, "%s: %s must toggle the flag", tc.file, flag.title)
			class, _ := btn.Attr("class")
			if tc.raised {
				assert.Contains(t, class, "filter-active", "%s: a raised %s is highlighted", tc.file, flag.title)
			} else {
				assert.NotContains(t, class, "filter-active", "%s: a lowered %s is not highlighted", tc.file, flag.title)
			}
		}
	}
}

// The "филтър:" group reads [folder] ◎ ○ ▶ ↕ 👍 👎 ⚠ — the flag filters gather
// after ↕. Each toggles its own parameter (yes ⇄ all) and is highlighted while
// it narrows the list.
func TestIndex_FilterBar_FlagButtons(t *testing.T) {
	// Every page comes from a fresh database, so no request inherits a filter
	// another one remembered.
	get := func(t *testing.T, target string) *goquery.Document {
		t.Helper()
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, target, nil)
		newRouter(t, openTestDB(t)).ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		return parseBody(t, w.Body.String())
	}

	var icons []string
	get(t, "/").Find(".filter-btns a.filter-btn").Each(func(_ int, sel *goquery.Selection) {
		icons = append(icons, strings.TrimSpace(sel.Text()))
	})
	assert.Equal(t, []string{"◎", "○", "▶", "↕", "👍", "👎", "⚠️"}, icons)

	for _, tc := range []struct{ param, title, icon string }{
		{"dnd", "За DnD", "👍"},
		{"del", "За изтриване", "👎"},
		{"subs", "Грешни субтитри", "⚠️"},
	} {
		t.Run(tc.param, func(t *testing.T) {
			sel := fmt.Sprintf(".filter-btns a.filter-btn[title='%s']", tc.title)

			btn := get(t, "/").Find(sel)
			require.Equal(t, 1, btn.Length(), "expected one %s filter button", tc.title)
			assert.Equal(t, tc.icon, strings.TrimSpace(btn.Text()))
			assert.Contains(t, btn.AttrOr("href", ""), tc.param+"=yes", "an inactive button narrows the list")
			assert.NotContains(t, btn.AttrOr("class", ""), "filter-active", "the button is inactive by default")

			btn = get(t, "/?"+tc.param+"=yes").Find(sel)
			require.Equal(t, 1, btn.Length(), "expected one %s filter button", tc.title)
			assert.Contains(t, btn.AttrOr("href", ""), tc.param+"=all", "an active button links back to all")
			assert.Contains(t, btn.AttrOr("class", ""), "filter-active", "the button is active while %s=yes", tc.param)
		})
	}
}

// Every link of the nav bar and both forms carry the flag filters, otherwise
// clicking one of them silently drops 👍 or ⚠.
func TestIndex_FlagFilters_PreservedByOtherFilters(t *testing.T) {
	d := openTestDB(t)
	// Both ↕ variants, because the disk toggle renders a different link for each.
	for _, tc := range []struct{ name, target string }{
		{"on disk", "/?dnd=yes&subs=yes"},
		{"all", "/?disk=all&dnd=yes&subs=yes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.target, nil)
			newRouter(t, d).ServeHTTP(w, req)

			doc := parseBody(t, w.Body.String())
			links := doc.Find(".filter-btns a.filter-btn, .sort-btns a.sort-btn")
			require.Equal(t, 12, links.Length(), "expected 7 filter + 5 sort links")
			links.Each(func(_ int, sel *goquery.Selection) {
				title, _ := sel.Attr("title")
				href, _ := sel.Attr("href")
				// A flag's own button intentionally toggles back to all, and the 👎
				// link intentionally clears 👍 (Bug 33: they exclude each other).
				switch title {
				case "За DnD":
				case "За изтриване":
					assert.Contains(t, href, "dnd=all", "Bug 33: the 👎 link must clear the 👍 filter")
				default:
					assert.Contains(t, href, "dnd=yes", "link %q must preserve the 👍 filter", title)
				}
				if title != "Грешни субтитри" {
					assert.Contains(t, href, "subs=yes", "link %q must preserve the ⚠ filter", title)
				}
			})

			// The search form (live search and the folder dropdown) and the 🎲 form
			// re-send both filters as hidden fields.
			for _, form := range []string{"form#search-form", "form[action='/media/random-new/start']"} {
				for _, name := range []string{"dnd", "subs"} {
					value, ok := doc.Find(form + " input[type=hidden][name=" + name + "]").Attr("value")
					require.True(t, ok, "%s: expected a hidden %s input", form, name)
					assert.Equal(t, "yes", value, "%s must re-send %s=yes", form, name)
				}
			}
		})
	}
}

// „За DnD" is a flag now: the status column shows only „Нова" or „Старт", also
// for a media with 👍 raised, and the green DnD label is gone.
func TestIndex_StatusColumn_NewOrStarted(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "DnDNew.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
		{Filename: "DnDStarted.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000},
	})
	raiseFlag(t, d, "for_dnd", "DnDNew.mkv")
	raiseFlag(t, d, "for_dnd", "DnDStarted.mkv")
	setStatus(t, d, "DnDStarted.mkv", "started")

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	newRouter(t, d).ServeHTTP(w, req)

	body := w.Body.String()
	doc := parseBody(t, body)
	statuses := map[string]string{}
	doc.Find("#media-list tbody tr").Each(func(_ int, row *goquery.Selection) {
		name := strings.TrimSpace(row.Find("td.filename button.filename-link").Text())
		statuses[name] = strings.TrimSpace(row.Find("td.status").Text())
	})
	assert.Equal(t, map[string]string{"DnDNew.mkv": "Нова", "DnDStarted.mkv": "Старт"}, statuses)
	assert.Equal(t, 0, doc.Find("span.status-dnd").Length(), "the DnD status label must be gone")
	assert.NotContains(t, body, ".status-dnd", "no CSS may be left for the removed label")
}

// ---------------------------------------------------------------------------
// Bug 32: последно стартираният филм над таблицата
// ---------------------------------------------------------------------------

// getPage renders target and parses it. Every request remembers its filters, so
// a test that needs a clean state uses its own database.
func getPage(t *testing.T, d *sql.DB, target string) *goquery.Document {
	t.Helper()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	newRouter(t, d).ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	return parseBody(t, w.Body.String())
}

// The row above the list is a copy of the last started media's list row: the
// name starts it again, the folder opens Explorer, then size, status and the
// ○ ▶ 👍 👎 ⚠ forms.
func TestIndex_LastStartedRow_ShowsLastStarted(t *testing.T) {
	d := openTestDB(t)
	seedMedia(t, d, []scanner.VideoFile{
		{Filename: "Alpha.mkv", FolderRelativePath: `Films\Alpha`, SizeBytes: 1_000_000_000},
		{Filename: "Beta.mkv", FolderRelativePath: `Films\Beta`, SizeBytes: 2_500_000_000},
	})
	beta := mediaIDByName(t, d, "Beta.mkv")
	startViaRoute(t, d, mediaIDByName(t, d, "Alpha.mkv"))
	startViaRoute(t, d, beta)

	row := getPage(t, d, "/").Find("table.last-started tr#last-started")
	require.Equal(t, 1, row.Length(), "expected the last started row in its own table")
	_, hidden := row.Attr("hidden")
	assert.False(t, hidden, "a started media makes the row visible")

	start := fmt.Sprintf("/media/%d/start", beta)
	form := row.Find("td.filename form")
	assert.Equal(t, start, form.AttrOr("action", ""), "the name starts the media again")
	assert.Equal(t, start, form.AttrOr("hx-post", ""))
	assert.Equal(t, "closest tr", form.AttrOr("hx-target", ""))
	assert.Equal(t, "Beta.mkv", strings.TrimSpace(form.Find("button.filename-link").Text()))
	assert.Contains(t, row.Find("td.filename .start-info").Text(), "Стартирана 1 пъти")

	folder := row.Find("td.folder a.folder-link")
	assert.Equal(t, fmt.Sprintf("/media/%d/open-folder", beta), folder.AttrOr("hx-get", ""), "the folder opens Explorer")
	assert.Equal(t, `Films\Beta`, strings.TrimSpace(folder.Text()))

	assert.Equal(t, "2.3 GB", strings.TrimSpace(row.Find("td.size").Text()))
	assert.Equal(t, "Старт", strings.TrimSpace(row.Find("td.status").Text()))

	forms := row.Find("td.actions form")
	require.Equal(t, 5, forms.Length(), "expected the ○ ▶ 👍 👎 ⚠ forms")
	var icons []string
	forms.Each(func(_ int, f *goquery.Selection) {
		assert.Contains(t, f.AttrOr("hx-post", ""), fmt.Sprintf("/media/%d/", beta))
		assert.Equal(t, "closest tr", f.AttrOr("hx-target", ""))
		icons = append(icons, strings.TrimSpace(f.Find("button.icon-btn").Text()))
	})
	assert.Equal(t, []string{"○", "▶", "👍", "👎", "⚠️"}, icons)
}

// With nothing started the row is still there — empty and hidden — so the first
// start has an element to swap out of band.
func TestIndex_LastStartedRow_HiddenWhenNothingStarted(t *testing.T) {
	d := openTestDB(t)
	seedOne(t, d)

	row := getPage(t, d, "/").Find("table.last-started tr#last-started")
	require.Equal(t, 1, row.Length(), "expected the empty last started row")
	_, hidden := row.Attr("hidden")
	assert.True(t, hidden, "the row must be hidden while nothing was started")
	assert.Equal(t, 0, row.Find("td").Length(), "the empty row has no cells")
}

// The row shows the last started media whatever the list shows: a status
// filter, a search text or ↕ (the media is no longer on the disk) may hide it
// from the list, but not from the row above.
func TestIndex_LastStartedRow_IgnoresFilters(t *testing.T) {
	for _, tc := range []struct {
		name, target string
		offDisk      bool
	}{
		{"status", "/?status=new", false},
		{"search", "/?q=beta", false},
		{"off disk", "/", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := openTestDB(t)
			seedMedia(t, d, []scanner.VideoFile{
				{Filename: "Alpha.mkv", FolderRelativePath: "Films", SizeBytes: 1_000_000_000},
				{Filename: "Beta.mkv", FolderRelativePath: "Films", SizeBytes: 2_000_000_000},
			})
			startViaRoute(t, d, mediaIDByName(t, d, "Alpha.mkv"))
			if tc.offDisk {
				_, err := d.Exec("UPDATE media SET on_disk = 0 WHERE filename = 'Alpha.mkv'")
				require.NoError(t, err)
			}

			doc := getPage(t, d, tc.target)
			require.Contains(t, doc.Find("#media-list").Text(), "Beta.mkv", "the list must still show the other media")
			assert.NotContains(t, doc.Find("#media-list").Text(), "Alpha.mkv", "the filter must hide the media from the list")
			assert.Equal(t, "Alpha.mkv",
				strings.TrimSpace(doc.Find("tr#last-started td.filename button.filename-link").Text()),
				"the row above the list must still show the last started media")
		})
	}
}

// The row sits between the flash and the list, outside #media-list: the live
// search swaps only #media-list (hx-select), so it never touches the row.
func TestIndex_LastStartedRow_AboveListOutsideMediaList(t *testing.T) {
	d := openTestDB(t)
	startViaRoute(t, d, seedOne(t, d))

	doc := getPage(t, d, "/")
	assert.Equal(t, 0, doc.Find("#media-list #last-started").Length(), "the row must not be inside #media-list")
	assert.Equal(t, 0, doc.Find("nav #last-started").Length(), "the row must not be inside the nav bar")
	var order []string
	doc.Find("#flash, #last-started, #media-list").Each(func(_ int, s *goquery.Selection) {
		order = append(order, s.AttrOr("id", ""))
	})
	assert.Equal(t, []string{"flash", "last-started", "media-list"}, order)
}

// ---------------------------------------------------------------------------
// Bug 33: 👍 👎 ⚠ are grey until selected; 👍 and 👎 exclude each other
// ---------------------------------------------------------------------------

// Clicking 👎 while 👍 narrows the list leads to 👎 alone, and the other way
// round: the link itself clears the other filter. ⚠ takes no part: its link keeps
// 👍 and 👎, and their links keep ⚠.
func TestIndex_FlagFilterLinks_Exclude(t *testing.T) {
	for _, tc := range []struct {
		name, target string
		// The {dnd, del} values each flag's link must carry.
		wantLike, wantDislike, wantSubs [2]string
	}{
		{
			name: "👍 active", target: "/?dnd=yes&del=all&subs=yes",
			wantLike:    [2]string{"all", "all"}, // 👍 toggles back
			wantDislike: [2]string{"all", "yes"}, // 👎 takes over
			wantSubs:    [2]string{"yes", "all"}, // ⚠ keeps 👍
		},
		{
			name: "👎 active", target: "/?dnd=all&del=yes&subs=yes",
			wantLike:    [2]string{"yes", "all"}, // 👍 takes over
			wantDislike: [2]string{"all", "all"}, // 👎 toggles back
			wantSubs:    [2]string{"all", "yes"}, // ⚠ keeps 👎
		},
		{
			name: "neither active", target: "/?dnd=all&del=all&subs=yes",
			wantLike:    [2]string{"yes", "all"},
			wantDislike: [2]string{"all", "yes"},
			wantSubs:    [2]string{"all", "all"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := getPage(t, openTestDB(t), tc.target)

			for _, link := range []struct {
				title    string
				want     [2]string
				wantSubs string
			}{
				{"За DnD", tc.wantLike, "yes"},
				{"За изтриване", tc.wantDislike, "yes"},
				{"Грешни субтитри", tc.wantSubs, "all"}, // its own button toggles back
			} {
				btn := doc.Find(fmt.Sprintf(".filter-btns a.filter-btn[title='%s']", link.title))
				require.Equal(t, 1, btn.Length(), "expected one %s filter button", link.title)
				u, err := url.Parse(btn.AttrOr("href", ""))
				require.NoError(t, err)
				assert.Equal(t, link.want[0], u.Query().Get("dnd"), "link %q: 👍 filter", link.title)
				assert.Equal(t, link.want[1], u.Query().Get("del"), "link %q: 👎 filter", link.title)
				assert.Equal(t, link.wantSubs, u.Query().Get("subs"), "link %q: ⚠ filter", link.title)
			}
		})
	}
}

// The 👍 👎 ⚠ icons are colour emoji, so each sits in a span.flag-icon the CSS
// turns grey until it is selected — in the list row, in the row above the list
// and in the nav bar. The other icons (○ ▶ ◎ ↕) are plain text and have no span.
func TestIndex_FlagIcons_HaveFlagIconSpan(t *testing.T) {
	d := openTestDB(t)
	startViaRoute(t, d, seedOne(t, d))
	doc := getPage(t, d, "/")

	for _, scope := range []struct{ name, selector string }{
		{"list row", "#media-list td.actions button.icon-btn[title='%s']"},
		{"row above the list", "table.last-started td.actions button.icon-btn[title='%s']"},
		{"nav bar", ".filter-btns a.filter-btn[title='%s']"},
	} {
		for _, flag := range []struct{ title, icon string }{
			{"За DnD", "👍"},
			{"За изтриване", "👎"},
			{"Грешни субтитри", "⚠️"},
		} {
			btn := doc.Find(fmt.Sprintf(scope.selector, flag.title))
			require.Equal(t, 1, btn.Length(), "%s: expected one %s button", scope.name, flag.title)
			icon := btn.Find("span.flag-icon")
			require.Equal(t, 1, icon.Length(), "%s: the %s icon must sit in span.flag-icon", scope.name, flag.title)
			assert.Equal(t, flag.icon, strings.TrimSpace(icon.Text()), "%s: %s", scope.name, flag.title)
		}

		for _, title := range []string{"Нова", "Стартирана"} {
			btn := doc.Find(fmt.Sprintf(scope.selector, title))
			require.Equal(t, 1, btn.Length(), "%s: expected one %s button", scope.name, title)
			assert.Equal(t, 0, btn.Find("span.flag-icon").Length(), "%s: %s is no flag", scope.name, title)
		}
	}

	allStatuses := doc.Find(".filter-btns a.filter-btn[title='Всички статуси']")
	require.Equal(t, 1, allStatuses.Length(), "expected the ◎ filter button")
	assert.Equal(t, 0, allStatuses.Find("span.flag-icon").Length(), "◎ is no flag")
	disk := doc.Find(".filter-btns a.filter-btn[title*='кликни']")
	require.Equal(t, 1, disk.Length(), "expected the ↕ filter button")
	assert.Equal(t, 0, disk.Find("span.flag-icon").Length(), "↕ is no flag")

	assert.Equal(t, 9, doc.Find("span.flag-icon").Length(), "only the three flags of the two rows and the nav bar have it")
}
