package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// writeFile writes a working-tree file without committing it. The file is left
// untracked or dirty on purpose: that is the state an agent's editor leaves behind.
func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	gittest.Write(t, dir, rel, content)
}

// gitRun runs git in dir and returns its stdout.
func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return gittest.Run(t, dir, args...)
}

// newTestRepo builds a real git repository containing the source tree the fixture map
// describes. Git is never mocked.
func newTestRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Init(t)

	gittest.Write(t, dir, "src/auth.py", "def login():\n    return 1\n")
	gittest.Write(t, dir, "src/db.py", "def query():\n    return 2\n")
	gittest.Write(t, dir, "src/render.py", "def page():\n    return 3\n")
	gittest.Write(t, dir, "templates/page.html", "<p>hi</p>\n")
	gittest.Write(t, dir, "tests/test_auth.py", "def test_logout():\n    pass\n")
	gittest.Write(t, dir, "tests/test_login.py", "def test_login():\n    pass\n")
	gittest.Write(t, dir, "tests/test_db.py", "def test_query():\n    pass\n")
	gittest.Write(t, dir, "tests/test_render.py", "def test_page():\n    pass\n")
	// The test repo ignores .rtdd/ so the fixture map and adapter never show up in the
	// changed set and every assertion is about the code under test. A real host repo
	// commits .rtdd/map.jsonl instead; either way it selects nothing.
	gittest.Write(t, dir, ".gitignore", ".rtdd/\n")
	gittest.Commit(t, dir, "init")
	return dir
}

func headShort(t *testing.T, dir string) string {
	t.Helper()
	return gittest.HeadShort(t, dir)
}

// installRTDD copies the fixture map, meta, and adapter into dir/.rtdd/, substituting
// seedSHA for the SEEDSHA token in the map and the meta.
func installRTDD(t *testing.T, dir, seedSHA string, cycles int) {
	t.Helper()
	raw, err := os.ReadFile("testdata/map.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	gittest.Write(t, dir, ".rtdd/map.jsonl", strings.ReplaceAll(string(raw), "SEEDSHA", seedSHA))

	ad, err := os.ReadFile("testdata/adapter.yaml")
	if err != nil {
		t.Fatal(err)
	}
	gittest.Write(t, dir, ".rtdd/adapter.yaml", string(ad))

	meta := `{"v":2,"adapter":"python","seeded_at":"` + seedSHA + `","cycles":` +
		strconv.Itoa(cycles) + "}\n"
	gittest.Write(t, dir, ".rtdd/meta.json", meta)
}

// rtdd runs the CLI with dir as the working directory.
func rtdd(t *testing.T, dir string, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	t.Chdir(dir)
	var out, errBuf bytes.Buffer
	code = run(args, &out, &errBuf)
	return code, out.String(), errBuf.String()
}

func TestRunWithNoArgsIsAUsageError(t *testing.T) {
	dir := newTestRepo(t)
	code, _, stderr := rtdd(t, dir)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "usage") {
		t.Errorf("stderr = %q, want usage text", stderr)
	}
}

func TestRunWithAnUnknownCommandIsAUsageError(t *testing.T) {
	dir := newTestRepo(t)
	code, _, stderr := rtdd(t, dir, "frobnicate")
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "frobnicate") {
		t.Errorf("stderr = %q, want it to name the unknown command", stderr)
	}
}

// An unknown flag is a usage error, not a crash and not a silent success.
func TestStatusWithAnUnknownFlagIsAUsageError(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)

	code, _, stderr := rtdd(t, dir, "status", "--frobnicate")
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if stderr == "" {
		t.Error("stderr is empty; a usage error must say what was wrong")
	}
}

// Exit code 1 means "a test failed". No M1a command executes a test, so no invocation of
// the CLI in this milestone may produce it — a 1 here would be an unrelated failure
// wearing the costume of a red test suite.
func TestNothingInM1aExitsOne(t *testing.T) {
	seeded := newTestRepo(t)
	installRTDD(t, seeded, headShort(t, seeded), 7)
	bare := newTestRepo(t)
	broken := newTestRepo(t)
	gittest.Write(t, broken, ".rtdd/map.jsonl", "this is not json\n")
	gittest.Write(t, broken, ".rtdd/adapter.yaml", ": : not: yaml: [\n")
	nonRepo := t.TempDir()

	cases := []struct {
		name string
		dir  string
		args []string
	}{
		{"no args", seeded, nil},
		{"help", seeded, []string{"--help"}},
		{"unknown command", seeded, []string{"frobnicate"}},
		{"status seeded", seeded, []string{"status"}},
		{"status unseeded", bare, []string{"status"}},
		{"status unknown flag", seeded, []string{"status", "--frobnicate"}},
		{"status malformed map", broken, []string{"status"}},
		{"status outside a repo", nonRepo, []string{"status"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, _, _ := rtdd(t, tc.dir, tc.args...)
			if code == 1 {
				t.Errorf("%v exited 1; nothing in M1a runs a test", tc.args)
			}
		})
	}
}

func TestWhichRejectsAnUnknownFlag(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)

	code, _, stderr := rtdd(t, dir, "which", "--nope")
	if code != 2 {
		t.Errorf("exit code = %d, want 2 (usage error)", code)
	}
	if stderr == "" {
		t.Error("stderr is empty")
	}
}

