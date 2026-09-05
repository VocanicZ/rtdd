package adapter

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// writeHostAdapter drops one YAML file into <root>/.rtdd/adapters/.
func writeHostAdapter(t *testing.T, root, name, body string) {
	t.Helper()
	dir := filepath.Join(root, ".rtdd", "adapters")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// hostPythonYAML is a host override of the shipped python adapter, distinguishable from
// the built-in by the `-p no:randomly` its subset carries.
const hostPythonYAML = `name: python
detect: ["pyproject.toml"]
seed: "pytest --cov --cov-context=test"
subset: "pytest {tests} --cov --cov-context=test -p no:randomly"
list: "pytest --collect-only -q"
coverage: sqlite
report: pytest-reportlog
test_globs: ["tests/**/*.py"]
source_globs: ["**/*.py"]
`

// brokenYAML declares a subset with no {tests} placeholder: a real contract violation
// that names a field, not a YAML syntax error.
const brokenYAML = `name: broken
detect: ["go.mod"]
seed: "go test ./..."
subset: "go test ./..."
coverage: sqlite
report: pytest-reportlog
`

// The whole point of §4.5: a language the engine has never heard of, supported by YAML.
func TestAvailableIncludesHostAdapters(t *testing.T) {
	root := t.TempDir()
	writeHostAdapter(t, root, "vitest.yaml", staticYAML)

	all, err := Available(root)
	if err != nil {
		t.Fatalf("Available: %v", err)
	}
	if byName(all, "python") == nil {
		t.Errorf("Available lost the built-in python adapter")
	}
	vitest := byName(all, "vitest")
	if vitest == nil {
		t.Fatalf("Available did not load .rtdd/adapters/vitest.yaml; got %v", all)
	}
	if !strings.HasSuffix(filepath.ToSlash(vitest.Src), ".rtdd/adapters/vitest.yaml") {
		t.Errorf("host adapter Src = %q, want the host path", vitest.Src)
	}
}

// The precedence rule this slice pins: the host wins, so a user can fix a shipped adapter
// for their own repo without waiting for a release.
func TestHostAdapterOverridesTheBuiltinOfTheSameName(t *testing.T) {
	root := t.TempDir()
	writeHostAdapter(t, root, "python.yaml", hostPythonYAML)

	all, err := Available(root)
	if err != nil {
		t.Fatalf("Available: %v", err)
	}
	n := 0
	var py *Adapter
	for _, a := range all {
		if a.Name == "python" {
			n++
			py = a
		}
	}
	if n != 1 {
		t.Fatalf("Available returned %d adapters named python, want exactly 1", n)
	}
	if !strings.Contains(py.Subset, "-p no:randomly") {
		t.Errorf("built-in python won over the host adapter: Subset = %q", py.Subset)
	}
}

// Two host files claiming one name have no principled winner, so it is exit 2 and the
// message names both paths.
func TestDuplicateHostAdapterNamesAreAnError(t *testing.T) {
	root := t.TempDir()
	writeHostAdapter(t, root, "a-vitest.yaml", staticYAML)
	writeHostAdapter(t, root, "z-vitest.yaml", staticYAML)

	_, err := Available(root)
	if err == nil {
		t.Fatalf("Available accepted two host adapters named vitest")
	}
	for _, want := range []string{"a-vitest.yaml", "z-vitest.yaml", "vitest"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to name %q", err.Error(), want)
		}
	}
}

// Every repo that exists today has no .rtdd/adapters/. That is the normal case, not a
// failure, and it must return exactly the built-ins.
func TestAvailableWithNoHostDirectoryReturnsTheBuiltins(t *testing.T) {
	got, err := Available(t.TempDir())
	if err != nil {
		t.Fatalf("Available: %v", err)
	}
	want, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("Available = %d adapters, want the %d built-ins", len(got), len(want))
	}
	for i := range got {
		if got[i].Name != want[i].Name {
			t.Errorf("Available[%d] = %q, want %q", i, got[i].Name, want[i].Name)
		}
	}
}

// doctor must be able to report a broken host adapter instead of dying on it, and it must
// still see the ones that are fine.
func TestLoadHostReportNamesTheInvalidFileAndKeepsTheRest(t *testing.T) {
	root := t.TempDir()
	writeHostAdapter(t, root, "vitest.yaml", staticYAML)
	writeHostAdapter(t, root, "broken.yaml", brokenYAML)

	ok, bad, err := LoadHostReport(root)
	if err != nil {
		t.Fatalf("LoadHostReport: %v", err)
	}
	if len(ok) != 1 || ok[0].Name != "vitest" {
		t.Errorf("valid adapters = %v, want just vitest", ok)
	}
	if len(bad) != 1 {
		t.Fatalf("invalid adapters = %v, want exactly one", bad)
	}
	if !strings.HasSuffix(filepath.ToSlash(bad[0].Path), ".rtdd/adapters/broken.yaml") {
		t.Errorf("invalid path = %q, want .rtdd/adapters/broken.yaml", bad[0].Path)
	}
	if !strings.Contains(bad[0].Err.Error(), "{tests}") {
		t.Errorf("invalid err = %q, want it to name the failing field", bad[0].Err)
	}
}

