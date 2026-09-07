package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
	"github.com/VocanicZ/rtdd/internal/selector"
)

// wireFidelities is the closed set spec §6 defines. A value outside it is a value no
// consumer can branch on, which is the whole failure `selection_fidelity` exists to
// prevent — so the tests below assert membership, not merely non-emptiness.
var wireFidelities = map[adapter.Fidelity]bool{
	adapter.FidelityExecution: true,
	adapter.FidelityStatic:    true,
	adapter.FidelityNone:      true,
}

// Spec §6: selection_fidelity answers "what was THIS answer derived from", which is not
// the same question Adapter.Fidelity() answers ("what is the best this adapter could ever
// produce"). A coverage adapter with an unseeded map escalates to T2, and that answer was
// derived from nothing at all.
func TestSelectionFidelityIsDerivedFromTheTierThatAnswered(t *testing.T) {
	for _, tc := range []struct {
		tier selector.Tier
		want adapter.Fidelity
	}{
		{selector.TierDirect, adapter.FidelityExecution},
		{selector.TierT0, adapter.FidelityExecution},
		{selector.TierT1, adapter.FidelityExecution},
		// TierEmpty is reachable only from a usable map, so the map DID answer and its
		// answer was "nothing". Labelling it `none` would say "no evidence" about the one
		// outcome `warnings` already has to defend as a real result.
		{selector.TierEmpty, adapter.FidelityExecution},
		{selector.TierTS, adapter.FidelityStatic},
		// T2 is the full suite: nothing was derived, which is why you run everything.
		{selector.TierT2, adapter.FidelityNone},
	} {
		if got := selectionFidelity(tc.tier); got != tc.want {
			t.Errorf("selectionFidelity(%s) = %q, want %q", tc.tier, got, tc.want)
		}
	}
}

// "Never null" is not a convention here, it is the contract §6 states. A consumer that
// has to branch on null cannot calibrate on the field at all.
func TestSelectionFidelityIsNeverEmpty(t *testing.T) {
	for tier := selector.TierEmpty; tier <= selector.TierT2; tier++ {
		if selectionFidelity(tier) == "" {
			t.Errorf("selectionFidelity(%s) is empty", tier)
		}
	}
	if got := selectionFidelity(selector.Tier(99)); got != adapter.FidelityNone {
		t.Errorf("an unknown tier must fall back to %q, got %q", adapter.FidelityNone, got)
	}
}

// Never null, never empty, never any other string — over every tier the type can hold,
// including the ones no selector branch produces today. A tier added later that fell
// through to some fourth value would fail here rather than in a consumer.
func TestSelectionFidelityIsAlwaysOneOfTheThreeWireStrings(t *testing.T) {
	for tier := selector.Tier(-3); tier <= selector.TierT2+3; tier++ {
		if got := selectionFidelity(tier); !wireFidelities[got] {
			t.Errorf("selectionFidelity(%d) = %q, which is not one of the three wire strings", tier, got)
		}
	}
}

// The document itself, not just the helper: a field that marshals to `null` would satisfy
// every assertion above and still break the contract.
func TestJSONSelectionFidelityIsNeverNullForAnyTier(t *testing.T) {
	for tier := selector.TierEmpty; tier <= selector.TierT2; tier++ {
		out := BuildOutput(OutputInput{Command: "which", Sel: selector.Selection{Tier: tier}})
		if !wireFidelities[out.SelectionFidelity] {
			t.Errorf("tier %s: selection_fidelity = %q", tier, out.SelectionFidelity)
		}
		b, err := json.Marshal(out)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), `"selection_fidelity":null`) ||
			strings.Contains(string(b), `"selection_fidelity":""`) {
			t.Errorf("tier %s: %s", tier, b)
		}
	}
}

func TestJSONCarriesSelectionFidelityForEveryCommand(t *testing.T) {
	for _, cmd := range []string{"which", "run"} {
		out := BuildOutput(OutputInput{
			Command: cmd,
			Sel:     selector.Selection{Tier: selector.TierTS, Tests: []string{"a"}},
		})
		b, err := json.Marshal(out)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), `"selection_fidelity":"static"`) {
			t.Errorf("%s --json has no selection_fidelity: %s", cmd, b)
		}
	}
}

// A polyglot document splits per adapter, and two adapters can answer at two fidelities —
// a seeded Python block beside a Go block that can only ever be static. A single top-level
// value would label one of them wrongly.
func TestEachPerAdapterBlockCarriesItsOwnFidelity(t *testing.T) {
	out := BuildOutput(OutputInput{
		Command: "which",
		Sel:     selector.Selection{Tier: selector.TierT2},
		Blocks: []AdapterSelection{
			{Adapter: "python", Selection: selector.Selection{Tier: selector.TierT0}},
			{Adapter: "go", Selection: selector.Selection{Tier: selector.TierTS}},
		},
	})
	if len(out.Selections) != 2 {
		t.Fatalf("want 2 blocks, got %d", len(out.Selections))
	}
	if out.Selections[0].SelectionFidelity != adapter.FidelityExecution {
		t.Errorf("python block = %q", out.Selections[0].SelectionFidelity)
	}
	if out.Selections[1].SelectionFidelity != adapter.FidelityStatic {
		t.Errorf("go block = %q", out.Selections[1].SelectionFidelity)
	}
}

