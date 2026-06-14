// Command bumpbuild increments a monotonic build counter stored in the
// gitignored file .build-number and prints the new value on stdout. It is
// invoked by the `task build` target to inject a build number via -ldflags.
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

const buildNumberFile = ".build-number"

// nextBuildNumber parses prev as an integer and returns the next value as a
// string. Any parse error or a negative value is treated as 0, so the first
// build (or a missing/corrupt file) yields "1".
func nextBuildNumber(prev string) string {
	n, err := strconv.Atoi(strings.TrimSpace(prev))
	if err != nil || n < 0 {
		n = 0
	}
	return strconv.Itoa(n + 1)
}

func main() {
	// A missing file is not fatal: treat it as an empty (zero) counter.
	prev, err := os.ReadFile(buildNumberFile)
	if err != nil {
		prev = nil
	}

	next := nextBuildNumber(string(prev))

	if err := os.WriteFile(buildNumberFile, []byte(next), 0644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Print(next)
}
