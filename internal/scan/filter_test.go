package scan

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

func write(t *testing.T, root, rel string, b []byte) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// Spec §4.1: binary, oversized and scan_exclude'd files are never read.
func TestFilterDropsBinaryOversizedAndEveryDefaultExclude(t *testing.T) {
	root := t.TempDir()
	src := []byte("def f():\n    return 1\n")
	files := []string{
		"app/keep.py",
		"vendor/x.py", "node_modules/x.js", "third_party/x.c", "web/app.min.js",
		"dist/x.js", "build/x.py", ".rtdd/graph.json", "graphify-out/graph.json",
		"app/nul.py", "app/late_nul.py", "app/big.py", "app/exactly_1mib.py",
	}
	for _, f := range files {
		write(t, root, f, src)
	}
	write(t, root, "app/nul.py", append([]byte("def f():\x00"), src...))
	write(t, root, "app/late_nul.py", append([]byte(strings.Repeat("#\n", 4097)), 0)) // NUL past 8 KiB: kept
	write(t, root, "app/big.py", []byte(strings.Repeat("x", MaxFileSize+1)))
	write(t, root, "app/exactly_1mib.py", []byte(strings.Repeat("x", MaxFileSize)))

	got := Filter(root, files, graph.DefaultConfig().ScanExclude)
	want := []string{"app/keep.py", "app/late_nul.py", "app/exactly_1mib.py"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Filter = %v, want %v", got, want)
	}
}
