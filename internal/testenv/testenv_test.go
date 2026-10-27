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

func pressure(v float64) func() (float64, error) {
	return func() (float64, error) { return v, nil }
}

// #514, #540: the rules a wall-clock budget test is skipped by. A quiet host — CPU pressure
// below PSIThreshold, load at or below its CPU count, no override — must never skip, or the
// budgets stop being enforced.
func TestSaturatedDecidesFromPressureLoadCPUsAndTheOverride(t *testing.T) {
	unreadable := func() (float64, error) { return 0, errors.New("no /proc/loadavg") }
	noPSI := func() (float64, error) { return 0, errors.New("no /proc/pressure/cpu") }
	for _, c := range []struct {
		name     string
		psi      func() (float64, error)
		load     func() (float64, error)
		cpus     int
		env      map[string]string
		want     bool
		inReason string
	}{
		{"load above cpus skips", noPSI, load(234), 64, nil, true, "load1=234"},
		{"load equal to cpus does not skip", noPSI, load(64), 64, nil, false, ""},
		{"load below cpus does not skip", noPSI, load(3), 64, nil, false, ""},
		{"an unreadable load does not skip", noPSI, unreadable, 64, nil, false, ""},
		{"pressure at or above the threshold skips under NumCPU (#540)", pressure(24.01), load(60), 64, nil, true, "avg10=24.01"},
		{"pressure below the threshold does not skip", pressure(0.5), load(3), 64, nil, false, ""},
		{"pressure exactly at the threshold skips", pressure(PSIThreshold), load(3), 64, nil, true, "avg10=10.00"},
		{"pressure just below the threshold does not skip", pressure(PSIThreshold - 0.01), load(3), 64, nil, false, ""},
		{"low pressure with load above cpus skips by load", pressure(0.5), load(234), 64, nil, true, "load1=234"},
		{"unreadable pressure and load do not skip", noPSI, unreadable, 64, nil, false, ""},
		{"pressure skips even when load is unreadable", pressure(30), unreadable, 64, nil, true, "avg10=30.00"},
		{"the override skips a quiet host", pressure(0), load(3), 64, map[string]string{Override: "1"}, true, Override},
		{"the override set to anything else does not skip", pressure(0), load(3), 64, map[string]string{Override: "0"}, false, ""},
		{"the override unset does not skip", pressure(0), load(3), 64, map[string]string{}, false, ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, reason := saturated(c.psi, c.load, c.cpus, env(c.env))
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

// #540: the `some` line's avg10 — the share of the last 10 s a runnable task waited for a CPU.
func TestParseCPUPressureReadsSomeAvg10OfProcPressureCPU(t *testing.T) {
	got, err := parseCPUPressure("some avg10=24.01 avg60=25.25 avg300=31.12 total=6667130327\nfull avg10=0.00 avg60=0.00 avg300=0.00 total=0\n")
	if err != nil {
		t.Fatal(err)
	}
	if got != 24.01 {
		t.Errorf("parseCPUPressure = %v, want 24.01", got)
	}
	for _, bad := range []string{
		"",
		"\n",
		"full avg10=0.00 avg60=0.00 avg300=0.00 total=0\n",
		"some avg60=25.25 avg300=31.12 total=6667130327\n",
		"some avg10=busy avg60=25.25 avg300=31.12 total=6667130327\n",
		"some avg10= avg60=25.25\n",
	} {
		if _, err := parseCPUPressure(bad); err == nil {
			t.Errorf("parseCPUPressure(%q) returned no error", bad)
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

func TestReadCPUPressureReadsThisHostsPressure(t *testing.T) {
	if _, err := os.ReadFile("/proc/pressure/cpu"); err != nil {
		t.Skipf("no readable /proc/pressure/cpu on this host: %v", err)
	}
	if got, err := readCPUPressure(); err != nil || got < 0 || got > 100 {
		t.Errorf("readCPUPressure = %v, %v", got, err)
	}
}
