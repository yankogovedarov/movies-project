package templates

import (
	"fmt"
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

func statusClass(status string) string {
	switch status {
	case "started":
		return "started"
	case "completed_both", "completed_yanko", "completed_liza":
		return "completed"
	default:
		return "new"
	}
}

func statusShort(status string) string {
	switch status {
	case "started":
		return "Старт"
	case "completed_both":
		return "c_Два"
	case "completed_yanko":
		return "c_Янко"
	case "completed_liza":
		return "c_Лиза"
	default:
		return "Нова"
	}
}

// defaultSortDir returns the direction for the first click on a sort button.
// Date sorts (added, marked) start descending — most recent first (Bug #19).
func defaultSortDir(btnSort string) string {
	if btnSort == "added" || btnSort == "marked" {
		return "desc"
	}
	return "asc"
}

// sortBtnHref returns the URL for a sort button click.
// Clicking an active button toggles the direction; clicking an inactive button
// resets to that button's default direction.
func sortBtnHref(status, disk, currentSort, currentDir, btnSort, q, trans, del string) string {
	newDir := defaultSortDir(btnSort)
	if currentSort == btnSort {
		if currentDir == "desc" {
			newDir = "asc"
		} else {
			newDir = "desc"
		}
	}
	return "/?status=" + status + "&disk=" + disk + "&sort=" + btnSort + "&dir=" + newDir + "&q=" + q + "&trans=" + trans + "&del=" + del
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
