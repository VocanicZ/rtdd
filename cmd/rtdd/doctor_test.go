package main

import (
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/doctor"
	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

func TestRenderDoctorTable(t *testing.T) {
	hubs := []doctor.Hub{
		{Path: "src/hub.py", TestCount: 3, Fraction: 0.75},
		{Path: "src/b.py", TestCount: 2, Fraction: 0.5},
		{Path: "src/a.py", TestCount: 1, Fraction: 0.25},
	}
	got := RenderDoctor(hubs, 4, 2, nil)
	want := "" +
		"fan-out over 4 tests (top 2 of 3 files)\n" +
		"\n" +
		"  tests  share  file\n" +
		"      3    75%  src/hub.py\n" +
		"      2    50%  src/b.py\n" +
		"\n" +
		doctor.Caveat + "\n"
	if got != want {
		t.Fatalf("RenderDoctor()\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestRenderDoctorAlwaysPrintsTheCaveat(t *testing.T) {
	// Spec §9: the limitation must be stated in the tool's own output, alongside the
	// table, because the repo's most coupled file can appear as its cleanest.
	for _, tc := range []struct {
		name string
		hubs []doctor.Hub
		tot  int
	}{
		{"populated", []doctor.Hub{{Path: "src/a.py", TestCount: 1, Fraction: 1}}, 1},
		{"empty map", nil, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := RenderDoctor(tc.hubs, tc.tot, 20, nil)
			if !strings.Contains(got, doctor.Caveat) {
				t.Fatalf("doctor output is missing the §9 caveat:\n%s", got)
			}
			for _, needle := range []string{"lru_cache", "session-scoped fixture", "fan-out of 1", "cleanest"} {
				if !strings.Contains(got, needle) {
					t.Fatalf("doctor output is missing %q:\n%s", needle, got)
				}
			}
		})
	}
}

// The caveat names every once-per-process mechanism the spec calls out, including the
// two the table itself cannot show: module singletons and DI container wiring.
func TestRenderDoctorCaveatNamesEveryOncePerProcessMechanism(t *testing.T) {
	got := RenderDoctor(nil, 0, 20, nil)
	for _, needle := range []string{"lru_cache", "singleton", "DI container", "session-scoped fixture"} {
		if !strings.Contains(got, needle) {
			t.Fatalf("doctor output is missing %q:\n%s", needle, got)
		}
	}
}

func TestRenderDoctorEmptyMap(t *testing.T) {
	got := RenderDoctor(nil, 0, 20, nil)
	if !strings.Contains(got, "map is empty") {
		t.Fatalf("RenderDoctor() on an empty map should say so:\n%s", got)
	}
	if !strings.Contains(got, "rtdd seed") {
		t.Fatalf("RenderDoctor() should point at `rtdd seed`:\n%s", got)
	}
}

// A limit at or above the file count is not a truncation, and the header must not claim
// a "top N of N" that implies something was left out.
func TestRenderDoctorLimitBeyondTheFileCountShowsEverything(t *testing.T) {
	hubs := []doctor.Hub{
		{Path: "src/hub.py", TestCount: 3, Fraction: 0.75},
		{Path: "src/b.py", TestCount: 2, Fraction: 0.5},
	}
	got := RenderDoctor(hubs, 4, 20, nil)
	want := "" +
		"fan-out over 4 tests (2 files)\n" +
		"\n" +
		"  tests  share  file\n" +
		"      3    75%  src/hub.py\n" +
		"      2    50%  src/b.py\n" +
		"\n" +
		doctor.Caveat + "\n"
	if got != want {
		t.Fatalf("RenderDoctor()\n got:\n%s\nwant:\n%s", got, want)
	}
}

// A non-positive limit means "no limit"; it must never render an empty table.
func TestRenderDoctorNonPositiveLimitShowsEverything(t *testing.T) {
	hubs := []doctor.Hub{
		{Path: "src/hub.py", TestCount: 3, Fraction: 0.75},
		{Path: "src/b.py", TestCount: 2, Fraction: 0.5},
	}
	for _, limit := range []int{0, -1} {
		got := RenderDoctor(hubs, 4, limit, nil)
		if !strings.Contains(got, "src/b.py") {
			t.Fatalf("limit=%d dropped rows:\n%s", limit, got)
		}
	}
}

// One test in the map is "1 test", not "1 tests": the header is read by humans.
func TestRenderDoctorSingularTestCount(t *testing.T) {
	hubs := []doctor.Hub{{Path: "src/a.py", TestCount: 1, Fraction: 1}}
	got := RenderDoctor(hubs, 1, 20, nil)
	if !strings.Contains(got, "fan-out over 1 test (1 file)") {
		t.Fatalf("RenderDoctor() header is not singular:\n%s", got)
	}
}

func TestDoctorCommandRanksFilesByFanOut(t *testing.T) {
	dir := newTestRepo(t)
	// The python adapter's detection marker: doctor reports fidelity per DETECTED
	// adapter, so the golden output below is what a plain seeded Python repo prints.
	gittest.Write(t, dir, "pyproject.toml", "[project]\nname = \"demo\"\nversion = \"0.1.0\"\n")
	installRTDD(t, dir, headShort(t, dir), 0)

	code, stdout, stderr := rtdd(t, dir, "doctor")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	// The selection-fidelity block precedes the fan-out table on every repo (spec §6);
	// the table itself and its §9 caveat are byte-identical to what doctor always printed.
	//
	// Between them sits the not-detected section, because the shipped set is ten adapters
	// and a Python repo detects one of them. It is rendered here rather than spelled out:
	// its content is every OTHER built-in with its markers, which changes whenever an
	// adapter ships, and pinning that list in a golden would make every new adapter a
	// failure in a test about fan-out ranking.
	want := "" +
		"selection fidelity\n" +
		"\n" +
		"  python  python.yaml  (built-in)  execution-derived\n" +
		"      selection: coverage with coverage: sqlite — tests are chosen from per-test coverage recorded by a real run\n" +
		"\n" +
		RenderUndetected(fidelityRows(dir, builtinsExcept(t, "python"))) +
		"fan-out over 4 tests (4 files)\n" +
		"\n" +
		"  tests  share  file\n" +
		"      2    50%  src/auth.py\n" +
		"      2    50%  src/db.py\n" +
		"      1    25%  src/render.py\n" +
		"      1    25%  templates/page.html\n" +
		"\n" +
		doctor.Caveat + "\n"
	if stdout != want {
		t.Fatalf("doctor output\n got:\n%s\nwant:\n%s", stdout, want)
	}
}

func TestDoctorCommandHonoursLimit(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)

	code, stdout, stderr := rtdd(t, dir, "doctor", "--limit", "2")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "top 2 of 4 files") {
		t.Errorf("doctor --limit 2 header is wrong:\n%s", stdout)
	}
	if strings.Contains(stdout, "src/render.py") {
		t.Errorf("doctor --limit 2 printed a third row:\n%s", stdout)
	}
	if !strings.Contains(stdout, doctor.Caveat) {
		t.Errorf("a limited table still needs the §9 caveat:\n%s", stdout)
	}
}

