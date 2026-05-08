package templates

import "fmt"

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
