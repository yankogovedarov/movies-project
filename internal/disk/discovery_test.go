package disk_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yankogovedarov/movie-tracker/internal/disk"
)

func TestDiscover(t *testing.T) {
	mockExe := func() (string, error) {
		return `D:\MovieTracker\movietracker.exe`, nil
	}
	paths, err := disk.Discover(mockExe)
	assert.NoError(t, err)
	assert.Equal(t, `D:\MovieTracker\`, paths.AppFolder)
	assert.Equal(t, `D:\`, paths.DiskRoot)
}