// An unseeded repo is a normal answer, not a crash and not a failure exit.
func TestDoctorOnAnUnseededRepoSaysToSeedAndStillCaveats(t *testing.T) {
	dir := newTestRepo(t)

	code, stdout, stderr := rtdd(t, dir, "doctor")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "rtdd seed") {
		t.Errorf("doctor on an empty map must say to seed:\n%s", stdout)
	}
	if !strings.Contains(stdout, doctor.Caveat) {
		t.Errorf("doctor on an empty map must still print the §9 caveat:\n%s", stdout)
	}
}

func TestDoctorWithAPositionalArgumentIsAUsageError(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)

	code, _, stderr := rtdd(t, dir, "doctor", "src/db.py")
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "usage") {
		t.Errorf("stderr = %q, want usage text", stderr)
	}
}

func TestDoctorWithAnUnknownFlagIsAUsageError(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)

	code, _, stderr := rtdd(t, dir, "doctor", "--frobnicate")
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if stderr == "" {
		t.Error("stderr is empty; a usage error must say what was wrong")
	}
}

func TestUsageDocumentsDoctor(t *testing.T) {
	dir := newTestRepo(t)
	code, stdout, _ := rtdd(t, dir, "help")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "rtdd doctor") {
		t.Errorf("usage text does not document doctor:\n%s", stdout)
	}
}

// builtinsExcept returns every embedded adapter but the named ones, in Builtin order —
// the resolved-but-undetected remainder a single-language repo leaves behind.
func builtinsExcept(t *testing.T, skip ...string) []*adapter.Adapter {
	t.Helper()
	all, err := adapter.Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	drop := map[string]bool{}
	for _, n := range skip {
		drop[n] = true
	}
	var out []*adapter.Adapter
	for _, a := range all {
		if !drop[a.Name] {
			out = append(out, a)
		}
	}
	return out
}
