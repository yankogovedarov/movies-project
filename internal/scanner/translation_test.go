package scanner

import "testing"

func TestDetectTranslation(t *testing.T) {
	tests := []struct {
		name          string
		filename      string
		folderRel     string
		subtitleBases map[string]bool
		want          string
	}{
		{"bg marker in filename", "Movie.BGAUDIO.mkv", "Action", nil, "bg"},
		{"bg dotted marker in filename", "Movie.BG.AUDIO.mkv", "Action", nil, "bg"},
		{"bg marker in folder", "Movie.mkv", "01_New\\Filmi BGAUDIO", nil, "bg"},
		{"bg marker case insensitive", "movie.bgaudio.mkv", "x", nil, "bg"},
		{"sub exact base match", "Movie.mkv", "Action", map[string]bool{"movie": true}, "sub"},
		{"sub prefix base match", "Movie.mkv", "Action", map[string]bool{"movie.bg": true}, "sub"},
		{"bg takes priority over sub", "Movie.BGAUDIO.mkv", "Action", map[string]bool{"movie.bgaudio": true}, "bg"},
		{"none — unrelated subtitle", "Movie.mkv", "Action", map[string]bool{"other": true}, ""},
		{"none — no subtitles", "Movie.mkv", "Action", nil, ""},
		{"none — subtitle is substring but not prefix.dot", "Movie.mkv", "Action", map[string]bool{"moviex": true}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectTranslation(tt.filename, tt.folderRel, tt.subtitleBases)
			if got != tt.want {
				t.Errorf("detectTranslation(%q, %q, %v) = %q, want %q",
					tt.filename, tt.folderRel, tt.subtitleBases, got, tt.want)
			}
		})
	}
}
