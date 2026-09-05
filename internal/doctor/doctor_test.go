package doctor

import (
	"reflect"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/mapstore"
)

func older(a, b string) string { return a }

func fixtureMap() *mapstore.Map {
	m := mapstore.New()
	m.Union(mapstore.Row{T: "tests/test_a.py::t1", F: []string{"src/hub.py", "src/a.py"}, C: "aaa", D: 10, S: "pass"}, older)
	m.Union(mapstore.Row{T: "tests/test_b.py::t2", F: []string{"src/hub.py", "src/b.py"}, C: "aaa", D: 20, S: "pass"}, older)
	m.Union(mapstore.Row{T: "tests/test_c.py::t3", F: []string{"src/hub.py"}, C: "aaa", D: 30, S: "pass"}, older)
	m.Union(mapstore.Row{T: "tests/test_d.py::t4", F: []string{"src/b.py"}, C: "aaa", D: 40, S: "pass"}, older)
	return m
}

func TestHubsRanksByDescendingTestCount(t *testing.T) {
	got := Hubs(fixtureMap())
	want := []Hub{
		{Path: "src/hub.py", TestCount: 3, Fraction: 0.75},
		{Path: "src/b.py", TestCount: 2, Fraction: 0.5},
		{Path: "src/a.py", TestCount: 1, Fraction: 0.25},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Hubs()\n got: %#v\nwant: %#v", got, want)
	}
}

func TestHubsTiesBreakByPathForDeterminism(t *testing.T) {
	m := mapstore.New()
	m.Union(mapstore.Row{T: "tests/t.py::t1", F: []string{"src/z.py", "src/a.py", "src/m.py"}, C: "aaa", D: 1, S: "pass"}, older)
	got := Hubs(m)
	want := []Hub{
		{Path: "src/a.py", TestCount: 1, Fraction: 1},
		{Path: "src/m.py", TestCount: 1, Fraction: 1},
		{Path: "src/z.py", TestCount: 1, Fraction: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Hubs()\n got: %#v\nwant: %#v", got, want)
	}
}

// An empty map must not divide by zero, and must not hand cmd/rtdd a nil slice to
// range over and then marshal as JSON `null` where `[]` is the contract.
func TestHubsEmptyMap(t *testing.T) {
	got := Hubs(mapstore.New())
	if got == nil {
		t.Fatal("Hubs() = nil, want an empty non-nil slice")
	}
	if len(got) != 0 {
		t.Fatalf("Hubs() = %#v, want empty", got)
	}
}

func TestCaveatNamesEveryOncePerProcessMechanism(t *testing.T) {
	for _, needle := range []string{
		"lru_cache",
		"module singleton",
		"DI container",
		"session-scoped fixture",
		"fan-out of 1",
		"most coupled",
		"cleanest",
	} {
		if !strings.Contains(Caveat, needle) {
			t.Fatalf("Caveat is missing %q; spec §9 requires the limitation be stated in the tool's own output.\nCaveat = %q", needle, Caveat)
		}
	}
}

// Spec §6: the static-tier caveat states BOTH halves — what a static selection can miss,
// and what passing one is worth. Either half alone reads as a failure or as a licence.
func TestStaticCaveatNamesTheMissAndTheWeakerEvidence(t *testing.T) {
	for _, needle := range []string{"static", "miss", "execution-derived", "weaker evidence"} {
		if !strings.Contains(StaticCaveat, needle) {
			t.Fatalf("StaticCaveat is missing %q:\n%s", needle, StaticCaveat)
		}
	}
}
