package adapter

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
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
	if len(got) != 1 || got[0].Name != "python" {
		t.Fatalf("Detect = %v, want [python]", names(got))
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
	if len(got) != 1 || got[0].Name != "golang" {
		t.Fatalf("Detect = %v, want [golang]", names(got))
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
	// The caller stands in one directory and RTDD walked another often enough that the
	// refusal is only actionable when it says which tree it searched.
	if !strings.Contains(err.Error(), dir) {
		t.Errorf("Detect error = %q, want it to name the repo root %q", err.Error(), dir)
	}
}

// This test used to assert the opposite: two matches was refused as "polyglot repos are
// out of scope in v1". Spec §4.4 makes a TypeScript service with a Python tooling
// directory ordinary, so the subject is unchanged — a repo two adapters match — and the
// expected outcome is what moved. The refusal returned a message instead of a selection,
// which left such a repo on the null baseline of BOTH toolchains rather than a narrowed
// suite from each.
func TestDetectReturnsEveryMatchingAdapter(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, dir, "pyproject.toml", "[project]\n")
	writeRepoFile(t, dir, "package.json", "{}\n")
	as := []*Adapter{
		mkAdapter("python", "pyproject.toml"),
		mkAdapter("js", "package.json"),
		mkAdapter("rspec", ".rspec"),
	}

	got, err := Detect(dir, as)
	if err != nil {
		t.Fatalf("Detect: %v, want no error for a polyglot repo", err)
	}
	if len(got) != 2 || got[0].Name != "python" || got[1].Name != "js" {
		t.Fatalf("Detect = %v, want [python js]", names(got))
	}
}

// Order is the order the adapters were given (adapter.Available sorts them by name), not
// the order the walk happened to reach their markers in. A set whose order moved between
// two calls would move every per-adapter row and selection block downstream of it.
func TestDetectOrderIsTheAdapterOrderAndIsStable(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, dir, "zeta/pyproject.toml", "[project]\n")
	writeRepoFile(t, dir, "alpha/package.json", "{}\n")
	as := []*Adapter{
		mkAdapter("python", "**/pyproject.toml"),
		mkAdapter("js", "**/package.json"),
	}

	first, err := Detect(dir, as)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	second, err := Detect(dir, as)
	if err != nil {
		t.Fatalf("Detect (second call): %v", err)
	}
	want := []string{"python", "js"}
	if got := names(first); !slices.Equal(got, want) {
		t.Fatalf("Detect = %v, want %v — declaration order, not walk order", got, want)
	}
	if got := names(second); !slices.Equal(got, want) {
		t.Fatalf("second Detect = %v, want %v — the order must reproduce", got, want)
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
		if len(got) != 1 || got[0].Name != "python" {
			t.Fatalf("Detect(%q) = %v, want [python]", root, names(got))
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
	if len(got) != 1 || got[0].Name != "python" {
		t.Fatalf("Detect = %v, want [python]", names(got))
	}
}

// names renders a detected set the way every failure message here wants to read it.
func names(as []*Adapter) []string {
	out := make([]string, 0, len(as))
	for _, a := range as {
		out = append(out, a.Name)
	}
	return out
}

// repoRootForTest walks up from the test's working directory to the directory holding
// go.mod, so a tree-wide assertion runs against the whole repository rather than this
// package.
func repoRootForTest(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod found above %s", dir)
		}
		dir = parent
	}
}

// PRD #232 AC5: the v1 refusal is gone from the tree, not merely unreachable. A dead
// string is one copied error message away from being live again, and it now contradicts
// spec §4.4.
//
// The banned text is assembled from halves on purpose: written as one literal it would
// live in this file and the assertion would find itself.
func TestV1PolyglotErrorStringIsDeletedFromTheTree(t *testing.T) {
	banned := "polyglot repos are out of " + "scope in v1"
	root := repoRootForTest(t)
	var hits []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(p) != ".go" {
			return nil
		}
		b, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		if strings.Contains(string(b), banned) {
			rel, relErr := filepath.Rel(root, p)
			if relErr != nil {
				rel = p
			}
			hits = append(hits, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(hits) != 0 {
		t.Errorf("the v1 string %q survives in %v; spec §4.4 makes polyglot repos supported", banned, hits)
	}
}
