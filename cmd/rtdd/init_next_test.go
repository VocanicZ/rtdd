package main

import (
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// newPolyglotRepo detects BOTH the built-in python adapter (pyproject.toml) and the
// host-authored vitest one (package.json). It is the case that stops the seed guidance
// from being a global flag flip: the repository still has a coverage adapter to seed.
func newPolyglotRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.Init(t)
	gittest.Write(t, dir, "pyproject.toml", "[project]\nname = \"demo\"\nversion = \"0.1.0\"\n")
	gittest.Write(t, dir, "src/logic.py", "def add(a, b):\n    return a + b\n")
	gittest.Write(t, dir, "tests/test_logic.py", "def test_add():\n    pass\n")
	gittest.Write(t, dir, "package.json", "{\n  \"name\": \"demo\"\n}\n")
	gittest.Write(t, dir, "src/logic.ts", "export const add = (a: number, b: number) => a + b;\n")
	gittest.Write(t, dir, "src/logic.test.ts", "it('adds', () => {});\n")
	gittest.Write(t, dir, ".gitignore", ".rtdd/\n")
	gittest.Commit(t, dir, "init")
	return dir
}

// The defect this closes: a repository whose only adapter declares selection: static was
// still told to run `rtdd seed`. coverage: none means no map is ever built, so the
// command cannot do anything — the agent that follows the instruction burns a cycle.
func TestInitInAStaticOnlyRepoDoesNotAdviseSeeding(t *testing.T) {
	dir := newUnsupportedRepo(t)
	writeVitestAdapter(t, dir, "")
	fixLookPath(t)

	code, stdout, stderr := rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("rtdd init = %d, want 0 (stderr: %s)", code, stderr)
	}
	if strings.Contains(stdout, "rtdd seed") {
		t.Errorf("init told a selection: static repository to seed:\n%s", stdout)
	}
	// It still has to say what to run: dropping the line leaves the caller with nothing.
	if !strings.Contains(stdout, "Next:") {
		t.Errorf("init printed no next step at all:\n%s", stdout)
	}
	for _, want := range []string{"rtdd which", "vitest"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("next step does not mention %q:\n%s", want, stdout)
		}
	}
}

// The Python-only path is unchanged, word for word: seeding really is the next step for
// an adapter that records coverage.
func TestInitInACoverageRepoKeepsTheSeedNextStep(t *testing.T) {
	dir := newDetectableRepo(t)
	fixLookPath(t)

	code, stdout, stderr := rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("rtdd init = %d, want 0 (stderr: %s)", code, stderr)
	}
	const want = "Next: run `rtdd seed` once to build .rtdd/map.jsonl, then commit it."
	if !strings.Contains(stdout, want) {
		t.Errorf("stdout does not contain %q:\n%s", want, stdout)
	}
}

// A polyglot repository keeps the seed guidance, scoped to the adapter it applies to.
// Suppressing it here would strand the coverage adapter's map, and printing it unscoped
// would send the caller to seed the static one.
func TestInitInAPolyglotRepoScopesTheSeedGuidance(t *testing.T) {
	dir := newPolyglotRepo(t)
	writeVitestAdapter(t, dir, "")
	fixLookPath(t)

	code, stdout, stderr := rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("rtdd init = %d, want 0 (stderr: %s)", code, stderr)
	}
	next := stdout[strings.Index(stdout, "Next:"):]
	if !strings.Contains(next, "rtdd seed") {
		t.Fatalf("polyglot repo lost the seed guidance its coverage adapter needs:\n%s", next)
	}
	if !strings.Contains(next, "python") {
		t.Errorf("seed guidance does not name the adapter it applies to:\n%s", next)
	}
	if !strings.Contains(next, "vitest") {
		t.Errorf("next step does not say which adapter seeding does NOT apply to:\n%s", next)
	}
}

// RenderNextStep unit-tested directly, because the end-to-end cases above can only exercise the
// adapter sets a fixture repository can actually detect. The plural forms matter: a
// message reading "the vitest adapters select" reads as a bug in the tool, and a reader
// who distrusts the sentence distrusts the advice in it.
func TestRenderNextStepIsFidelityAware(t *testing.T) {
	ad := func(name, selection string) *adapter.Adapter {
		return &adapter.Adapter{Name: name, Selection: selection}
	}
	py := ad("python", adapter.SelectionCoverage)
	rb := ad("ruby", adapter.SelectionCoverage)
	vi := ad("vitest", adapter.SelectionStatic)
	je := ad("jest", adapter.SelectionStatic)

	const seedLine = "Next: run `rtdd seed` once to build .rtdd/map.jsonl, then commit it."

	for _, tc := range []struct {
		name     string
		detected []*adapter.Adapter
		want     string
		absent   []string
	}{
		{
			name:     "a coverage adapter keeps today's line word for word",
			detected: []*adapter.Adapter{py},
			want:     seedLine,
		},
		{
			// --force in a repo nothing matched: no declaration says seeding is
			// pointless, and the installed front-end already carries the caveat.
			name:     "no detected adapter keeps today's line",
			detected: nil,
			want:     seedLine,
		},
		{
			name:     "one static adapter",
			detected: []*adapter.Adapter{vi},
			want:     "The vitest adapter selects statically and records no coverage",
			absent:   []string{"run `rtdd seed`", "map.jsonl"},
		},
		{
			name:     "two static adapters agree in number",
			detected: []*adapter.Adapter{vi, je},
			want:     "The vitest, jest adapters select statically and record no coverage",
			absent:   []string{"run `rtdd seed`", "map.jsonl"},
		},
		{
			name:     "a mix names every adapter seeding applies to",
			detected: []*adapter.Adapter{py, rb, vi, je},
			want:     "for the python, ruby adapters",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := RenderNextStep(tc.detected)
			if !strings.Contains(got, tc.want) {
				t.Errorf("RenderNextStep = %q, want it to contain %q", got, tc.want)
			}
			for _, never := range tc.absent {
				if strings.Contains(got, never) {
					t.Errorf("RenderNextStep = %q, must not contain %q", got, never)
				}
			}
		})
	}
}