// The decided polyglot rule (docs/plans/00-interfaces.md): the flat value is the WEAKEST
// fidelity any answering adapter reported, because the flat `selection` it labels is the
// union of every block and a union is only as well-evidenced as its worst member.
//
// It falls out of the fold rather than being computed twice: foldBlocks already reports
// the WIDEST tier, and fidelity is monotonically non-increasing in tier breadth. The
// assertion is over the folded document, so a change to either half breaks it here.
func TestFlatSelectionFidelityIsTheWeakestOfTheAnsweringAdapters(t *testing.T) {
	for _, tc := range []struct {
		name  string
		tiers []selector.Tier
		want  adapter.Fidelity
	}{
		{"execution beside static reports static", []selector.Tier{selector.TierT0, selector.TierTS}, adapter.FidelityStatic},
		{"static beside execution reports static", []selector.Tier{selector.TierTS, selector.TierT0}, adapter.FidelityStatic},
		{"anything beside T2 reports none", []selector.Tier{selector.TierT0, selector.TierT2}, adapter.FidelityNone},
		{"static beside T2 reports none", []selector.Tier{selector.TierTS, selector.TierT2}, adapter.FidelityNone},
		{"two execution-derived blocks stay execution-derived", []selector.Tier{selector.TierT0, selector.TierT1}, adapter.FidelityExecution},
		{"an empty block does not weaken a mapped one", []selector.Tier{selector.TierEmpty, selector.TierT1}, adapter.FidelityExecution},
	} {
		t.Run(tc.name, func(t *testing.T) {
			blocks := make([]AdapterSelection, 0, len(tc.tiers))
			for i, tier := range tc.tiers {
				blocks = append(blocks, AdapterSelection{
					Adapter:   string(rune('a' + i)),
					Selection: selector.Selection{Tier: tier},
				})
			}
			folded, _, fallback, _ := foldBlocks(blocks)
			out := BuildOutput(OutputInput{
				Command: "which", Sel: folded, ImportFallback: fallback, Blocks: blocks,
			})
			if out.SelectionFidelity != tc.want {
				t.Errorf("flat selection_fidelity = %q, want %q", out.SelectionFidelity, tc.want)
			}
			weakest := adapter.FidelityExecution
			for _, blk := range out.Selections {
				if fidelityStrength(blk.SelectionFidelity) < fidelityStrength(weakest) {
					weakest = blk.SelectionFidelity
				}
			}
			if out.SelectionFidelity != weakest {
				t.Errorf("flat %q is not the weakest block value %q", out.SelectionFidelity, weakest)
			}
		})
	}
}

// fidelityStrength orders the three wire strings for the weakest-wins assertion above.
// It is test-only: the production rule reads the folded tier, and a second production
// path would be a second thing to keep in step.
func fidelityStrength(f adapter.Fidelity) int {
	switch f {
	case adapter.FidelityExecution:
		return 2
	case adapter.FidelityStatic:
		return 1
	default:
		return 0
	}
}

// Every shipped adapter has a committed fixture repository, and each one selects at TS —
// a `coverage: none` adapter can never report anything but `static`, whatever its
// language. AC3's first clause, over the whole shipped set rather than one example.
func TestEveryFixtureAdapterReportsAWireFidelityForItsSelection(t *testing.T) {
	all, err := adapter.Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	for _, fc := range shippedFixtures {
		t.Run(fc.fixture, func(t *testing.T) {
			root := fixtureRoot(t, fc.fixture)
			ads, derr := adapter.Detect(root, all)
			if derr != nil {
				t.Fatalf("Detect: %v", derr)
			}
			blocks, berr := selectPerAdapter(root, ads, mapstore.New(), mapstore.Meta{}, selectionContext{
				Changes: []gitctx.Change{{Path: fc.changed, Status: gitctx.Modified}},
				Cfg:     selector.DefaultConfig(),
			})
			if berr != nil {
				t.Fatalf("selectPerAdapter: %v", berr)
			}
			folded, _, fallback, _ := foldBlocks(blocks)
			out := BuildOutput(OutputInput{
				Command: "which", Sel: folded, ImportFallback: fallback, Blocks: blocks,
			})
			if !wireFidelities[out.SelectionFidelity] {
				t.Fatalf("selection_fidelity = %q", out.SelectionFidelity)
			}
			for _, blk := range out.Selections {
				if !wireFidelities[blk.SelectionFidelity] {
					t.Errorf("block %s: selection_fidelity = %q", blk.Adapter, blk.SelectionFidelity)
				}
			}
			// The adapter that owns the changed file answered statically, and its block
			// must say so — a `coverage: none` adapter has no other honest value.
			for _, blk := range blocks {
				if blk.Ad != nil && blk.Ad.IsInstrumentable(fc.changed) {
					if got := selectionFidelity(blk.Selection.Tier); got != adapter.FidelityStatic {
						t.Errorf("%s answered %s at fidelity %q, want %q",
							blk.Adapter, blk.Selection.Tier, got, adapter.FidelityStatic)
					}
				}
			}
		})
	}
}

