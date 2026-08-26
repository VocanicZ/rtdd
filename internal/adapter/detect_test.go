package adapter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeRepoFile materialises a file (and any parent directories) under dir. Detection is
// the one part of this package that reads the filesystem, so its tests need real files.
func writeRepoFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func mkAdapter(name string, detect ...string) *Adapter {
	return &Adapter{
		Name: name, Detect: detect,
		Seed: "x", Subset: "{tests}", Coverage: "sqlite", Report: "pytest-reportlog",
	}
}

func TestDetectExactlyOne(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, dir, "pyproject.toml", "[project]\nname='x'\n")
	writeRepoFile(t, dir, "src/logic.py", "x = 1\n")

	py := mkAdapter("python", "pytest.ini", "pyproject.toml", "setup.cfg")
	got, err := Detect(dir, []*Adapter{py})
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if got.Name != "python" {
		t.Fatalf("Detect = %q, want python", got.Name)
	}
}

// AC: a detect glob such as **/go.mod matches a marker nested at sub/deep/go.mod.
func TestDetectGlobPatternMatchesANestedMarker(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, dir, "sub/deep/go.mod", "module x\n")
	got, err := Detect(dir, []*Adapter{mkAdapter("golang", "**/go.mod")})
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if got.Name != "golang" {
		t.Fatalf("Detect = %q, want golang", got.Name)
	}
}

func TestDetectZeroMatches(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, dir, "README.md", "hi\n")
	_, err := Detect(dir, []*Adapter{mkAdapter("python", "pyproject.toml")})
	if err == nil {
		t.Fatal("Detect with no matching marker = nil error, want error")
	}
	if !strings.Contains(err.Error(), "no adapter detected") {
		t.Fatalf("Detect error = %q, want it to contain %q", err.Error(), "no adapter detected")
	}
}

func TestDetectMultipleMatchesIsAnError(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, dir, "pyproject.toml", "[project]\n")
	writeRepoFile(t, dir, "package.json", "{}\n")
	_, err := Detect(dir, []*Adapter{
		mkAdapter("python", "pyproject.toml"),
		mkAdapter("js", "package.json"),
	})
	if err == nil {
		t.Fatal("Detect with two matching adapters = nil error, want error (polyglot is out of scope in v1)")
	}
	if !strings.Contains(err.Error(), "python") || !strings.Contains(err.Error(), "js") {
		t.Fatalf("Detect error = %q, want it to name both ambiguous adapters", err.Error())
	}
}

// A marker is a FILE. A directory that happens to carry the marker's name declares
// nothing about the toolchain.
func TestDetectIgnoresDirectoryNamedLikeAMarker(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, dir, "pyproject.toml/keep.txt", "a directory, not a marker file\n")
	_, err := Detect(dir, []*Adapter{mkAdapter("python", "pyproject.toml")})
	if err == nil {
		t.Fatal("Detect matched a directory named pyproject.toml, want no match")
	}
}

// A vendored dependency tree carries its own markers. Walking into one detects the
// dependency's toolchain, not the host repo's.
func TestDetectSkipsVendoredAndCacheDirectories(t *testing.T) {
	for _, skipped := range []string{".git", ".venv", "venv", "node_modules", "__pycache__", ".tox", ".mypy_cache", ".pytest_cache", ".rtdd"} {
		t.Run(skipped, func(t *testing.T) {
			dir := t.TempDir()
			writeRepoFile(t, dir, skipped+"/lib/pyproject.toml", "[project]\n")
			writeRepoFile(t, dir, "README.md", "hi\n")
			_, err := Detect(dir, []*Adapter{mkAdapter("python", "**/pyproject.toml")})
			if err == nil {
				t.Fatalf("Detect matched a marker inside %s/, want it skipped", skipped)
			}
			if !strings.Contains(err.Error(), "no adapter detected") {
				t.Fatalf("Detect error = %q, want %q", err.Error(), "no adapter detected")
			}
		})
	}
}

// The repo root arrives from the CLI's own discovery, which may hand back a relative or
// trailing-slash path. Normalisation is internal/paths' job, not each caller's.
func TestDetectAcceptsANonCanonicalRepoRoot(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, dir, "src/pkg/pyproject.toml", "[project]\n")
	for _, root := range []string{dir + string(filepath.Separator), filepath.Join(dir, "src", "..")} {
		got, err := Detect(root, []*Adapter{mkAdapter("python", "**/pyproject.toml")})
		if err != nil {
			t.Fatalf("Detect(%q): %v", root, err)
		}
		if got.Name != "python" {
			t.Fatalf("Detect(%q) = %q, want python", root, got.Name)
		}
	}
}

func TestDetectMissingRepoRootIsAnError(t *testing.T) {
	_, err := Detect(filepath.Join(t.TempDir(), "no-such-dir"), []*Adapter{mkAdapter("python", "pyproject.toml")})
	if err == nil {
		t.Fatal("Detect on a nonexistent repo root = nil error, want error")
	}
	if strings.Contains(err.Error(), "no adapter detected") {
		t.Fatalf("Detect error = %q; an unreadable repo root is not the same as a repo with no marker", err.Error())
	}
}

func TestDetectWithNoAdaptersIsAnError(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, dir, "pyproject.toml", "[project]\n")
	if _, err := Detect(dir, nil); err == nil {
		t.Fatal("Detect with an empty adapter list = nil error, want error")
	}
}

// The shipped adapter must detect a real Python repo shape end to end: this is the
// path `rtdd seed` takes before it can run anything at all.
func TestDetectFindsTheBuiltinPythonAdapter(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, dir, "pyproject.toml", "[project]\nname='x'\n")
	all, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	got, err := Detect(dir, all)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if got.Name != "python" {
		t.Fatalf("Detect = %q, want python", got.Name)
	}
}
