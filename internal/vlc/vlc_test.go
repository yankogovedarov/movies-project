package vlc_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yankogovedarov/movie-tracker/internal/vlc"
)

func writeTempFile(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte{}, 0o644))
	return path
}

func TestDetect_ReturnsFirstMatchingPath(t *testing.T) {
	dir := t.TempDir()
	absent := filepath.Join(dir, "absent.exe")
	present := writeTempFile(t, dir, "vlc.exe")

	got, err := vlc.Detect([]string{absent, present}, "")
	require.NoError(t, err)
	assert.Equal(t, present, got)
}

func TestDetect_UsesConfigOverride_WhenNoStandardFound(t *testing.T) {
	dir := t.TempDir()
	absent := filepath.Join(dir, "absent.exe")
	override := writeTempFile(t, dir, "vlc-override.exe")

	got, err := vlc.Detect([]string{absent}, override)
	require.NoError(t, err)
	assert.Equal(t, override, got)
}

func TestDetect_ReturnsError_WhenNothingFound(t *testing.T) {
	dir := t.TempDir()
	absent := filepath.Join(dir, "absent.exe")

	_, err := vlc.Detect([]string{absent}, "")
	assert.True(t, errors.Is(err, vlc.ErrNotFound))
}

func TestDetect_IgnoresAbsentConfigOverride(t *testing.T) {
	dir := t.TempDir()
	absent := filepath.Join(dir, "absent.exe")
	absentOverride := filepath.Join(dir, "also-absent.exe")

	_, err := vlc.Detect([]string{absent}, absentOverride)
	assert.True(t, errors.Is(err, vlc.ErrNotFound))
}
