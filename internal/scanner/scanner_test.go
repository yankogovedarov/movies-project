package scanner_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yankogovedarov/movie-tracker/internal/scanner"
)

func TestScan(t *testing.T) {
	root := t.TempDir()

	// Create test files
	moviesDir := filepath.Join(root, "Movies")
	os.MkdirAll(moviesDir, 0755)

	os.WriteFile(filepath.Join(moviesDir, "film.mkv"), []byte("fake mkv"), 0644)
	os.WriteFile(filepath.Join(moviesDir, "film.avi"), []byte("fake avi"), 0644)
	os.WriteFile(filepath.Join(root, "other.txt"), []byte("not video"), 0644)

	files, err := scanner.Scan(root)
	assert.NoError(t, err)
	assert.Len(t, files, 2)

	filenames := map[string]bool{}
	for _, f := range files {
		filenames[f.Filename] = true
	}
	assert.True(t, filenames["film.mkv"])
	assert.True(t, filenames["film.avi"])
	assert.False(t, filenames["other.txt"])
}
