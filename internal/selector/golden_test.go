package selector

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx"
)

// -update regenerates the golden. It is run ONCE, in this task, to capture behaviour as
// it stands before the static tier exists. A later change that finds the golden moved has
// found a regression, not a stale file: spec §4.1 makes a seeded repository's selection
// byte-identical a requirement, so the fix is the code, never this file.
var update = flag.Bool("update", false, "rewrite testdata/selection_golden.txt")

type goldenCase struct {
	name   string
	mutate func(*Inputs)
}

// seededCases covers every shape the existing fixtures reach with a SEEDED map: the
// direct tier, T0, each T1 escalation, each T2 escalation, and the explicit empty.
func seededCases() []goldenCase {
	return []goldenCase{
		{"direct only, brand new test file", func(in *Inputs) {
			in.Changes = []gitctx.Change{added("tests/test_brand_new.py")}
		}},
		{"direct before mapped", func(in *Inputs) {
			in.Changes = []gitctx.Change{mod("src/auth.py"), mod("tests/test_db.py")}
		}},
		{"deleted test file is not run", func(in *Inputs) {
			in.Changes = []gitctx.Change{deleted("tests/test_auth.py")}
		}},
		{"T0 one source file", func(in *Inputs) {
			in.Changes = []gitctx.Change{mod("src/auth.py")}
		}},
		{"T0 two source files", func(in *Inputs) {
			in.Changes = []gitctx.Change{mod("src/auth.py"), mod("src/db.py")}
		}},
		{"T0 through a rename's old path", func(in *Inputs) {
			in.Changes = []gitctx.Change{{
				Path: "src/auth2.py", OldPath: "src/auth.py", Status: gitctx.Renamed,
			}}
		}},
		{"empty, nothing covers the change", func(in *Inputs) {
			in.Changes = []gitctx.Change{mod("src/untouched.py")}
		}},
		{"T1 merge commit", func(in *Inputs) {
			in.Changes = []gitctx.Change{mod("src/auth.py")}
			in.Merge = true
		}},
		{"T1 opaque file", func(in *Inputs) {
			in.Changes = []gitctx.Change{mod("templates/page.html")}
		}},
		{"T1 stale row", func(in *Inputs) {
			in.Changes = []gitctx.Change{mod("src/auth.py")}
			in.Distance = func(string) int { return 999 }
		}},
		{"T2 full-escalate file", func(in *Inputs) {
			in.Changes = []gitctx.Change{mod("requirements.txt")}
		}},
		{"T2 drift guard", func(in *Inputs) {
			in.Changes = []gitctx.Change{mod("src/auth.py")}
			in.Cycles = 100
		}},
	}
}

func renderSelection(name string, s Selection) string {
	var b strings.Builder
	fmt.Fprintf(&b, "== %s\n", name)
	fmt.Fprintf(&b, "tier:   %s\n", s.Tier)
	fmt.Fprintf(&b, "direct: %s\n", strings.Join(s.Direct, " "))
	fmt.Fprintf(&b, "tests:  %s\n", strings.Join(s.Tests, " "))
	fmt.Fprintf(&b, "reason: %s\n\n", s.Reason)
	return b.String()
}

// The whole Selection is compared, not a field of it: a regression that kept the tier and
// reordered the ranked list would pass any spot check and would still change what an
// agent runs first.
func TestSeededSelectionIsByteIdentical(t *testing.T) {
	var b strings.Builder
	for _, tc := range seededCases() {
		in := baseInputs()
		in.AllTests = []string{
			"tests/test_auth.py::test_login",
			"tests/test_auth.py::test_logout",
			"tests/test_db.py::test_query",
			"tests/test_render.py::test_page",
		}
		tc.mutate(&in)
		b.WriteString(renderSelection(tc.name, Select(in)))
	}
	got := b.String()

	golden := filepath.Join("testdata", "selection_golden.txt")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("reading the golden: %v (generate it once with -update)", err)
	}
	if got != string(want) {
		t.Errorf("a seeded repository's selection moved; spec §4.1 forbids it.\n"+
			"--- got ---\n%s--- want ---\n%s", got, want)
	}
}
