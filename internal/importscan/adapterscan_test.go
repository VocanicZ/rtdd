package importscan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
)

func scannerRepo(t *testing.T, script string) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".rtdd", "adapters")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if script != "" {
		src, err := os.ReadFile(filepath.Join("testdata", script))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, script), src, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func scanAdapter(script string) *adapter.Adapter {
	return &adapter.Adapter{
		Name:       "typescript",
		Selection:  adapter.SelectionStatic,
		Coverage:   adapter.CoverageNone,
		Importscan: &adapter.Importscan{Command: "bash {script}", Script: script},
	}
}

func TestAdapterScannerReturnsHopCounts(t *testing.T) {
	root := scannerRepo(t, "scan-imports-stub.sh")
	s := NewAdapterScanner(root, scanAdapter("scan-imports-stub.sh"),
		[]string{"src/auth/session.test.ts", "src/api/gateway.test.ts"})

	got := s.Distances("src/auth/token.ts")

	if s.Err() != nil {
		t.Fatalf("Err() = %v, want nil", s.Err())
	}
	if got["src/auth/session.test.ts"] != 1 || got["src/api/gateway.test.ts"] != 3 {
		t.Errorf("Distances = %#v, want session:1 gateway:3", got)
	}
}

// A missing script degrades selection to levels 1 and 3. It must not fail the command:
// spec §4.2 makes importscan optional, and a broken one is no worse than an absent one.
func TestAdapterScannerDegradesWhenTheScriptIsMissing(t *testing.T) {
	root := scannerRepo(t, "")
	s := NewAdapterScanner(root, scanAdapter("scan-imports-stub.sh"), []string{"a.test.ts"})

	if got := s.Distances("src/auth/token.ts"); got != nil {
		t.Errorf("Distances = %#v, want nil", got)
	}
	if s.Err() == nil {
		t.Error("Err() = nil, want the failure recorded for the caller to report")
	}
}

// An adapter that declares no scanner is the ordinary case, not an error.
func TestAdapterScannerWithNoImportscanIsInert(t *testing.T) {
	s := NewAdapterScanner(t.TempDir(), &adapter.Adapter{Name: "python"}, nil)
	if got := s.Distances("src/a.py"); got != nil {
		t.Errorf("Distances = %#v, want nil", got)
	}
	if s.Err() != nil {
		t.Errorf("Err() = %v, want nil: an absent scanner is not a failure", s.Err())
	}
}

// One scanner, one child process: Distances is memoised across repeated lookups within a
// command invocation, and a second target answered by the same run costs nothing more.
func TestAdapterScannerRunsTheScriptOncePerTarget(t *testing.T) {
	root := scannerRepo(t, "scan-imports-stub.sh")
	s := NewAdapterScanner(root, scanAdapter("scan-imports-stub.sh"), nil)
	runs := 0
	inner := s.scan
	s.scan = func(target string) (map[string]int, error) {
		runs++
		return inner(target)
	}

	s.Distances("src/auth/token.ts")
	s.Distances("src/auth/token.ts")

	if runs != 1 {
		t.Errorf("scan ran %d times for one target, want 1", runs)
	}
}