// `which --json` emits the frozen v1 document (schema §"The --json output schema"), the
// same one `run --json` emits, so an agent front-end binds once and reads both. The M1a
// which-only shape it replaced was declared PROVISIONAL in the code that carried it.
func TestWhichJSON(t *testing.T) {
	dir := newTestRepo(t)
	sha := headShort(t, dir)
	installRTDD(t, dir, sha, 3)
	writeFile(t, dir, "src/auth.py", "def login():\n    return 42\n")
	writeFile(t, dir, "tests/test_brand_new.py", "def test_new():\n    assert True\n")

	code, stdout, stderr := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	got := decodeOutput(t, stdout)

	if got.Schema != SchemaVersion {
		t.Errorf("schema = %d, want %d", got.Schema, SchemaVersion)
	}
	if got.Command != "which" {
		t.Errorf("command = %q, want which", got.Command)
	}
	if got.Tier != "T0" {
		t.Errorf("tier = %q, want T0 (reason: %s)", got.Tier, got.Reason)
	}
	if got.Base != "HEAD" {
		t.Errorf("base = %q, want HEAD", got.Base)
	}
	if len(got.Selection.Direct) != 1 || got.Selection.Direct[0] != "tests/test_brand_new.py" {
		t.Errorf("selection.direct = %#v, want [tests/test_brand_new.py]", got.Selection.Direct)
	}
	want := []string{
		"tests/test_brand_new.py",
		"tests/test_auth.py",
		"tests/test_login.py",
	}
	if !reflect.DeepEqual(got.Selection.Tests, want) {
		t.Errorf("selection.tests = %#v, want %#v", got.Selection.Tests, want)
	}
	if got.Selection.Count != len(want) {
		t.Errorf("selection.count = %d, want %d", got.Selection.Count, len(want))
	}
	if got.Reason == "" {
		t.Error("reason is empty; every selection must explain itself")
	}
	if got.Adapter != "python" {
		t.Errorf("adapter = %q, want python", got.Adapter)
	}
}

func TestWhichJSONReportsChangedLineRanges(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)
	writeFile(t, dir, "src/auth.py", "def login():\n    return 42\n")

	code, stdout, _ := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	got := decodeOutput(t, stdout)

	if len(got.Changed) != 1 {
		t.Fatalf("changed = %#v, want exactly one entry", got.Changed)
	}
	c := got.Changed[0]
	if c.Path != "src/auth.py" || c.Status != "modified" {
		t.Errorf("changed[0] = %+v, want src/auth.py modified", c)
	}
	if !c.Instrumentable {
		t.Errorf("changed[0].instrumentable = false; src/auth.py is source the adapter instruments")
	}
	if len(c.Lines) != 1 || c.Lines[0].Start != 2 || c.Lines[0].End != 2 {
		t.Errorf("lines = %#v, want [{2 2}]", c.Lines)
	}
}

// Every slice is an array, never null: an agent consumer must not have to special-case
// an absent key.
func TestWhichJSONEmptySelectionEmitsArraysNotNull(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)
	writeFile(t, dir, "src/orphan_module.py", "def orphan():\n    return 0\n")

	code, stdout, _ := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (an empty selection is a signal, not a failure)", code)
	}
	if strings.Contains(stdout, "null") {
		t.Errorf("which --json emitted null:\n%s", stdout)
	}
	got := decodeOutput(t, stdout)
	if got.Tier != "empty" {
		t.Errorf("tier = %q, want empty", got.Tier)
	}
	if len(got.Selection.Tests) != 0 {
		t.Errorf("selection.tests = %#v, want empty", got.Selection.Tests)
	}
}

// A missing adapter disables file classification entirely, which narrows the selection
// without narrowing anything a consumer can see. It is a `warnings` entry, not a stderr
// aside — `adapter` being `""` is a symptom, not the explanation.
func TestWhichJSONCarriesTheMissingAdapterWarningInTheDocument(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)
	if err := os.Remove(filepath.Join(dir, ".rtdd", "adapter.yaml")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "tests/test_new.py", "def test_new():\n    pass\n")

	code, stdout, stderr := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	got := decodeOutput(t, stdout)
	if !anyWarningContains(got.Warnings, "classification is disabled") {
		t.Errorf("warnings must say file classification is disabled, got %#v", got.Warnings)
	}
	if !strings.Contains(stderr, "classification is disabled") {
		t.Errorf("stderr must keep the warning for humans:\n%s", stderr)
	}
}

// The fields are a guarantee, not decoration: a fully-enumerated tier reports itself
// complete and carries no warnings, so `warnings` being non-empty always means something.
func TestWhichJSONReportsACompleteSelectionWithNoWarnings(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)
	writeFile(t, dir, "src/auth.py", "def login():\n    return True\n")

	code, stdout, stderr := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	got := decodeOutput(t, stdout)
	if got.Tier != "T0" {
		t.Fatalf("tier = %q, want T0 (reason: %s)", got.Tier, got.Reason)
	}
	if !got.Complete {
		t.Errorf("complete = false; a T0 selection names every test it will run:\n%s", stdout)
	}
	if len(got.Warnings) != 0 {
		t.Errorf("warnings = %#v, want none for an unremarkable T0 selection", got.Warnings)
	}
}
