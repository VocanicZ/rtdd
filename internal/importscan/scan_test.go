package importscan

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func requirePython(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("python3"); err != nil {
		if _, err2 := exec.LookPath("python"); err2 != nil {
			t.Skip("no python interpreter on PATH")
		}
	}
}

func write(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// cycleFixture is fixture F3, verified on 2026-08-26: src/a.py and src/b.py import
// each other, so any traversal without a visited set hangs.
func cycleFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write(t, root, "src/__init__.py", "")
	write(t, root, "tests/__init__.py", "")
	write(t, root, "src/constants.py", "MAX = 3\n")
	write(t, root, "src/a.py", "from src.constants import MAX\nimport src.b\n")
	write(t, root, "src/b.py", "import src.a\n")
	write(t, root, "tests/test_direct.py", "from src.constants import MAX\ndef test_d(): assert MAX == 3\n")
	write(t, root, "tests/test_trans.py", "from src import a\ndef test_t(): assert a.MAX == 3\n")
	write(t, root, "tests/test_unrelated.py", "def test_u(): assert True\n")
	return root
}

func allTests() []string {
	return []string{"tests/test_direct.py", "tests/test_trans.py", "tests/test_unrelated.py"}
}

func TestScanDirectAndTransitive(t *testing.T) {
	requirePython(t)
	root := cycleFixture(t)
	got, err := Scan(root, []string{"src/constants.py"}, allTests())
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	want := map[string][]string{
		"src/constants.py": {"tests/test_direct.py", "tests/test_trans.py"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan()\n got: %#v\nwant: %#v", got, want)
	}
}

func TestScanTerminatesOnImportCycle(t *testing.T) {
	requirePython(t)
	root := cycleFixture(t)
	// src/b.py is reachable only by walking into the a <-> b cycle.
	got, err := Scan(root, []string{"src/b.py", "src/nonexistent.py"}, allTests())
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	want := map[string][]string{
		"src/b.py":           {"tests/test_trans.py"},
		"src/nonexistent.py": {},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan()\n got: %#v\nwant: %#v", got, want)
	}
}

func TestScanRelativeImports(t *testing.T) {
	requirePython(t)
	root := t.TempDir()
	write(t, root, "pkg/__init__.py", "")
	write(t, root, "pkg/models.py", "NAME = 'x'\n")
	write(t, root, "pkg/service.py", "from .models import NAME\n")
	write(t, root, "tests/__init__.py", "")
	write(t, root, "tests/test_svc.py", "from pkg.service import NAME\ndef test_s(): assert NAME == 'x'\n")
	got, err := Scan(root, []string{"pkg/models.py"}, []string{"tests/test_svc.py"})
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	want := map[string][]string{"pkg/models.py": {"tests/test_svc.py"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan()\n got: %#v\nwant: %#v", got, want)
	}
}

func TestScanSyntaxErrorIsNotFatal(t *testing.T) {
	requirePython(t)
	root := cycleFixture(t)
	write(t, root, "src/broken.py", "def (((\n")
	got, err := Scan(root, []string{"src/constants.py"}, allTests())
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	want := map[string][]string{
		"src/constants.py": {"tests/test_direct.py", "tests/test_trans.py"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Scan()\n got: %#v\nwant: %#v", got, want)
	}
}

func TestScanNoTargetsIsEmpty(t *testing.T) {
	requirePython(t)
	root := cycleFixture(t)
	got, err := Scan(root, nil, allTests())
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if got == nil {
		t.Fatal("Scan() returned a nil map; the map is never nil")
	}
	if len(got) != 0 {
		t.Fatalf("Scan() = %#v, want empty", got)
	}
}

// The scanner must travel inside the binary: spec D4 promises a single static binary with
// no runtime dependencies forced into the host repo, so scan.py cannot be read off disk
// beside the executable. Reading it out of the embedded bytes is what proves it is linked
// in rather than located at run time.
func TestScanScriptIsEmbedded(t *testing.T) {
	if len(scanScript) == 0 {
		t.Fatal("scanScript is empty: scan.py must be embedded with go:embed")
	}
	if !strings.Contains(string(scanScript), "ast.parse") {
		t.Errorf("embedded scan.py does not call ast.parse; the AST walk is the whole scan")
	}
	onDisk, err := os.ReadFile("scan.py")
	if err != nil {
		t.Fatalf("read scan.py: %v", err)
	}
	if string(onDisk) != string(scanScript) {
		t.Error("embedded scan.py does not match internal/importscan/scan.py")
	}
}
