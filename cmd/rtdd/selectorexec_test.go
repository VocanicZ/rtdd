package main

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/selector"
)

// PRD #334 AC3, and the one place in this milestone where a real runner is invoked.
//
// The Go toolchain is the only one of the nine already on the machine that runs this
// suite, so it is the only adapter whose selection can be proven end to end without
// installing anything. `go test -json` is also where the defect was reported from: the TS
// selection named `calc/calc_test.go`, `subset` spliced it into `-run`, and `go test -run
// 'calc/calc_test.go' ./...` printed "no tests to run" and exited 0.
//
// Both halves are asserted. The fixed invocation must actually run TestAdd, and the shape
// the adapter USED to build must still match nothing — otherwise a future edit could make
// the first assertion pass for a reason that has nothing to do with #334.
func TestGoFixtureSelectionActuallyExecutesTestAdd(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skipf("go is not on PATH: %v", err)
	}
	all, err := adapter.Builtin()
	if err != nil {
		t.Fatalf("Builtin: %v", err)
	}
	root := fixtureRoot(t, "go")
	ads, err := adapter.Detect(root, all)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	ad := adapterNamed(t, ads, "go")

	sel := whichSelectionForFixture(t, root, ads, "calc/calc.go")
	if sel.Tier != selector.TierTS {
		t.Fatalf("tier = %s, want TS; %s", sel.Tier, sel.Reason)
	}
	selectors, err := ad.Selectors(sel.Tests)
	if err != nil {
		t.Fatalf("Selectors(%v): %v", sel.Tests, err)
	}
	argv, err := ad.ExpandTests(ad.Subset, map[string]string{}, selectors)
	if err != nil {
		t.Fatalf("ExpandTests: %v", err)
	}

	ran, out := goTestJSONRan(t, root, argv)
	if !ran["TestAdd"] {
		t.Fatalf("the selection for calc/calc.go produced %v, whose invocation %v ran %v — TestAdd never executed;\n%s",
			sel.Tests, argv, keysOf(ran), out)
	}

	// The pre-#334 invocation, built the way the adapter used to build it: a test FILE
	// path spliced into `-run`, which takes a regex over test NAMES.
	broken := []string{"go", "test", "-json", "-run", sel.Tests[0], "./..."}
	brokenRan, brokenOut := goTestJSONRan(t, root, broken)
	if brokenRan["TestAdd"] {
		t.Errorf("%v ran TestAdd; the regression this test guards cannot be reproduced, so the guard proves nothing\n%s", broken, brokenOut)
	}
}

// goTestJSONRan runs one `go test -json` invocation in dir and returns the set of test
// names that actually produced a run event, plus the raw output for a failure message.
//
// The exit code is deliberately ignored: "no tests to run" exits 0, which is the whole
// point — a green exit is not evidence that anything executed.
func goTestJSONRan(t *testing.T, dir string, argv []string) (map[string]bool, string) {
	t.Helper()
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	out, _ := cmd.CombinedOutput()

	ran := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var ev struct {
			Action string
			Test   string
		}
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		if ev.Test == "" {
			continue
		}
		switch ev.Action {
		case "pass", "fail":
			ran[ev.Test] = true
		}
	}
	return ran, string(out)
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
