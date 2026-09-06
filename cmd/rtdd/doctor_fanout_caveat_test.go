package main

import (
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/doctor"
	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// Issue #346 / PRD #233 AC9c: doctor states that fan-out is never computed for an adapter
// that records nothing, and then qualified that non-computation with the §9 caveat — a
// caveat about numbers that were never produced, written in Python vocabulary, in a repo
// with no Python in it. Where fan-out cannot be computed the caveat is suppressed.
func TestRenderDoctorSuppressesTheFanOutCaveatWhereFanOutIsNeverComputed(t *testing.T) {
	got := RenderDoctor(nil, 0, 20, []*adapter.Adapter{staticAdapter()})
	if strings.Contains(got, doctor.Caveat) {
		t.Errorf("the fan-out caveat qualifies a computation that never happened:\n%s", got)
	}
	for _, forbidden := range []string{"lru_cache", "CAVEAT"} {
		if strings.Contains(got, forbidden) {
			t.Errorf("doctor output still contains %q for a non-recording adapter:\n%s", forbidden, got)
		}
	}
	// Suppressing the caveat must not suppress the sentence it was attached to.
	for _, want := range []string{"fan-out", "stays empty", "vitest"} {
		if !strings.Contains(got, want) {
			t.Errorf("doctor output no longer states %q:\n%s", want, got)
		}
	}
}

// A repository whose adapters record nothing gets no trailing blank line where the caveat
// used to be: the block ends on the sentence, not on whitespace.
func TestRenderDoctorStaticOnlyBlockEndsOnItsOwnSentence(t *testing.T) {
	got := RenderDoctor(nil, 0, 20, []*adapter.Adapter{staticAdapter()})
	if strings.HasSuffix(got, "\n\n") {
		t.Errorf("doctor left the caveat's blank separator behind:\n%q", got)
	}
}

// The mixed repository is the case suppression must not swallow: one half records, so
// fan-out IS computed here and the §9 caveat is exactly as load-bearing as ever.
func TestRenderDoctorKeepsTheFanOutCaveatWhenAnyDetectedAdapterRecords(t *testing.T) {
	for _, tc := range []struct {
		name     string
		detected []*adapter.Adapter
	}{
		{"mixed", []*adapter.Adapter{staticAdapter(), coverageAdapter()}},
		{"coverage only", []*adapter.Adapter{coverageAdapter()}},
		{"nothing detected", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := RenderDoctor(nil, 0, 20, tc.detected)
			if !strings.Contains(got, doctor.Caveat) {
				t.Fatalf("the §9 caveat was dropped where fan-out is computed:\n%s", got)
			}
		})
	}
}

// A populated table is a computed fan-out whatever the adapters declare: the numbers are
// on screen, so the limitation that qualifies them stays on screen too.
func TestRenderDoctorKeepsTheFanOutCaveatOverAPopulatedTable(t *testing.T) {
	hubs := []doctor.Hub{{Path: "src/a.ts", TestCount: 1, Fraction: 1}}
	got := RenderDoctor(hubs, 1, 20, []*adapter.Adapter{staticAdapter()})
	if !strings.Contains(got, doctor.Caveat) {
		t.Fatalf("a rendered fan-out table lost its §9 caveat:\n%s", got)
	}
}

// AC3: a non-Python `coverage: none` repository, end to end through cmdDoctor.
func TestDoctorOnANonPythonNonRecordingRepoOmitsTheFanOutCaveat(t *testing.T) {
	dir := newVitestRepo(t)

	code, stdout, stderr := rtdd(t, dir, "doctor")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if strings.Contains(stdout, "lru_cache") {
		t.Errorf("a TypeScript repo was handed Python vocabulary:\n%s", stdout)
	}
	if strings.Contains(stdout, doctor.Caveat) {
		t.Errorf("the fan-out caveat was printed where fan-out is never computed:\n%s", stdout)
	}
	// The static-tier caveat is a different claim and stays; nothing else may say CAVEAT.
	if strings.Contains(strings.ReplaceAll(stdout, doctor.StaticCaveat, ""), "CAVEAT") {
		t.Errorf("a fan-out CAVEAT survived in a repo that computes no fan-out:\n%s", stdout)
	}
	if !strings.Contains(stdout, "stays empty") {
		t.Errorf("doctor no longer states that no map is ever built here:\n%s", stdout)
	}
}

// AC2: where fan-out IS computed the output is byte-identical to what it always was.
func TestDoctorOnAPythonRepoPrintsTheFanOutCaveatUnchanged(t *testing.T) {
	dir := newTestRepo(t)
	gittest.Write(t, dir, "pyproject.toml", "[project]\nname = \"demo\"\nversion = \"0.1.0\"\n")
	installRTDD(t, dir, headShort(t, dir), 0)

	code, stdout, stderr := rtdd(t, dir, "doctor")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	want := "" +
		"fan-out over 4 tests (4 files)\n" +
		"\n" +
		"  tests  share  file\n" +
		"      2    50%  src/auth.py\n" +
		"      2    50%  src/db.py\n" +
		"      1    25%  src/render.py\n" +
		"      1    25%  templates/page.html\n" +
		"\n" +
		doctor.Caveat + "\n"
	if !strings.HasSuffix(stdout, want) {
		t.Fatalf("the Python-adapter fan-out block changed\n got:\n%s\nwant suffix:\n%s", stdout, want)
	}
}
