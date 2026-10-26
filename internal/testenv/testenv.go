// Package testenv holds helpers for tests whose verdict depends on the host they run on.
//
// It is a non-test package so tests in any package (internal or external) can import it,
// and it imports only the standard library: nothing outside a _test.go file may use it.
package testenv

import (
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// Override forces SkipIfSaturated to skip when set to "1", whatever the load.
const Override = "RTDD_PERF_INCONCLUSIVE"

// SkipIfSaturated skips a wall-clock budget test when the host is oversubscribed: a
// timing taken while more runnable work than CPUs competes for the box is not a
// measurement of the code, so it can neither pass nor fail a budget (#514). On a quiet
// host, or where the load cannot be read, it does nothing and the budget is enforced.
func SkipIfSaturated(t testing.TB) {
	t.Helper()
	if skip, reason := saturated(readLoad1, runtime.NumCPU(), os.Getenv); skip {
		t.Skip(reason)
	}
}

// saturated decides whether to skip, and says why in a message starting INCONCLUSIVE:.
// The host is saturated when the 1-minute load average exceeds cpus. A load that cannot
// be read is not evidence of saturation.
func saturated(readLoad func() (float64, error), cpus int, getenv func(string) string) (bool, string) {
	if getenv(Override) == "1" {
		return true, fmt.Sprintf("INCONCLUSIVE: %s=1 set; budget not measured", Override)
	}
	load1, err := readLoad()
	if err != nil || load1 <= float64(cpus) {
		return false, ""
	}
	return true, fmt.Sprintf("INCONCLUSIVE: host saturated (load1=%.0f, cpus=%d); budget not measured", load1, cpus)
}

func readLoad1() (float64, error) {
	b, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, err
	}
	return parseLoad1(string(b))
}

// parseLoad1 returns the first field of a /proc/loadavg line: the 1-minute load average.
func parseLoad1(line string) (float64, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return 0, fmt.Errorf("empty /proc/loadavg")
	}
	return strconv.ParseFloat(fields[0], 64)
}
