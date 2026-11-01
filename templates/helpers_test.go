package templates

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSortBtnHref_InactiveDateSort_StartsDescending(t *testing.T) {
	// Bug #19: first click on a date sort button must show most recent first.
	// Bug 27: "added" is the only date sort left ("marked" is removed).
	href := sortBtnHref("all", "on_disk", "name", "asc", "added", "", "all", "all", "all")
	assert.Contains(t, href, "sort=added")
	assert.Contains(t, href, "dir=desc", "first click on added should sort descending")
}

func TestSortBtnHref_InactiveNonDateSort_StartsAscending(t *testing.T) {
	for _, btn := range []string{"name", "path", "size", "last_started"} {
		href := sortBtnHref("all", "on_disk", "added", "desc", btn, "", "all", "all", "all")
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
		href := sortBtnHref("all", "on_disk", tc.btn, tc.currentDir, tc.btn, "", "all", "all", "all")
		assert.Contains(t, href, "dir="+tc.wantDir,
			"active %q with dir=%s should toggle to %s", tc.btn, tc.currentDir, tc.wantDir)
	}
}

func TestSortBtnHref_PreservesFilters(t *testing.T) {
	href := sortBtnHref("started", "all", "name", "asc", "added", "matrix", "sub", "yes", "Serials")
	assert.Contains(t, href, "status=started")
	assert.Contains(t, href, "disk=all")
	assert.Contains(t, href, "q=matrix")
	assert.Contains(t, href, "trans=sub")
	assert.Contains(t, href, "del=yes")
	assert.Contains(t, href, "folder=Serials")
}

// Bug 26: folder names may contain spaces, Cyrillic or "&", so the value that
// goes into a href is always query-escaped.
func TestFolderParam_EscapesValue(t *testing.T) {
	assert.Equal(t, "&folder=all", folderParam("all"))
	assert.Equal(t, "&folder=A%26B+C", folderParam("A&B C"))
}

func TestDelToggleValue(t *testing.T) {
	assert.Equal(t, "yes", delToggleValue("all"), "inactive filter links to del=yes")
	assert.Equal(t, "all", delToggleValue("yes"), "active filter links back to del=all")
}

func TestForDeletionToggleValue(t *testing.T) {
	assert.Equal(t, "1", forDeletionToggleValue(0), "unmarked media submits 1 (raise)")
	assert.Equal(t, "0", forDeletionToggleValue(1), "marked media submits 0 (clear)")
}

// Bug 27: the statuses are new / started / for_dnd; "За DnD" gets its own CSS
// class and the short label "DnD" in the status column.
func TestStatusClassAndShort(t *testing.T) {
	cases := []struct {
		status, class, short string
	}{
		{"new", "new", "Нова"},
		{"started", "started", "Старт"},
		{"for_dnd", "dnd", "DnD"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.class, statusClass(tc.status), "statusClass(%q)", tc.status)
		assert.Equal(t, tc.short, statusShort(tc.status), "statusShort(%q)", tc.status)
	}
}
