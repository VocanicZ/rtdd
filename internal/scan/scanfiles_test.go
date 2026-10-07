package scan

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// ScanFiles is ScanFile over each readable file, in the order given, whatever it runs on.
func TestScanFilesKeepsInputOrderAndSkipsUnreadableFiles(t *testing.T) {
	root := t.TempDir()
	var files []string
	var want []FileResult
	for i := 0; i < 200; i++ {
		rel := fmt.Sprintf("d%d/f%03d.py", i%7, 199-i)
		files = append(files, rel)
		if i%10 == 3 {
			continue // never written: unreadable, so skipped
		}
		src := []byte(fmt.Sprintf("def f%d(x):\n    return g%d(x)\n", i, i))
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, src, 0o644); err != nil {
			t.Fatal(err)
		}
		want = append(want, ScanFile(rel, src))
	}
	if got := ScanFiles(root, files); !reflect.DeepEqual(got, want) {
		t.Errorf("ScanFiles returned %d results, want %d in input order", len(got), len(want))
	}
}
