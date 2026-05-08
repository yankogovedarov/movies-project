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

// sortBtnHref returns the URL for a sort button click.
// Clicking an active button toggles the direction; clicking an inactive button resets to asc.
func sortBtnHref(status, disk, currentSort, currentDir, btnSort string) string {
	newDir := "asc"
	if currentSort == btnSort && currentDir == "asc" {
		newDir = "desc"
	}
	return "/?status=" + status + "&disk=" + disk + "&sort=" + btnSort + "&dir=" + newDir
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
