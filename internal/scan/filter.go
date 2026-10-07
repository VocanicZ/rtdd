package scan

import (
	"bytes"
	"io"
	"os"
	"path/filepath"

	"github.com/VocanicZ/rtdd/internal/paths"
)

// MaxFileSize is the largest file the scanner reads (spec §4.1).
const MaxFileSize = 1 << 20

// sniffLen is how much of a file is searched for a NUL byte (spec §4.1).
const sniffLen = 8 << 10

// Filter keeps the files the scanner reads: not matching exclude, at most MaxFileSize
// bytes, no NUL byte in the first 8 KiB (spec §4.1). files are repo-relative and
// slash-separated, as gitctx.ListFiles returns them; a file that cannot be read is
// skipped, never an error — it is not a definition the graph can hold.
func Filter(root string, files, exclude []string) []string {
	var out []string
	for _, rel := range files {
		if matchAny(exclude, rel) {
			continue
		}
		abs := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Stat(abs)
		if err != nil || !info.Mode().IsRegular() || info.Size() > MaxFileSize {
			continue
		}
		f, err := os.Open(abs)
		if err != nil {
			continue
		}
		head := make([]byte, sniffLen)
		n, _ := io.ReadFull(f, head)
		f.Close()
		if bytes.IndexByte(head[:n], 0) >= 0 {
			continue
		}
		out = append(out, rel)
	}
	return out
}

func matchAny(globs []string, rel string) bool {
	for _, g := range globs {
		if paths.MatchGlob(g, rel) {
			return true
		}
	}
	return false
}
