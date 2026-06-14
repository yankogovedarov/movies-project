package main

import "testing"

func TestNextBuildNumber(t *testing.T) {
	tests := []struct {
		name string
		prev string
		want string
	}{
		{name: "zero", prev: "0", want: "1"},
		{name: "empty", prev: "", want: "1"},
		{name: "trimmed", prev: "  7  ", want: "8"},
		{name: "invalid", prev: "abc", want: "1"},
		{name: "negative", prev: "-3", want: "1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nextBuildNumber(tt.prev)
			if got != tt.want {
				t.Errorf("nextBuildNumber(%q) = %q, want %q", tt.prev, got, tt.want)
			}
		})
	}
}
