package testenv

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func load(v float64) func() (float64, error) {
	return func() (float64, error) { return v, nil }
}

func env(vars map[string]string) func(string) string {
	return func(k string) string { return vars[k] }
}

// #514: the rules a wall-clock budget test is skipped by. A quiet host — load at or below
// its CPU count, no override — must never skip, or the budgets stop being enforced.
func TestSaturatedDecidesFromLoadCPUsAndTheOverride(t *testing.T) {
	unreadable := func() (float64, error) { return 0, errors.New("no /proc/loadavg") }
	for _, c := range []struct {
		name     string
		load     func() (float64, error)
		cpus     int
		env      map[string]string
		want     bool
		inReason string
	}{
		{"load above cpus skips", load(234), 64, nil, true, "load1=234"},
		{"load equal to cpus does not skip", load(64), 64, nil, false, ""},
		{"load below cpus does not skip", load(3), 64, nil, false, ""},
		{"an unreadable load does not skip", unreadable, 64, nil, false, ""},
		{"the override skips a quiet host", load(3), 64, map[string]string{Override: "1"}, true, Override},
		{"the override set to anything else does not skip", load(3), 64, map[string]string{Override: "0"}, false, ""},
		{"the override unset does not skip", load(3), 64, map[string]string{}, false, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, reason := saturated(c.load, c.cpus, env(c.env))
			if got != c.want {
				t.Fatalf("saturated = %v (%q), want %v", got, reason, c.want)
			}
			if !c.want {
				return
			}
			if !strings.HasPrefix(reason, "INCONCLUSIVE: ") {
				t.Errorf("reason %q does not start with INCONCLUSIVE:", reason)
			}
			if !strings.Contains(reason, c.inReason) {
				t.Errorf("reason %q does not name %q", reason, c.inReason)
			}
		})
	}
}

func TestParseLoad1ReadsTheFirstFieldOfProcLoadavg(t *testing.T) {
	got, err := parseLoad1("233.61 242.87 250.82 172/7976 178030\n")
	if err != nil {
		t.Fatal(err)
	}
	if got != 233.61 {
		t.Errorf("parseLoad1 = %v, want 233.61", got)
	}
	for _, bad := range []string{"", "\n", "busy 1 2 3/4 5\n"} {
		if _, err := parseLoad1(bad); err == nil {
			t.Errorf("parseLoad1(%q) returned no error", bad)
		}
	}
}

func TestSkipIfSaturatedSkipsUnderTheOverride(t *testing.T) {
	t.Setenv(Override, "1")
	ran := false
	t.Run("budget", func(t *testing.T) {
		SkipIfSaturated(t)
		ran = true
	})
	if ran {
		t.Error("SkipIfSaturated with the override set let the test body run")
	}
}

func TestReadLoad1ReadsThisHostsLoad(t *testing.T) {
	if _, err := os.Stat("/proc/loadavg"); err != nil {
		t.Skip("no /proc/loadavg on this host")
	}
	if got, err := readLoad1(); err != nil || got < 0 {
		t.Errorf("readLoad1 = %v, %v", got, err)
	}
}
