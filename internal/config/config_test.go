package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yankogovedarov/movie-tracker/internal/config"
)

func TestLoad_ReturnsDefaultConfig_WhenFileAbsent(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "", cfg.VLCPath)
}

func TestLoad_ParsesVLCPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LOCALAPPDATA", dir)

	cfgDir := filepath.Join(dir, "MovieTracker")
	require.NoError(t, os.MkdirAll(cfgDir, 0o755))
	require.NoError(t, os.WriteFile(
		filepath.Join(cfgDir, "config.toml"),
		[]byte(`vlc_path = "C:\\Program Files\\VideoLAN\\VLC\\vlc.exe"`),
		0o644,
	))

	cfg, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, `C:\Program Files\VideoLAN\VLC\vlc.exe`, cfg.VLCPath)
}
