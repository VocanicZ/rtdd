package main

import (
	"bytes"
	"encoding/json"
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
	gittest.Write(t, dir, "tests/test_auth.py", "def test_login():\n    pass\n")
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

	meta := `{"v":1,"adapter":"python","seeded_at":"` + seedSHA + `","cycles":` +
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

func TestStatusOnASeededRepo(t *testing.T) {
	dir := newTestRepo(t)
	sha := headShort(t, dir)
	installRTDD(t, dir, sha, 7)

	code, stdout, stderr := rtdd(t, dir, "status")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, want := range []string{
		"adapter: python",
		"4 tests",
		"4 files",
		"cycles:  7 / 100",
		sha,
		"0 commits ago",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("status output is missing %q:\n%s", want, stdout)
		}
	}
}

func TestStatusOnAnUnseededRepo(t *testing.T) {
	dir := newTestRepo(t)

	code, stdout, stderr := rtdd(t, dir, "status")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "UNSEEDED") {
		t.Errorf("status must say UNSEEDED when the map is empty:\n%s", stdout)
	}
	if !strings.Contains(stdout, "adapter: none") {
		t.Errorf("status must report a missing adapter explicitly:\n%s", stdout)
	}
}

// An unreachable seed commit is reported as UNREACHABLE, never as fresh.
func TestStatusReportsAnUnreachableSeedCommit(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, "deadbee", 0)

	code, stdout, _ := rtdd(t, dir, "status")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "UNREACHABLE") {
		t.Errorf("status must flag an unreachable seed commit:\n%s", stdout)
	}
	if strings.Contains(stdout, "0 commits ago") {
		t.Errorf("status reported an unreachable commit as fresh:\n%s", stdout)
	}
}

func TestStatusOutsideAGitRepoExitsThree(t *testing.T) {
	dir := t.TempDir()
	code, _, stderr := rtdd(t, dir, "status")
	if code != 3 {
		t.Errorf("exit code = %d, want 3 (fatal environment error)", code)
	}
	if stderr == "" {
		t.Error("stderr is empty; a fatal environment error must say what went wrong")
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

// The regression that killed v1: a test file written but never `git add`ed is invisible
// to `git diff`, so it was never selected and never run.
func TestWhichSelectsAJustWrittenUntrackedTestFile(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)
	writeFile(t, dir, "tests/test_brand_new.py", "def test_new():\n    assert True\n")

	code, stdout, stderr := rtdd(t, dir, "which")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "tests/test_brand_new.py") {
		t.Fatalf("which did not select the just-written test file:\n%s", stdout)
	}
	if !strings.Contains(stdout, "direct:") {
		t.Errorf("which must report the direct tier:\n%s", stdout)
	}
}

func TestWhichRanksT0(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)
	writeFile(t, dir, "src/auth.py", "def login():\n    return 42\n")

	code, stdout, stderr := rtdd(t, dir, "which")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "tier:     T0") {
		t.Fatalf("want tier T0:\n%s", stdout)
	}
	logout := strings.Index(stdout, "tests/test_auth.py::test_logout")
	login := strings.Index(stdout, "tests/test_auth.py::test_login\n")
	if logout < 0 || login < 0 {
		t.Fatalf("both auth tests must be selected:\n%s", stdout)
	}
	if logout > login {
		t.Errorf("the last-failed, higher-ratio test must be listed first:\n%s", stdout)
	}
}

// An empty selection is exit 0 AND an explicit warning. It must never read as a pass.
func TestWhichEmptySelectionIsExitZeroAndSaysSo(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)
	writeFile(t, dir, "src/orphan_module.py", "def orphan():\n    return 0\n")

	code, stdout, stderr := rtdd(t, dir, "which")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (an empty selection is a signal, not a failure); stderr: %s", code, stderr)
	}
	if !strings.Contains(stdout, "tier:     empty") {
		t.Fatalf("want tier empty:\n%s", stdout)
	}
	if !strings.Contains(stdout, "not a pass") {
		t.Errorf("an empty selection must be reported in words, not as silence:\n%s", stdout)
	}
	if !strings.Contains(stdout, "selected: 0") {
		t.Errorf("want an explicit zero count:\n%s", stdout)
	}
}

func TestWhichEscalatesOnAFullEscalateFile(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)
	writeFile(t, dir, "requirements.txt", "pytest==9.0.3\n")

	code, stdout, _ := rtdd(t, dir, "which")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "tier:     T2") {
		t.Errorf("want tier T2:\n%s", stdout)
	}
	if !strings.Contains(stdout, "requirements.txt") {
		t.Errorf("the reason must name the file that forced the escalation:\n%s", stdout)
	}
}

