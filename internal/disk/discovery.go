package disk

import (
	"path/filepath"
)

type AppPaths struct {
	AppFolder string
	DiskRoot  string
}

func Discover(executableFn func() (string, error)) (AppPaths, error) {
	exe, err := executableFn()
	if err != nil {
		return AppPaths{}, err
	}
	appFolder := filepath.Dir(exe) + string(filepath.Separator)
	diskRoot := filepath.VolumeName(exe) + string(filepath.Separator)
	return AppPaths{
		AppFolder: appFolder,
		DiskRoot:  diskRoot,
	}, nil
}
