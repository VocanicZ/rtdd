// Package paths is the single boundary where operating-system paths become
// repo-relative, slash-separated, cleaned paths. Nothing else in the engine may
// convert a path ad hoc.
package paths

import (
	"path/filepath"
	"strings"
)

// Normalize converts an absolute or runner-relative path to a cleaned,
// slash-separated path relative to repoRoot. Returns ok=false if p escapes repoRoot.
func Normalize(repoRoot, p string) (rel string, ok bool) {
	if p == "" {
		return "", false
	}
	root := filepath.Clean(repoRoot)
	q := filepath.FromSlash(p)
	if !filepath.IsAbs(q) {
		q = filepath.Join(root, q)
	}
	r, err := filepath.Rel(root, filepath.Clean(q))
	if err != nil {
		return "", false
	}
	r = filepath.ToSlash(r)
	if r == "." || r == ".." || strings.HasPrefix(r, "../") {
		return "", false
	}
	return r, true
}

// StripModulePrefix removes a Go module path prefix. Unused in M1; present so the
// Go adapter (deferred) has a defined home.
func StripModulePrefix(modulePath, p string) string {
	if modulePath == "" {
		return p
	}
	prefix := strings.TrimSuffix(modulePath, "/") + "/"
	if strings.HasPrefix(p, prefix) {
		return strings.TrimPrefix(p, prefix)
	}
	return p
}