func TestWhichEscalatesWhenTheSeedCommitIsUnreachable(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, "deadbee", 0)
	writeFile(t, dir, "src/auth.py", "def login():\n    return 42\n")

	code, stdout, _ := rtdd(t, dir, "which")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "tier:     T1") {
		t.Errorf("an unreachable row commit is unknown age, which escalates:\n%s", stdout)
	}
}

func TestWhichReportsDeletionsAndRespectsBase(t *testing.T) {
	dir := newTestRepo(t)
	base := headShort(t, dir)
	installRTDD(t, dir, base, 0)

	if err := os.Remove(filepath.Join(dir, "src", "render.py")); err != nil {
		t.Fatal(err)
	}

	code, stdout, _ := rtdd(t, dir, "which", "--base", base)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "deleted   src/render.py") {
		t.Errorf("a deletion must be shown:\n%s", stdout)
	}
	if !strings.Contains(stdout, "tests/test_render.py::test_page") {
		t.Errorf("a deleted path must still select the tests whose F contains it:\n%s", stdout)
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

type whichJSONForTest struct {
	Base    string   `json:"base"`
	Head    string   `json:"head"`
	Tier    string   `json:"tier"`
	Reason  string   `json:"reason"`
	Direct  []string `json:"direct"`
	Tests   []string `json:"tests"`
	Changed []struct {
		Path    string `json:"path"`
		OldPath string `json:"old_path"`
		Status  string `json:"status"`
		Lines   []struct {
			Start int `json:"start"`
			End   int `json:"end"`
		} `json:"lines"`
	} `json:"changed"`
	MapTests int `json:"map_tests"`
}

func decodeWhichJSON(t *testing.T, s string) whichJSONForTest {
	t.Helper()
	var out whichJSONForTest
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		t.Fatalf("which --json emitted unparseable JSON: %v\n%s", err, s)
	}
	return out
}

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
	got := decodeWhichJSON(t, stdout)

	if got.Tier != "T0" {
		t.Errorf("tier = %q, want T0 (reason: %s)", got.Tier, got.Reason)
	}
	if got.Base != "HEAD" {
		t.Errorf("base = %q, want HEAD", got.Base)
	}
	if got.Head != sha {
		t.Errorf("head = %q, want %q", got.Head, sha)
	}
	if got.MapTests != 4 {
		t.Errorf("map_tests = %d, want 4", got.MapTests)
	}
	if len(got.Direct) != 1 || got.Direct[0] != "tests/test_brand_new.py" {
		t.Errorf("direct = %#v, want [tests/test_brand_new.py]", got.Direct)
	}
	want := []string{
		"tests/test_brand_new.py",
		"tests/test_auth.py::test_logout",
		"tests/test_auth.py::test_login",
	}
	if !reflect.DeepEqual(got.Tests, want) {
		t.Errorf("tests = %#v, want %#v", got.Tests, want)
	}
	if got.Reason == "" {
		t.Error("reason is empty; every selection must explain itself")
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
	got := decodeWhichJSON(t, stdout)

	if len(got.Changed) != 1 {
		t.Fatalf("changed = %#v, want exactly one entry", got.Changed)
	}
	c := got.Changed[0]
	if c.Path != "src/auth.py" || c.Status != "modified" {
		t.Errorf("changed[0] = %+v, want src/auth.py modified", c)
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
	got := decodeWhichJSON(t, stdout)
	if got.Tier != "empty" {
		t.Errorf("tier = %q, want empty", got.Tier)
	}
	if len(got.Tests) != 0 {
		t.Errorf("tests = %#v, want empty", got.Tests)
	}
}

func TestWhichJSONReportsARename(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)
	if err := os.Rename(
		filepath.Join(dir, "src", "render.py"),
		filepath.Join(dir, "src", "renderer.py"),
	); err != nil {
		t.Fatal(err)
	}
	gitRun(t, dir, "add", "-A")

	code, stdout, _ := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	got := decodeWhichJSON(t, stdout)

	found := false
	for _, c := range got.Changed {
		if c.Path == "src/renderer.py" && c.OldPath == "src/render.py" {
			found = true
		}
	}
	if !found {
		t.Errorf("changed = %#v, want a rename carrying old_path", got.Changed)
	}
	hit := false
	for _, id := range got.Tests {
		if id == "tests/test_render.py::test_page" {
			hit = true
		}
	}
	if !hit {
		t.Errorf("tests = %#v, want the old path to still select its test", got.Tests)
	}
}
