package main

import (
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/mapstore"
)

func explainFixture() *mapstore.Map {
	keep := func(a, b string) string { return a }
	m := mapstore.New()
	m.Union(mapstore.Row{T: "tests/test_b.py::t2", F: []string{"src/hub.py"}, C: "aaa", D: 90, S: "pass"}, keep)
	m.Union(mapstore.Row{T: "tests/test_a.py::t1", F: []string{"src/hub.py", "src/a.py"}, C: "aaa", D: 12, S: "fail"}, keep)
	m.Union(mapstore.Row{T: "tests/test_c.py::t3", F: []string{"src/other.py"}, C: "aaa", D: 5, S: "pass"}, keep)
	return m
}

func TestRenderExplainListsCoveringTests(t *testing.T) {
	got := RenderExplain(explainFixture(), "src/hub.py", nil)
	want := "" +
		"src/hub.py is covered by 2 tests:\n" +
		"    tests/test_a.py::t1        12ms  fail\n" +
		"    tests/test_b.py::t2        90ms  pass\n"
	if got != want {
		t.Fatalf("RenderExplain()\n got:\n%s\nwant:\n%s", got, want)
	}
}

// One covering test is "1 test", not "1 tests": the count is read by humans.
func TestRenderExplainSingularCount(t *testing.T) {
	got := RenderExplain(explainFixture(), "src/a.py", nil)
	want := "" +
		"src/a.py is covered by 1 test:\n" +
		"    tests/test_a.py::t1        12ms  fail\n"
	if got != want {
		t.Fatalf("RenderExplain()\n got:\n%s\nwant:\n%s", got, want)
	}
}

// Equal durations tie-break on test id so the listing is stable across runs.
func TestRenderExplainBreaksDurationTiesOnTestID(t *testing.T) {
	keep := func(a, b string) string { return a }
	m := mapstore.New()
	m.Union(mapstore.Row{T: "tests/test_z.py::t", F: []string{"src/hub.py"}, C: "aaa", D: 7, S: "pass"}, keep)
	m.Union(mapstore.Row{T: "tests/test_a.py::t", F: []string{"src/hub.py"}, C: "aaa", D: 7, S: "pass"}, keep)

	got := RenderExplain(m, "src/hub.py", nil)
	want := "" +
		"src/hub.py is covered by 2 tests:\n" +
		"    tests/test_a.py::t          7ms  pass\n" +
		"    tests/test_z.py::t          7ms  pass\n"
	if got != want {
		t.Fatalf("RenderExplain()\n got:\n%s\nwant:\n%s", got, want)
	}
}

// Zero covering tests is the import-time-only case as often as the untested case. The
// output must never let a reader conclude "untested" on its own.
func TestRenderExplainNoCoveringTests(t *testing.T) {
	got := RenderExplain(explainFixture(), "src/constants.py", nil)
	want := "" +
		"src/constants.py is covered by 0 tests.\n" +
		"  No map row lists this file. Either nothing exercises it, or it only ever\n" +
		"  executes at import time, where coverage attributes it to no test at all\n" +
		"  (spec §6). Selection falls back to a static import scan for this file.\n"
	if got != want {
		t.Fatalf("RenderExplain()\n got:\n%s\nwant:\n%s", got, want)
	}
	if !strings.Contains(got, "import time") {
		t.Error("the zero-coverage message must name the import-time case")
	}
}

func TestRenderExplainEmptyMap(t *testing.T) {
	got := RenderExplain(mapstore.New(), "src/hub.py", nil)
	want := "" +
		"src/hub.py is covered by 0 tests.\n" +
		"  The map is empty. Run `rtdd seed` first.\n"
	if got != want {
		t.Fatalf("RenderExplain()\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestExplainCommandListsCoveringTestsAscendingByDuration(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)

	code, stdout, stderr := rtdd(t, dir, "explain", "src/db.py")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	// The id column widens to the longest id, so the durations stay in one column.
	want := "" +
		"src/db.py is covered by 2 tests:\n" +
		"    tests/test_db.py::test_query    15ms  pass\n" +
		"    tests/test_auth.py::test_login 412ms  pass\n"
	if stdout != want {
		t.Fatalf("explain output\n got:\n%s\nwant:\n%s", stdout, want)
	}
}

// The argument goes through internal/paths: a dot-relative or absolute path names the
// same map row as the repo-relative one.
func TestExplainNormalisesThePathArgument(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)

	_, want, _ := rtdd(t, dir, "explain", "src/db.py")
	for _, arg := range []string{"./src/db.py", "src/../src/db.py"} {
		code, stdout, stderr := rtdd(t, dir, "explain", arg)
		if code != 0 {
			t.Fatalf("explain %s: exit code = %d, want 0 (stderr: %s)", arg, code, stderr)
		}
		if stdout != want {
			t.Errorf("explain %s\n got:\n%s\nwant:\n%s", arg, stdout, want)
		}
	}
}

// A path the map has never heard of is a normal answer, not an error.
func TestExplainOnAPathAbsentFromTheMapExitsZero(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)

	code, stdout, stderr := rtdd(t, dir, "explain", "src/never_seen.py")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, want := range []string{
		"src/never_seen.py is covered by 0 tests.",
		"import time",
		"static import scan",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("explain output is missing %q:\n%s", want, stdout)
		}
	}
}

func TestExplainOnAnUnseededRepoSaysToSeed(t *testing.T) {
	dir := newTestRepo(t)

	code, stdout, stderr := rtdd(t, dir, "explain", "src/db.py")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "rtdd seed") {
		t.Errorf("explain on an empty map must say to seed:\n%s", stdout)
	}
}

func TestExplainWithoutAFileIsAUsageError(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)

	for _, args := range [][]string{
		{"explain"},
		{"explain", "src/db.py", "src/auth.py"},
	} {
		code, _, stderr := rtdd(t, dir, args...)
		if code != 2 {
			t.Errorf("rtdd %v: exit code = %d, want 2", args, code)
		}
		if !strings.Contains(stderr, "usage") {
			t.Errorf("rtdd %v: stderr = %q, want usage text", args, stderr)
		}
	}
}

func TestExplainOnAPathOutsideTheRepositoryIsAUsageError(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)

	code, _, stderr := rtdd(t, dir, "explain", "../outside.py")
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "outside") {
		t.Errorf("stderr = %q, want it to say the path is outside the repository", stderr)
	}
}

func TestExplainWithAnUnknownFlagIsAUsageError(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)

	code, _, stderr := rtdd(t, dir, "explain", "--frobnicate", "src/db.py")
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if stderr == "" {
		t.Error("stderr is empty; a usage error must say what was wrong")
	}
}

func TestUsageDocumentsExplain(t *testing.T) {
	dir := newTestRepo(t)
	code, stdout, _ := rtdd(t, dir, "help")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "rtdd explain <file>") {
		t.Errorf("usage text does not document explain:\n%s", stdout)
	}
}
