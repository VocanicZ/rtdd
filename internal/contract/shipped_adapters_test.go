package contract

import (
	"io/fs"
	"reflect"
	"regexp"
	"strings"
	"testing"

	rtddadapters "github.com/VocanicZ/rtdd/adapters"
	"github.com/VocanicZ/rtdd/internal/adapter"
)

// The completeness gate for the shipped adapter set (PRD #232 AC2, spec §8): a future
// adapter cannot ship half-specified.
//
// It walks the EMBEDDED filesystem rather than a hand-written list of adapter names. A
// list is exactly the hole this test exists to close: an adapter dropped into adapters/
// and forgotten in the list would ship unchecked, which is the failure, not a slip.
//
// The required set is conditioned on the adapter's declared report mode, and the reason
// is the freeze: adapters/python.yaml is BYTE-FROZEN by
// internal/contract/adapter_freeze_test.go (spec §4.1 makes a seeded Python repo's
// selection byte-identical to today a requirement), and it declares no report_path, no
// id_template and no test_for. A flat "every adapter declares every key" rule is
// therefore unsatisfiable without editing the very file the digest exists to hold still.
// So the universally-required keys are asserted for EVERY embedded adapter, python
// included, and the junit-xml companion keys are asserted for exactly the adapters that
// declare `report: junit-xml`. The asymmetry is the freeze, not an oversight, and it is
// not to be "fixed".
func TestEveryShippedAdapterIsFullySpecified(t *testing.T) {
	all, err := adapter.Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}

	// Builtin must have read every *.yaml the binary embeds. Counting them here is what
	// makes the walk an enumeration rather than a sample: a file that loaded into
	// nothing, or a reader that learned to skip one, is caught before the keys are.
	yamls, err := fs.Glob(rtddadapters.FS, "*.yaml")
	if err != nil {
		t.Fatalf("glob embedded adapters: %v", err)
	}
	if len(yamls) == 0 {
		t.Fatal("the embedded adapter FS holds no *.yaml; the shipped binary would resolve no adapter at all")
	}
	if len(all) != len(yamls) {
		t.Fatalf("Builtin returned %d adapters but the embedded FS holds %d *.yaml files (%v)", len(all), len(yamls), yamls)
	}

	for _, a := range all {
		t.Run(a.Name, func(t *testing.T) {
			for _, line := range missingKeys(a) {
				t.Error(line)
			}
			if a.Report != "junit-xml" {
				return
			}
			// A junit-xml adapter is the static tier (plan 06-m6d decision 2): it
			// declares no seed and records no coverage, so an adapter that claims the
			// report but not the tier would select from a map it never writes.
			if a.Selection != adapter.SelectionStatic {
				t.Errorf("%s (%s): selection = %q, want %q", a.Name, a.Src, a.Selection, adapter.SelectionStatic)
			}
			if a.Coverage != adapter.CoverageNone {
				t.Errorf("%s (%s): coverage = %q, want %q", a.Name, a.Src, a.Coverage, adapter.CoverageNone)
			}
			if a.Fidelity() != adapter.FidelityStatic {
				t.Errorf("%s (%s): Fidelity() = %q, want %q; an adapter that can select nothing is not worth shipping", a.Name, a.Src, a.Fidelity(), adapter.FidelityStatic)
			}
		})
	}
}

// The gate is only worth what its failure message is worth: an implementer adding an
// adapter must read WHAT to add, not that something is wrong. This is the test of the
// test — it pins the message shape against a hand-built adapter with known holes, so the
// wording cannot rot unnoticed while every shipped adapter happens to be complete.
func TestMissingKeysNamesTheAdapterAndTheKey(t *testing.T) {
	holed := &adapter.Adapter{
		Name:   "acme",
		Src:    "acme.yaml",
		Detect: []string{"acme.toml"},
		// subset, list and test_globs are omitted.
		SourceGlobs:  []string{"**/*.acme"},
		Opaque:       []string{"**/*.json"},
		FullEscalate: []string{"acme.lock"},
		Report:       "junit-xml",
		ReportPath:   "target/acme.xml",
		// id_template and test_for are omitted.
	}

	want := []string{
		"acme (acme.yaml): subset is empty; every shipped adapter must declare it",
		"acme (acme.yaml): list is empty; every shipped adapter must declare it",
		"acme (acme.yaml): test_globs is empty; every shipped adapter must declare it",
		"acme (acme.yaml): id_template is empty; report: junit-xml requires it",
		"acme (acme.yaml): test_for is empty; report: junit-xml requires at least one template",
	}
	if got := missingKeys(holed); !reflect.DeepEqual(got, want) {
		t.Errorf("missingKeys =\n%s\nwant\n%s", join(got), join(want))
	}

	// A complete adapter produces no lines at all, so a green gate is silence rather
	// than a message nobody reads.
	complete := *holed
	complete.Subset = "acme test {tests}"
	complete.List = "acme list"
	complete.TestGlobs = []string{"**/*_test.acme"}
	complete.IDTemplate = "{classname}::{name}"
	complete.TestFor = []string{"{dir}/{name}_test.acme"}
	if got := missingKeys(&complete); len(got) != 0 {
		t.Errorf("missingKeys on a complete adapter = %v, want none", got)
	}
}

