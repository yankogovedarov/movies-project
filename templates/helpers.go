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
