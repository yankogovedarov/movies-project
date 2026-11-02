package templates

import (
	"fmt"
	"net/url"
	"strconv"
	"time"
)

func formatDate(t time.Time) string {
	return t.Format("02.01.2006")
}

func formatInt(n int64) string {
	return strconv.FormatInt(n, 10)
}

func formatSize(bytes int64) string {
	gb := float64(bytes) / 1_073_741_824
	if gb >= 1 {
		return fmt.Sprintf("%.1f GB", gb)
	}
	mb := float64(bytes) / 1_048_576
	return fmt.Sprintf("%.1f MB", mb)
}

// statusClass returns the CSS class suffix of a status (Bug 30: the statuses
// are new / started; „За DnD" is a flag now).
func statusClass(status string) string {
	switch status {
	case "started":
		return "started"
	default:
		return "new"
	}
}

// statusShort returns the short label shown in the "Статус" column.
func statusShort(status string) string {
	switch status {
	case "started":
		return "Старт"
	default:
		return "Нова"
	}
}

// Filters is the filter/sort state of the list page — every query parameter
// the links of the nav bar carry (Bug 30). Each link is built from one value,
// so a parameter cannot be left out of a single link by accident.
type Filters struct {
	Status string
	Disk   string
	Sort   string
	Dir    string
	Q      string
	Del    string
	Dnd    string
	Subs   string
	Folder string
}

// Href returns the list URL with every field. The folder is query-escaped
// (folderParam); q is passed through as it is (see Bug #31).
func (f Filters) Href() string {
	return "/?status=" + f.Status + "&disk=" + f.Disk + "&sort=" + f.Sort + "&dir=" + f.Dir +
		"&q=" + f.Q + "&del=" + f.Del + "&dnd=" + f.Dnd + "&subs=" + f.Subs + folderParam(f.Folder)
}

// WithStatus returns a copy of f with the status filter set to status.
func (f Filters) WithStatus(status string) Filters {
	f.Status = status
	return f
}

// WithDisk returns a copy of f with the ↕ disk filter set to disk.
func (f Filters) WithDisk(disk string) Filters {
	f.Disk = disk
	return f
}

// WithSort returns a copy of f sorted by sort in direction dir.
func (f Filters) WithSort(sort, dir string) Filters {
	f.Sort = sort
	f.Dir = dir
	return f
}

// WithDel returns a copy of f with the 👎 „За изтриване" filter set to del.
func (f Filters) WithDel(del string) Filters {
	f.Del = del
	return f
}

// WithDnd returns a copy of f with the 👍 „За DnD" filter set to dnd.
func (f Filters) WithDnd(dnd string) Filters {
	f.Dnd = dnd
	return f
}

// WithSubs returns a copy of f with the ⚠ „Грешни субтитри" filter set to subs.
func (f Filters) WithSubs(subs string) Filters {
	f.Subs = subs
	return f
}

// defaultSortDir returns the direction for the first click on a sort button.
// The date sort (added) starts descending — most recent first (Bug #19).
func defaultSortDir(btnSort string) string {
	if btnSort == "added" {
		return "desc"
	}
	return "asc"
}

// sortBtnHref returns the URL for a sort button click; the filters of f stay.
// Clicking an active button toggles the direction; clicking an inactive button
// resets to that button's default direction.
func sortBtnHref(f Filters, btnSort string) string {
	newDir := defaultSortDir(btnSort)
	if f.Sort == btnSort {
		if f.Dir == "desc" {
			newDir = "asc"
		} else {
			newDir = "desc"
		}
	}
	return f.WithSort(btnSort, newDir).Href()
}

// folderParam returns the query-string fragment that carries the main-folder
// filter (Bug 26). Folder names may contain spaces, Cyrillic or "&", so the
// value is always query-escaped.
func folderParam(folder string) string {
	return "&folder=" + url.QueryEscape(folder)
}

// toggleFilter returns the value a 👍 👎 ⚠ filter button links to: clicking
// it while active clears the filter, otherwise it narrows to the flagged media.
func toggleFilter(current string) string {
	if current == "yes" {
		return "all"
	}
	return "yes"
}

// toggleFlag returns the value a row's 👍 👎 ⚠ button submits: "0" to clear an
// already raised flag, "1" to raise it.
func toggleFlag(current int64) string {
	if current == 1 {
		return "0"
	}
	return "1"
}

// sortBtnContent returns the button label, appending ↑ or ↓ when the button is active.
func sortBtnContent(icon, currentSort, currentDir, btnSort string) string {
	if currentSort != btnSort {
		return icon
	}
	if currentDir == "asc" {
		return icon + "↑"
	}
	return icon + "↓"
}
