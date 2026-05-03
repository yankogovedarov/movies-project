package scanner_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yankogovedarov/movie-tracker/internal/scanner"
)

const MB = 1024 * 1024

func TestFilterSamples_RemovesSample(t *testing.T) {
	files := []scanner.VideoFile{
		{Filename: "Movie.mkv", SizeBytes: 800 * MB, FolderRelativePath: "Films/Movie"},
		{Filename: "Sample.mkv", SizeBytes: 30 * MB, FolderRelativePath: "Films/Movie"},
	}
	result := scanner.FilterSamples(files)
	assert.Len(t, result, 1)
	assert.Equal(t, "Movie.mkv", result[0].Filename)
}

func TestFilterSamples_KeepsSmallOnlyFolder(t *testing.T) {
	files := []scanner.VideoFile{
		{Filename: "Short.mkv", SizeBytes: 20 * MB, FolderRelativePath: "Shorts"},
	}
	result := scanner.FilterSamples(files)
	assert.Len(t, result, 1)
}

func TestFilterSamples_KeepsAllWhenNoLargeFile(t *testing.T) {
	files := []scanner.VideoFile{
		{Filename: "Ep01.mkv", SizeBytes: 45 * MB, FolderRelativePath: "Series/S01"},
		{Filename: "Ep02.mkv", SizeBytes: 40 * MB, FolderRelativePath: "Series/S01"},
	}
	result := scanner.FilterSamples(files)
	assert.Len(t, result, 2)
}
