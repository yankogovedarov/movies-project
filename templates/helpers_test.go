package templates

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// defaultFilters is the list page's state on a fresh database.
var defaultFilters = Filters{
	Status: "all", Disk: "on", Sort: "name", Dir: "asc", Q: "",
	Del: "all", Dnd: "all", Subs: "all", Folder: "all",
}

func TestSortBtnHref_InactiveDateSort_StartsDescending(t *testing.T) {
	// Bug #19: first click on a date sort button must show most recent first.
	// Bug 27: "added" is the only date sort left ("marked" is removed).
	href := sortBtnHref(defaultFilters, "added")
	assert.Contains(t, href, "sort=added")
	assert.Contains(t, href, "dir=desc", "first click on added should sort descending")
}

func TestSortBtnHref_InactiveNonDateSort_StartsAscending(t *testing.T) {
	f := defaultFilters
	f.Sort, f.Dir = "added", "desc"
	for _, btn := range []string{"name", "path", "size", "last_started"} {
		href := sortBtnHref(f, btn)
		assert.Contains(t, href, "sort="+btn)
		assert.Contains(t, href, "dir=asc", "first click on %q should sort ascending", btn)
	}
}

func TestSortBtnHref_ActiveButton_TogglesDirection(t *testing.T) {
	cases := []struct {
		btn, currentDir, wantDir string
	}{
		{"added", "desc", "asc"},
		{"added", "asc", "desc"},
		{"name", "asc", "desc"},
		{"name", "desc", "asc"},
	}
	for _, tc := range cases {
		f := defaultFilters
		f.Sort, f.Dir = tc.btn, tc.currentDir
		href := sortBtnHref(f, tc.btn)
		assert.Contains(t, href, "dir="+tc.wantDir,
			"active %q with dir=%s should toggle to %s", tc.btn, tc.currentDir, tc.wantDir)
	}
}

// Bug 30: a sort link is the current list URL with only the sort changed, so it
// keeps every filter, the three flag filters included.
func TestSortBtnHref_PreservesFilters(t *testing.T) {
	f := Filters{Status: "started", Disk: "all", Sort: "name", Dir: "asc", Q: "matrix",
		Del: "yes", Dnd: "yes", Subs: "yes", Folder: "Serials"}
	assert.Equal(t, f.WithSort("added", "desc").Href(), sortBtnHref(f, "added"))
}

// Bug 26: folder names may contain spaces, Cyrillic or "&", so the value that
// goes into a href is always query-escaped.
func TestFolderParam_EscapesValue(t *testing.T) {
	assert.Equal(t, "&folder=all", folderParam("all"))
	assert.Equal(t, "&folder=A%26B+C", folderParam("A&B C"))
}

// Bug 30: every link of the nav bar is built from one Filters value, so Href
// must carry every field; the folder comes from the disk and is query-escaped.
func TestFilters_Href_CarriesEveryField(t *testing.T) {
	f := Filters{Status: "started", Disk: "all", Sort: "size", Dir: "desc", Q: "dune",
		Del: "yes", Dnd: "yes", Subs: "all", Folder: "99 Гледани & Co"}

	u, err := url.Parse(f.Href())
	require.NoError(t, err)
	assert.Equal(t, "/", u.Path)
	assert.Equal(t, url.Values{
		"status": {"started"},
		"disk":   {"all"},
		"sort":   {"size"},
		"dir":    {"desc"},
		"q":      {"dune"},
		"del":    {"yes"},
		"dnd":    {"yes"},
		"subs":   {"all"},
		"folder": {"99 Гледани & Co"},
	}, u.Query())
}

// Bug 33: 👍 and 👎 exclude each other, so a link that narrows to one of them
// clears the other's filter. A link that clears one leaves the other alone, and
// ⚠ takes no part.
func TestFilters_WithDndAndDel_Exclude(t *testing.T) {
	for _, tc := range []struct {
		name             string
		base             Filters
		with             func(Filters) Filters
		wantDnd, wantDel string
	}{
		{"👍 yes clears 👎", Filters{Dnd: "all", Del: "yes", Subs: "yes"},
			func(f Filters) Filters { return f.WithDnd("yes") }, "yes", "all"},
		{"👎 yes clears 👍", Filters{Dnd: "yes", Del: "all", Subs: "yes"},
			func(f Filters) Filters { return f.WithDel("yes") }, "all", "yes"},
		{"👍 all leaves 👎", Filters{Dnd: "yes", Del: "yes", Subs: "yes"},
			func(f Filters) Filters { return f.WithDnd("all") }, "all", "yes"},
		{"👎 all leaves 👍", Filters{Dnd: "yes", Del: "yes", Subs: "yes"},
			func(f Filters) Filters { return f.WithDel("all") }, "yes", "all"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.with(tc.base)
			assert.Equal(t, tc.wantDnd, got.Dnd, "👍 filter")
			assert.Equal(t, tc.wantDel, got.Del, "👎 filter")
			assert.Equal(t, "yes", got.Subs, "⚠ takes no part")
		})
	}
}

// Bug 30: each With method returns a copy with exactly one filter changed (in
// base 👍 and 👎 are "all", so the Bug 33 rule above changes nothing here).
func TestFilters_With_ChangesOneField(t *testing.T) {
	base := Filters{Status: "new", Disk: "on", Sort: "name", Dir: "asc", Q: "dune",
		Del: "all", Dnd: "all", Subs: "all", Folder: "Movies"}

	want := base
	want.Status = "started"
	assert.Equal(t, want, base.WithStatus("started"))

	want = base
	want.Disk = "all"
	assert.Equal(t, want, base.WithDisk("all"))

	want = base
	want.Sort, want.Dir = "size", "desc"
	assert.Equal(t, want, base.WithSort("size", "desc"))

	want = base
	want.Del = "yes"
	assert.Equal(t, want, base.WithDel("yes"))

	want = base
	want.Dnd = "yes"
	assert.Equal(t, want, base.WithDnd("yes"))

	want = base
	want.Subs = "yes"
	assert.Equal(t, want, base.WithSubs("yes"))
}

// Bug 30: the 👍 👎 ⚠ filter buttons share one toggle.
func TestToggleFilter(t *testing.T) {
	assert.Equal(t, "yes", toggleFilter("all"), "an inactive filter links to yes")
	assert.Equal(t, "all", toggleFilter("yes"), "an active filter links back to all")
}

// Bug 30: the 👍 👎 ⚠ row buttons share one toggle.
func TestToggleFlag(t *testing.T) {
	assert.Equal(t, "1", toggleFlag(0), "a lowered flag submits 1 (raise)")
	assert.Equal(t, "0", toggleFlag(1), "a raised flag submits 0 (clear)")
}

// Bug 30: the statuses are new / started. „За DnD" became the 👍 flag, so a
// leftover for_dnd is no status any more and renders like any unknown value.
func TestStatusClassAndShort(t *testing.T) {
	cases := []struct {
		status, class, short string
	}{
		{"new", "new", "Нова"},
		{"started", "started", "Старт"},
		{"for_dnd", "new", "Нова"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.class, statusClass(tc.status), "statusClass(%q)", tc.status)
		assert.Equal(t, tc.short, statusShort(tc.status), "statusShort(%q)", tc.status)
	}
}