// Partial failure, the second behaviour the issue puts on this slice: a repo where
// someone is halfway through authoring an adapter still runs on the ones that are valid.
func TestAvailableSkipsAMalformedHostAdapterAndKeepsTheRest(t *testing.T) {
	root := t.TempDir()
	writeHostAdapter(t, root, "vitest.yaml", staticYAML)
	writeHostAdapter(t, root, "broken.yaml", brokenYAML)

	all, err := Available(root)
	if err != nil {
		t.Fatalf("Available: %v", err)
	}
	if byName(all, "vitest") == nil {
		t.Errorf("a malformed sibling took the valid host adapter down with it: %v", all)
	}
	if byName(all, "python") == nil {
		t.Errorf("a malformed host adapter took the built-ins down with it: %v", all)
	}
	if byName(all, "broken") != nil {
		t.Errorf("the malformed adapter was resolved anyway: %v", all)
	}
}

// AvailableReport is Available for callers that must say WHY a host adapter is missing.
func TestAvailableReportCarriesTheInvalidFilesAlongsideTheResolvedSet(t *testing.T) {
	root := t.TempDir()
	writeHostAdapter(t, root, "vitest.yaml", staticYAML)
	writeHostAdapter(t, root, "broken.yaml", brokenYAML)

	all, bad, err := AvailableReport(root)
	if err != nil {
		t.Fatalf("AvailableReport: %v", err)
	}
	if byName(all, "vitest") == nil {
		t.Errorf("AvailableReport lost the valid host adapter: %v", all)
	}
	if len(bad) != 1 || !strings.Contains(bad[0].Err.Error(), "{tests}") {
		t.Fatalf("invalid = %v, want one entry naming the failing field", bad)
	}
}

// One reader: a host adapter is held to exactly the contract the built-ins are, including
// everything contract v2 added. The message is the one Load would give a built-in.
func TestHostAdaptersAreValidatedByTheContractV2Rules(t *testing.T) {
	root := t.TempDir()
	// selection: static forbids seed — a contract v2 rule, not a v1 required-field check.
	writeHostAdapter(t, root, "vitest.yaml", strings.Replace(staticYAML,
		"selection: static", "selection: static\nseed: \"npx vitest run\"", 1))

	_, bad, err := LoadHostReport(root)
	if err != nil {
		t.Fatalf("LoadHostReport: %v", err)
	}
	if len(bad) != 1 {
		t.Fatalf("invalid = %v, want the v2 rule to reject the host adapter", bad)
	}
	if !strings.Contains(bad[0].Err.Error(), "selection: static forbids seed") {
		t.Errorf("err = %q, want the same message a built-in would get", bad[0].Err)
	}
}

// Resolution order is deterministic, so repeated runs on the same repo produce the same
// adapter set in the same order.
func TestAvailableIsDeterministic(t *testing.T) {
	root := t.TempDir()
	writeHostAdapter(t, root, "vitest.yaml", staticYAML)
	writeHostAdapter(t, root, "python.yaml", hostPythonYAML)

	var first []string
	for i := 0; i < 5; i++ {
		all, err := Available(root)
		if err != nil {
			t.Fatalf("Available: %v", err)
		}
		names := make([]string, 0, len(all))
		for _, a := range all {
			names = append(names, a.Name)
		}
		if i == 0 {
			first = names
			continue
		}
		if !reflect.DeepEqual(names, first) {
			t.Fatalf("Available order run %d = %v, want %v", i, names, first)
		}
	}
	if !reflect.DeepEqual(first, []string{"python", "vitest"}) {
		t.Errorf("Available = %v, want the set sorted by name", first)
	}
}

// A host adapter is available everywhere a built-in is, Detect included: without this the
// override rule would be decorative.
func TestDetectResolvesAHostAuthoredAdapter(t *testing.T) {
	root := t.TempDir()
	writeHostAdapter(t, root, "vitest.yaml", staticYAML)
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatalf("write package.json: %v", err)
	}

	all, err := Available(root)
	if err != nil {
		t.Fatalf("Available: %v", err)
	}
	ad, err := Detect(root, all)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if ad.Name != "vitest" {
		t.Errorf("Detect = %q, want the host-authored vitest adapter", ad.Name)
	}
}

// Src is how doctor and every error message name the file an adapter came from.
func TestLoadRecordsTheSourcePath(t *testing.T) {
	dir := t.TempDir()
	p := writeAdapter(t, dir, "demo.yaml", validYAML)
	a, err := Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if a.Src != p {
		t.Errorf("Src = %q, want %q", a.Src, p)
	}
}

// An embedded adapter has a source too, and it is not a path on the host's disk.
func TestBuiltinAdaptersRecordAnEmbeddedSource(t *testing.T) {
	all, err := Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	py := byName(all, "python")
	if py == nil {
		t.Fatalf("Builtin has no python adapter")
	}
	if py.Src != "python.yaml" {
		t.Errorf("built-in Src = %q, want %q", py.Src, "python.yaml")
	}
}

// LoadAll's Src must be the on-disk path, not the fs-relative name os.DirFS produces.
func TestLoadAllRecordsTheOnDiskPath(t *testing.T) {
	dir := t.TempDir()
	writeAdapter(t, dir, "demo.yaml", validYAML)
	all, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("LoadAll = %d adapters, want 1", len(all))
	}
	if want := filepath.Join(dir, "demo.yaml"); all[0].Src != want {
		t.Errorf("Src = %q, want %q", all[0].Src, want)
	}
}

// `src` is not a declarable key: it is where the adapter came from, and a YAML file that
// claims its own provenance would be lying to doctor.
func TestSrcIsNotADeclarableKey(t *testing.T) {
	_, err := Load(writeAdapter(t, t.TempDir(), "demo.yaml", validYAML+"src: elsewhere.yaml\n"))
	if err == nil {
		t.Fatal("Load accepted a declared src key")
	}
	if !strings.Contains(err.Error(), "src") {
		t.Errorf("err = %q, want it to name the unknown field", err)
	}
}
