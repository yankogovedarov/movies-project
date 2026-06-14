package scanner

import (
	"path/filepath"
	"strings"
)

// subtitleExts are the file extensions recognized as subtitle files for SUB detection.
var subtitleExts = map[string]bool{
	".srt": true,
	".sub": true,
	".ass": true,
	".ssa": true,
	".idx": true,
	".vtt": true,
}

// isSubtitleExt reports whether ext (with leading dot) is a recognized subtitle extension.
func isSubtitleExt(ext string) bool {
	return subtitleExts[strings.ToLower(ext)]
}

// hasBGAudio reports whether name contains a Bulgarian-audio marker
// ("BGAUDIO" or "BG.AUDIO"), case-insensitive.
func hasBGAudio(name string) bool {
	lower := strings.ToLower(name)
	return strings.Contains(lower, "bgaudio") || strings.Contains(lower, "bg.audio")
}

// baseName returns the filename without its extension, lower-cased.
func baseName(filename string) string {
	return strings.ToLower(strings.TrimSuffix(filename, filepath.Ext(filename)))
}

// detectTranslation determines the translation-type hint for a video file.
//
// BG (Bulgarian audio) takes priority over SUB (sibling subtitle file): a file
// whose name or folder contains a BGAUDIO marker is "bg" even if a subtitle file
// also sits next to it.
//
// subtitleBases is the set of subtitle base names (extension stripped, lower-cased)
// present in the same folder. A SUB match requires a subtitle whose base name
// equals the video's base name, or starts with it followed by a dot
// (e.g. "movie.bg" for "Movie.mkv").
//
// Returns "bg", "sub", or "" (unset) when no signal is found.
func detectTranslation(filename, folderRel string, subtitleBases map[string]bool) string {
	if hasBGAudio(filename) || hasBGAudio(folderRel) {
		return "bg"
	}
	base := baseName(filename)
	for sb := range subtitleBases {
		if sb == base || strings.HasPrefix(sb, base+".") {
			return "sub"
		}
	}
	return ""
}
