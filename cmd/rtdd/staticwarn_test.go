package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/doctor"
	"github.com/VocanicZ/rtdd/internal/selector"
)

// The static-tier caveat (PRD #233 AC7, spec §6) is the sentence that stops an agent
// reading a green static run as the same proof a green execution-derived run is. These
// tests assert it on every surface the agent actually reads: `warnings` in both --json
// documents, and the human notes beside them.

// hasStaticCaveat reports whether s carries the caveat's wording. The needle is the
// shared const, so a reworded caveat moves every assertion at once rather than leaving
// the tests asserting a sentence the tool no longer prints.
func hasStaticCaveat(s string) bool {
	return strings.Contains(s, doctor.StaticSelectionCaveat)
}

// anyStaticCaveat is the same question over a warnings array.
func anyStaticCaveat(warnings []string) bool {
	for _, w := range warnings {
		if hasStaticCaveat(w) {
			return true
		}
	}
	return false
}

// AC2: the text states BOTH halves. Either alone is misread — the miss alone reads as a
// bug report about the tier, and the weaker-evidence half alone never says what is weaker
// about it.
func TestStaticSelectionCaveatStatesTheMissAndTheWeakerEvidence(t *testing.T) {
	for _, want := range []string{"static", "miss", "execution-derived", "weaker evidence"} {
		if !strings.Contains(doctor.StaticSelectionCaveat, want) {
			t.Errorf("the static caveat does not contain %q:\n%s", want, doctor.StaticSelectionCaveat)
		}
	}
	// doctor prints the same sentence under its own CAVEAT: prefix. One const, three
	// surfaces: a caveat worded two ways is one an agent has to reconcile.
	if !strings.Contains(doctor.StaticCaveat, doctor.StaticSelectionCaveat) {
		t.Errorf("doctor.StaticCaveat no longer carries the shared sentence:\n%s", doctor.StaticCaveat)
	}
}

// AC1, `which --json`: the document is the WHOLE of stdout for an agent front-end, so a
// caveat that lived only on stderr would be one it never sees.
func TestStaticSelectionEmitsTheCaveatInWhichJSONWarnings(t *testing.T) {
	dir := staticRunRepo(t)
	touchStaticSource(t, dir)

	code, stdout, stderr := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	got := decodeOutput(t, stdout)
	if got.Tier != "TS" {
		t.Fatalf("fixture selected at tier %s, want TS", got.Tier)
	}
	if !anyStaticCaveat(got.Warnings) {
		t.Errorf("which --json warnings carry no static caveat: %#v", got.Warnings)
	}
	// AC1's other half: the same sentence reaches the human, on the stream that cannot
	// corrupt the single JSON document a parser reads from stdout.
	if !hasStaticCaveat(stderr) {
		t.Errorf("which --json printed no static caveat to stderr:\n%s", stderr)
	}
}

// The human surface of `which`, which prints its notes rather than a document.
func TestStaticSelectionEmitsTheCaveatInWhichHumanNotes(t *testing.T) {
	dir := staticRunRepo(t)
	touchStaticSource(t, dir)

	code, stdout, stderr := rtdd(t, dir, "which")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !hasStaticCaveat(stdout) {
		t.Errorf("which printed no static caveat:\n%s", stdout)
	}
}

// AC1, `rtdd run --json`. captureStdout, not the rtdd() helper: emitJSON writes the
// document to os.Stdout itself.
func TestStaticSelectionEmitsTheCaveatInRunJSONWarnings(t *testing.T) {
	dir := staticRunRepo(t)
	touchStaticSource(t, dir)
	chdir(t, dir)

	code := -1
	stdout := captureStdout(t, func() { code = cmdRun([]string{"--json"}) })
	if code != 0 {
		t.Fatalf("rtdd run --json = %d, want 0\nstdout:\n%s", code, stdout)
	}
	got := decodeOutput(t, stdout)
	if got.Tier != "TS" {
		t.Fatalf("fixture selected at tier %s, want TS", got.Tier)
	}
	if !anyStaticCaveat(got.Warnings) {
		t.Errorf("run --json warnings carry no static caveat: %#v", got.Warnings)
	}
}

// `rtdd run`'s human surface puts its caveats on stderr, where stdout stays the report.
func TestStaticSelectionEmitsTheCaveatOnRunStderr(t *testing.T) {
	dir := staticRunRepo(t)
	touchStaticSource(t, dir)
	chdir(t, dir)

	code := -1
	stderr := captureStderr(t, func() {
		captureStdout(t, func() { code = cmdRun(nil) })
	})
	if code != 0 {
		t.Fatalf("rtdd run = %d, want 0\nstderr:\n%s", code, stderr)
	}
	if !hasStaticCaveat(stderr) {
		t.Errorf("rtdd run printed no static caveat to stderr:\n%s", stderr)
	}
}

