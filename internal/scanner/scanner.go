package scanner

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type VideoFile struct {
	Path               string
	Filename           string
	SizeBytes          int64
	FolderRelativePath string
	FileCreatedAt      time.Time
	TranslationType    string // detected hint: "bg" | "sub" | "" (unset)
}

func Scan(root string, excludeDirs []string) ([]VideoFile, error) {
	var files []VideoFile
	videoExts := map[string]bool{".mkv": true, ".avi": true, ".mp4": true}
	excludeMap := make(map[string]bool)
	for _, dir := range excludeDirs {
		excludeMap[dir] = true
	}

	// subtitleBases[folderRelPath] = set of subtitle base names (lower-cased, no ext)
	// found in that folder. Collected during the walk so SUB detection works even
	// when a subtitle is visited after its video file.
	subtitleBases := make(map[string]map[string]bool)

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

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			relPath = path
		}
		folderRelPath := filepath.Dir(relPath)
		if folderRelPath == "." {
			folderRelPath = ""
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))

		if isSubtitleExt(ext) {
			if subtitleBases[folderRelPath] == nil {
				subtitleBases[folderRelPath] = make(map[string]bool)
			}
			subtitleBases[folderRelPath][baseName(d.Name())] = true
			return nil
		}

		if !videoExts[ext] {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		files = append(files, VideoFile{
			Path:               path,
			Filename:           d.Name(),
			SizeBytes:          info.Size(),
			FolderRelativePath: folderRelPath,
			FileCreatedAt:      fileCreatedAt(path),
		})
		return nil
	})

	// Second pass: now that all subtitle files are indexed, compute the
	// translation-type hint for each video file.
	for i := range files {
		files[i].TranslationType = detectTranslation(
			files[i].Filename,
			files[i].FolderRelativePath,
			subtitleBases[files[i].FolderRelativePath],
		)
	}

	return FilterSamples(files), err
}