// missingKeys returns one line per key this adapter leaves empty — the adapter's name,
// the file it was read from, the key, and why that key is required. One line per omission
// so an adapter with three holes reads as three fixes, and the file name is there because
// the fix is always an edit to that YAML, never a weakened assertion here.
//
// The universal block is every key the engine needs from any adapter whatsoever. The
// junit-xml block is the companion set spec §4.3 requires of a report the engine has to
// locate and whose ids have to round-trip back into `subset`; adapters/python.yaml
// declares none of it and is byte-frozen, which is why the split exists at all — see the
// comment on TestEveryShippedAdapterIsFullySpecified.
func missingKeys(a *adapter.Adapter) []string {
	var out []string
	universal := "; every shipped adapter must declare it"
	for _, k := range []struct {
		key   string
		empty bool
	}{
		{"name", a.Name == ""},
		{"detect", len(a.Detect) == 0},
		{"subset", a.Subset == ""},
		{"list", a.List == ""},
		{"test_globs", len(a.TestGlobs) == 0},
		{"source_globs", len(a.SourceGlobs) == 0},
		{"opaque", len(a.Opaque) == 0},
		{"full_escalate", len(a.FullEscalate) == 0},
	} {
		if k.empty {
			out = append(out, line(a, k.key, universal))
		}
	}
	if a.Report != "junit-xml" {
		return out
	}
	if a.ReportPath == "" {
		out = append(out, line(a, "report_path", "; report: junit-xml requires it"))
	}
	if a.IDTemplate == "" {
		out = append(out, line(a, "id_template", "; report: junit-xml requires it"))
	}
	if len(a.TestFor) == 0 {
		out = append(out, line(a, "test_for", "; report: junit-xml requires at least one template"))
	}
	return out
}

func line(a *adapter.Adapter, key, why string) string {
	return a.Name + " (" + a.Src + "): " + key + " is empty" + why
}

func join(lines []string) string {
	if len(lines) == 0 {
		return "  (none)"
	}
	return "  " + strings.Join(lines, "\n  ")
}

// The freeze is a global constraint of the M6d plan, and a global constraint nothing
// checks is a paragraph. If a later task "adds the new keys" to the Python adapter, the
// digest test fires — and this test says WHY, so the fix is reverting the edit rather than
// updating the constant.
func TestNoShippedAdapterEditIsAllowedToTouchThePythonAdapter(t *testing.T) {
	src := readRepoFile(t, "internal/contract/adapter_freeze_test.go")
	const want = `pythonAdapterSHA256 = "a20c009fed0db285f4cfe04d0bbb1752fb6e0beca9a469736cb5f0eedf3df123"`
	if !strings.Contains(src, want) {
		t.Errorf("the python adapter digest changed; PRD #232 licenses no edit to adapters/python.yaml (M6d global constraints)")
	}
	yaml := readRepoFile(t, "adapters/python.yaml")
	for _, key := range []string{"test_flag:", "test_join:", "report_cmd:", "test_selector:"} {
		if strings.Contains(yaml, "\n"+key) {
			t.Errorf("adapters/python.yaml declares %q; every M6d key is optional and python declares none", key)
		}
	}
}

// PRD #232 AC5, stated once more where a reviewer will look for it: the v1 refusal is not
// in the tree, and no adapter YAML re-introduces the one-adapter assumption by claiming a
// marker that belongs to another ecosystem.
func TestNoTwoShippedAdaptersShareADetectMarker(t *testing.T) {
	src := readRepoFile(t, "docs/plans/06-m6d-shipped-adapters.md")
	if !regexp.MustCompile(`(?m)^\| ` + "`vitest`" + ` \|`).MatchString(src) {
		t.Fatal("the plan's decision-1 marker table is missing; the shipped set's detection rule has no written source")
	}
	// The real assertion is over the loaded adapters, not the plan: two adapters sharing a
	// marker makes every repo of that ecosystem detect both, which is decision 1's defect.
	seen := map[string]string{}
	all := builtinForTest(t)
	for _, a := range all {
		for _, m := range a.Detect {
			if prev, dup := seen[m]; dup {
				t.Errorf("adapters %s and %s both detect on %q; every repo of that ecosystem would match both (decision 1)", prev, a.Name, m)
			}
			seen[m] = a.Name
		}
	}
}

// builtinForTest is the embedded adapter set, or a failed test. Every guard in this file
// asserts over what the shipped binary would actually resolve, so a loader error is a
// fatal, not a skipped assertion.
func builtinForTest(t *testing.T) []*adapter.Adapter {
	t.Helper()
	all, err := adapter.Builtin()
	if err != nil {
		t.Fatalf("adapter.Builtin: %v", err)
	}
	return all
}
