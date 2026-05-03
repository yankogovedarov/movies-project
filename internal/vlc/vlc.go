package vlc

import (
	"errors"
	"os"
)

var ErrNotFound = errors.New("VLC not found; set vlc_path in config.toml")

var DefaultPaths = []string{
	`C:\Program Files\VideoLAN\VLC\vlc.exe`,
	`C:\Program Files (x86)\VideoLAN\VLC\vlc.exe`,
}

func Detect(candidates []string, configOverride string) (string, error) {
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if configOverride != "" {
		if _, err := os.Stat(configOverride); err == nil {
			return configOverride, nil
		}
	}
	return "", ErrNotFound
}

func DetectDefault(configOverride string) (string, error) {
	return Detect(DefaultPaths, configOverride)
}