// AC3: an execution-derived selection emits NO such warning, and its warnings array is
// exactly what it was before this caveat existed — empty for a seeded repository whose
// map answered.
func TestExecutionDerivedSelectionWarningsAreUnchanged(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)
	writeFile(t, dir, "src/auth.py", "def login():\n    return True\n")

	code, stdout, stderr := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	got := decodeOutput(t, stdout)
	if got.Tier != "T0" {
		t.Fatalf("fixture selected at tier %s, want T0", got.Tier)
	}
	if len(got.Warnings) != 0 {
		t.Errorf("an execution-derived selection gained warnings: %#v", got.Warnings)
	}
	if hasStaticCaveat(stderr) {
		t.Errorf("an execution-derived selection printed the static caveat:\n%s", stderr)
	}
}

// AC4: in a polyglot repository with MIXED fidelities the caveat qualifies the static
// half only, and says which half that is. Unattributed it reads as a claim about the
// whole document — including the block whose map really did answer.
func TestMixedFidelityPolyglotNamesTheAdapterTheCaveatQualifies(t *testing.T) {
	dir := mixedFidelityRepo(t)

	code, stdout, stderr := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	got := decodeOutput(t, stdout)
	if len(got.Selections) != 2 {
		t.Fatalf("selections = %+v, want one block per detected adapter", got.Selections)
	}
	byName := map[string]JSONAdapterSelection{}
	for _, blk := range got.Selections {
		byName[blk.Adapter] = blk
	}
	if byName["python"].SelectionFidelity != "execution-derived" {
		t.Fatalf("python block fidelity = %q (tier %s), want execution-derived",
			byName["python"].SelectionFidelity, byName["python"].Tier)
	}
	if byName["vitest"].SelectionFidelity != "static" {
		t.Fatalf("vitest block fidelity = %q (tier %s), want static",
			byName["vitest"].SelectionFidelity, byName["vitest"].Tier)
	}

	var caveats []string
	for _, w := range got.Warnings {
		if hasStaticCaveat(w) {
			caveats = append(caveats, w)
		}
	}
	if len(caveats) != 1 {
		t.Fatalf("want exactly one static caveat over a mixed repository, got %#v", got.Warnings)
	}
	if !strings.HasPrefix(caveats[0], "vitest: ") {
		t.Errorf("the caveat does not name the adapter it qualifies: %q", caveats[0])
	}
	if strings.Contains(caveats[0], "python") {
		t.Errorf("the caveat tars the execution-derived adapter: %q", caveats[0])
	}
}

// AC5: nothing selected under static fidelity is the weakest evidence of all, so the
// caveat is emitted for an empty TS selection too — beside the note that says an empty
// selection is not a pass, never instead of it.
func TestEmptyStaticSelectionCarriesBothTheEmptyNoteAndTheCaveat(t *testing.T) {
	sel := selector.Selection{Tier: selector.TierTS, Reason: "declared test_for correspondence"}
	notes := runNotes(sel, nil)
	if !anyStaticCaveat(notes) {
		t.Errorf("an empty TS selection emits no static caveat: %#v", notes)
	}
	var empty bool
	for _, n := range notes {
		if strings.Contains(n, "empty selection is not a pass") {
			empty = true
		}
	}
	if !empty {
		t.Errorf("the empty-selection note was displaced by the caveat: %#v", notes)
	}
}

// A T2 selection derived nothing at all — fidelity `none`, not `static` — and the caveat
// is about a selection that WAS derived, statically. Emitting it there would tell the
// reader a static answer exists to be weaker than a recorded one.
func TestFullSuiteSelectionEmitsNoStaticCaveat(t *testing.T) {
	notes := runNotes(selector.Selection{Tier: selector.TierT2, Tests: []string{"a"}}, nil)
	if anyStaticCaveat(notes) {
		t.Errorf("a T2 selection emitted the static caveat: %#v", notes)
	}
}

// mixedFidelityRepo is a polyglot repository whose two adapters answer at two different
// fidelities: the Python half selects from a seeded map, the vitest half can only ever
// select statically. It is the case AC4 is about, and the only one where scoping the
// caveat is load-bearing.
func mixedFidelityRepo(t *testing.T) string {
	t.Helper()
	dir := newPolyglotRepo(t)
	writeVitestAdapter(t, dir, "")

	sha := headShort(t, dir)
	writeFile(t, dir, ".rtdd/map.jsonl",
		`{"t":"tests/test_logic.py::test_add","f":["src/logic.py"],"c":"`+sha+`","d":11,"s":"pass"}`+"\n")
	writeFile(t, dir, ".rtdd/meta.json",
		`{"v":1,"adapter":"python","seeded_at":"`+sha+`","cycles":0}`+"\n")

	// Both halves change, so both adapters answer: the Python one over its map, the
	// vitest one over its declared correspondence.
	writeFile(t, dir, "src/logic.py", "def add(a, b):\n    return a + b + 0\n")
	writeFile(t, dir, "src/logic.ts", "export const add = (a: number, b: number) => a + b + 0;\n")
	if _, err := os.Stat(filepath.Join(dir, ".rtdd", "map.jsonl")); err != nil {
		t.Fatalf("map: %v", err)
	}
	return dir
}
