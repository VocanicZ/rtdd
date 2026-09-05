package selector

import (
	"os"
	"strings"
	"testing"
)

func TestTierString(t *testing.T) {
	tests := []struct {
		tier Tier
		want string
	}{
		{TierEmpty, "empty"},
		{TierDirect, "direct"},
		{TierT0, "T0"},
		{TierT1, "T1"},
		{TierT2, "T2"},
	}
	for _, tc := range tests {
		if got := tc.tier.String(); got != tc.want {
			t.Errorf("Tier(%d).String() = %q, want %q", int(tc.tier), got, tc.want)
		}
	}
}

// Every declared tier must have its own name: a fallthrough that returned "unknown" for a
// real tier would print a selection the agent cannot act on.
func TestTierStringCoversEveryDeclaredTier(t *testing.T) {
	for tier := TierEmpty; tier <= TierT2; tier++ {
		if got := tier.String(); got == "unknown" || got == "" {
			t.Errorf("Tier(%d).String() = %q, want a real tier name", int(tier), got)
		}
	}
	if got := Tier(99).String(); got != "unknown" {
		t.Errorf("Tier(99).String() = %q, want %q", got, "unknown")
	}
}

// TierEmpty must be the zero value so a Selection nobody filled in reads as "nothing
// selected" rather than as a successful tier.
func TestTierEmptyIsTheZeroValue(t *testing.T) {
	var s Selection
	if s.Tier != TierEmpty {
		t.Errorf("zero Selection.Tier = %v, want TierEmpty", s.Tier)
	}
	if s.Tier.String() != "empty" {
		t.Errorf("zero Selection.Tier.String() = %q, want %q", s.Tier.String(), "empty")
	}
}

func TestDefaultConfig(t *testing.T) {
	c := DefaultConfig()
	if c.StaleCommits != 50 {
		t.Errorf("StaleCommits = %d, want 50", c.StaleCommits)
	}
	if c.DriftGuard != 100 {
		t.Errorf("DriftGuard = %d, want 100", c.DriftGuard)
	}
	if c.HubThreshold != 0.40 {
		t.Errorf("HubThreshold = %v, want 0.40", c.HubThreshold)
	}
}

// A caller that never set Cfg must get the documented defaults, not a config that
// escalates on every commit (StaleCommits 0) and treats every file as a hub
// (HubThreshold 0).
func TestZeroConfigIsTreatedAsDefaultConfig(t *testing.T) {
	var zero Config
	if got := zero.orDefault(); got != DefaultConfig() {
		t.Errorf("zero Config.orDefault() = %+v, want %+v", got, DefaultConfig())
	}
}

// A partially-filled Config is the caller's deliberate choice and is left alone; only the
// all-zero value means "unset".
func TestPartialConfigIsLeftAlone(t *testing.T) {
	c := Config{StaleCommits: 7}
	if got := c.orDefault(); got != c {
		t.Errorf("Config{StaleCommits: 7}.orDefault() = %+v, want %+v", got, c)
	}
}

// The engine prints only from cmd/. A stray print inside selector would corrupt the
// --json surface every agent front-end depends on.
func TestPackagePrintsNothing(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	banned := []string{"fmt.Print", "fmt.Fprint", "println(", "print(", "os.Stdout", "os.Stderr", "log."}
	scanned := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		for _, bad := range banned {
			if strings.Contains(src, bad) {
				t.Errorf("%s contains %q: only cmd/ may write to stdout/stderr", name, bad)
			}
		}
		scanned++
	}
	if scanned == 0 {
		t.Fatal("scanned no non-test source files; the guard would pass vacuously")
	}
}

func TestTierTSString(t *testing.T) {
	if got := TierTS.String(); got != "TS" {
		t.Errorf("TierTS.String() = %q, want %q", got, "TS")
	}
}

// Spec §4.1 inserts TS BETWEEN T1 and T2, and the constants are that order. TierEmpty
// must survive the insertion as the zero value: a constant added in the wrong place
// renumbers it, and an unfilled Selection would then read as a successful tier.
func TestTierTSIsOrderedBetweenT1AndT2(t *testing.T) {
	if int(TierEmpty) != 0 {
		t.Fatalf("TierEmpty = %d, want 0 (the zero value)", int(TierEmpty))
	}
	if !(TierT1 < TierTS && TierTS < TierT2) {
		t.Errorf("order = T1:%d TS:%d T2:%d, want T1 < TS < T2",
			int(TierT1), int(TierTS), int(TierT2))
	}
}

// The static tier's two questions — "does this file exist" and "which tests import this
// file, how far away" — arrive as injected functions so Select stays pure.
func TestInputsCarriesTheStaticResolvers(t *testing.T) {
	in := Inputs{
		Exists:         func(rel string) bool { return rel == "src/a.test.ts" },
		ImportDistance: func(string) map[string]int { return map[string]int{"src/a.test.ts": 2} },
	}
	if !in.Exists("src/a.test.ts") || in.Exists("nope.ts") {
		t.Error("Inputs.Exists did not round-trip")
	}
	if got := in.ImportDistance("src/a.ts")["src/a.test.ts"]; got != 2 {
		t.Errorf("Inputs.ImportDistance(...)[src/a.test.ts] = %d, want 2", got)
	}
}