// AC3, end to end over the CLI: a `coverage: none` adapter reports `static` on both
// --json surfaces. The document is what an agent front-end reads; a value only
// BuildOutput agrees with is a value the CLI could still fail to emit.
func TestStaticAdapterReportsStaticSelectionFidelityOnWhichJSON(t *testing.T) {
	dir := staticRunRepo(t)
	touchStaticSource(t, dir)

	code, stdout, stderr := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	got := decodeOutput(t, stdout)
	if got.SelectionFidelity != adapter.FidelityStatic {
		t.Errorf("which --json: selection_fidelity = %q, want %q (tier %s)",
			got.SelectionFidelity, adapter.FidelityStatic, got.Tier)
	}
}

// The same fact on the other --json surface. captureStdout, not the rtdd() helper:
// emitJSON writes the document to os.Stdout itself, which is what "--json is the WHOLE of
// stdout" means.
func TestStaticAdapterReportsStaticSelectionFidelityOnRunJSON(t *testing.T) {
	dir := staticRunRepo(t)
	touchStaticSource(t, dir)
	chdir(t, dir)

	code := -1
	stdout := captureStdout(t, func() { code = cmdRun([]string{"--json"}) })
	if code != 0 {
		t.Fatalf("rtdd run --json = %d, want 0\nstdout:\n%s", code, stdout)
	}
	got := decodeOutput(t, stdout)
	if got.SelectionFidelity != adapter.FidelityStatic {
		t.Errorf("run --json: selection_fidelity = %q, want %q (tier %s)",
			got.SelectionFidelity, adapter.FidelityStatic, got.Tier)
	}
}

// AC3: the Python coverage adapter, over a seeded map, reports `execution-derived` — the
// map rows it selected from were recorded during real execution.
func TestSeededPythonRepositoryReportsExecutionDerivedSelectionFidelity(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)
	writeFile(t, dir, "src/auth.py", "def login():\n    return True\n")

	code, stdout, stderr := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	got := decodeOutput(t, stdout)
	if got.SelectionFidelity != adapter.FidelityExecution {
		t.Errorf("selection_fidelity = %q, want %q (tier %s)",
			got.SelectionFidelity, adapter.FidelityExecution, got.Tier)
	}
}

// AC3: a repository where no adapter answered reports `none`. Nothing was derived — not
// from coverage, which there is none of, and not from a declaration, because there is no
// adapter to declare one.
func TestRepositoryWhereNoAdapterAnsweredReportsNoneSelectionFidelity(t *testing.T) {
	dir := newTestRepo(t)
	writeFile(t, dir, "src/auth.py", "def login():\n    return True\n")

	code, stdout, stderr := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	got := decodeOutput(t, stdout)
	if got.Adapter != "" {
		t.Fatalf("fixture resolved adapter %q; this test needs a repository with none", got.Adapter)
	}
	if got.SelectionFidelity != adapter.FidelityNone {
		t.Errorf("selection_fidelity = %q, want %q (tier %s)",
			got.SelectionFidelity, adapter.FidelityNone, got.Tier)
	}
}

// The new key must not cost the single-adapter document its determinism: two identical
// runs marshal to identical bytes, `selections` stays absent, and the emitted key order
// is the struct's.
func TestSingleAdapterDocumentStaysByteStableAcrossTwoIdenticalRuns(t *testing.T) {
	dir := newTestRepo(t)
	installRTDD(t, dir, headShort(t, dir), 0)
	writeFile(t, dir, "src/auth.py", "def login():\n    return True\n")

	_, first, _ := rtdd(t, dir, "which", "--json")
	_, second, _ := rtdd(t, dir, "which", "--json")
	if first != second {
		t.Fatalf("which --json is not byte-stable:\n%s\n%s", first, second)
	}
	if !strings.Contains(first, `"selection_fidelity": "execution-derived"`) {
		t.Errorf("document has no selection_fidelity: %s", first)
	}
	if strings.Contains(first, `"selections"`) {
		t.Errorf("a single-adapter document must not gain a selections key: %s", first)
	}
	// Field order is emitted key order, and fidelity qualifies the tier and reason it
	// sits beside.
	if strings.Index(first, `"reason"`) > strings.Index(first, `"selection_fidelity"`) {
		t.Errorf("selection_fidelity must follow reason: %s", first)
	}
}
