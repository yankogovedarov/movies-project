package templates

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSortBtnHref_InactiveDateSort_StartsDescending(t *testing.T) {
	// Bug #19: first click on a date sort button must show most recent first.
	for _, btn := range []string{"added", "marked"} {
		href := sortBtnHref("all", "on_disk", "name", "asc", btn, "", "all")
		assert.Contains(t, href, "sort="+btn)
		assert.Contains(t, href, "dir=desc", "first click on %q should sort descending", btn)
	}
}

func TestSortBtnHref_InactiveNonDateSort_StartsAscending(t *testing.T) {
	for _, btn := range []string{"name", "path", "size", "last_started"} {
		href := sortBtnHref("all", "on_disk", "marked", "desc", btn, "", "all")
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
		{"marked", "desc", "asc"},
		{"marked", "asc", "desc"},
		{"name", "asc", "desc"},
		{"name", "desc", "asc"},
	}
	for _, tc := range cases {
		href := sortBtnHref("all", "on_disk", tc.btn, tc.currentDir, tc.btn, "", "all")
		assert.Contains(t, href, "dir="+tc.wantDir,
			"active %q with dir=%s should toggle to %s", tc.btn, tc.currentDir, tc.wantDir)
	}
}

func TestSortBtnHref_PreservesFilters(t *testing.T) {
	href := sortBtnHref("started", "all", "name", "asc", "added", "matrix", "sub")
	assert.Contains(t, href, "status=started")
	assert.Contains(t, href, "disk=all")
	assert.Contains(t, href, "q=matrix")
	assert.Contains(t, href, "trans=sub")
}
