//go:build windows

package scanner

import (
	"os"
	"syscall"
	"time"
)

func fileCreatedAt(path string) time.Time {
	fi, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	if stat, ok := fi.Sys().(*syscall.Win32FileAttributeData); ok {
		return time.Unix(0, stat.CreationTime.Nanoseconds())
	}
	return fi.ModTime()
}
