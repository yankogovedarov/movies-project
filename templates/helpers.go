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

// statusClass returns the CSS class suffix of a status (Bug 27: the statuses
// are new / started / for_dnd).
func statusClass(status string) string {
	switch status {
	case "started":
		return "started"
	case "for_dnd":
		return "dnd"
	default:
		return "new"
	}
}

// statusShort returns the short label shown in the "Статус" column.
func statusShort(status string) string {
	switch status {
	case "started":
		return "Старт"
	case "for_dnd":
		return "DnD"
	default:
		return "Нова"
	}
}

// defaultSortDir returns the direction for the first click on a sort button.
// The date sort (added) starts descending — most recent first (Bug #19).
func defaultSortDir(btnSort string) string {
	if btnSort == "added" {
		return "desc"
	}
	return "asc"
}

// sortBtnHref returns the URL for a sort button click.
// Clicking an active button toggles the direction; clicking an inactive button
// resets to that button's default direction.
func sortBtnHref(status, disk, currentSort, currentDir, btnSort, q, trans, del, folder string) string {
	newDir := defaultSortDir(btnSort)
	if currentSort == btnSort {
		if currentDir == "desc" {
			newDir = "asc"
		} else {
			newDir = "desc"
		}
	}
	return "/?status=" + status + "&disk=" + disk + "&sort=" + btnSort + "&dir=" + newDir + "&q=" + q + "&trans=" + trans + "&del=" + del + folderParam(folder)
}

// folderParam returns the query-string fragment that carries the main-folder
// filter (Bug 26). Folder names may contain spaces, Cyrillic or "&", so the
// value is always query-escaped.
func folderParam(folder string) string {
	return "&folder=" + url.QueryEscape(folder)
}

// translationToggleValue returns "" (unset) if current==target, else target.
func translationToggleValue(current, target string) string {
	if current == target {
		return ""
	}
	return target
}

// delToggleValue returns the value the "за изтриване" filter button links to:
// clicking it while active clears the filter, otherwise it narrows to marked media.
func delToggleValue(current string) string {
	if current == "yes" {
		return "all"
	}
	return "yes"
}

// forDeletionToggleValue returns the value a row's 🗑 button submits: "0" to
// clear an already raised flag, "1" to raise it.
func forDeletionToggleValue(current int64) string {
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
