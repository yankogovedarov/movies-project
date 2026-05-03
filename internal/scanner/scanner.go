package scanner

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type VideoFile struct {
	Path                string
	Filename            string
	SizeBytes           int64
	FolderRelativePath  string
}

func Scan(root string, excludeDirs []string) ([]VideoFile, error) {
	var files []VideoFile
	videoExts := map[string]bool{".mkv": true, ".avi": true, ".mp4": true}
	excludeMap := make(map[string]bool)
	for _, dir := range excludeDirs {
		excludeMap[dir] = true
	}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Skip files/dirs we can't access (permission denied, etc)
			if os.IsPermission(err) {
				return nil
			}
			// For other errors, stop walking
			return err
		}
		if d.IsDir() {
			if excludeMap[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))
		if !videoExts[ext] {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			relPath = path
		}
		folderRelPath := filepath.Dir(relPath)
		if folderRelPath == "." {
			folderRelPath = ""
		}

		files = append(files, VideoFile{
			Path:               path,
			Filename:           d.Name(),
			SizeBytes:          info.Size(),
			FolderRelativePath: folderRelPath,
		})
		return nil
	})

	return FilterSamples(files), err
}
