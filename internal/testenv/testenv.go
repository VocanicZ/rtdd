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

// PSIThreshold is the CPU pressure, `some avg10` of /proc/pressure/cpu, at and above which
// the host counts as contended: a runnable task waited for a CPU at least this percent of
// the last 10 s. A quiet box reads about 0; the oversubscribed LXC host of #540 reads 25–30
// while its load stays below runtime.NumCPU(), which there counts host threads, not quota.
const PSIThreshold = 10.0

// SkipIfSaturated skips a wall-clock budget test when the host is oversubscribed: a
// timing taken while runnable work waits for a CPU is not a measurement of the code, so
// it can neither pass nor fail a budget (#514). The host counts as oversubscribed when its
// CPU pressure (PSI) reaches PSIThreshold, or its 1-minute load exceeds its CPU count
// (#540). On a quiet host, or where neither can be read, it does nothing and the budget
// is enforced.
func SkipIfSaturated(t testing.TB) {
	t.Helper()
	if skip, reason := saturated(readCPUPressure, readLoad1, runtime.NumCPU(), os.Getenv); skip {
		t.Skip(reason)
	}
}

// saturated decides whether to skip, and says why in a message starting INCONCLUSIVE:.
// In order: the override skips; CPU pressure `some avg10` at or above PSIThreshold skips;
// a 1-minute load average above cpus skips. Pressure is read first because it holds where
// load cannot: inside a container cpus overstates the CPUs the quota allows (#540). A
// pressure or load that cannot be read is not evidence of saturation.
func saturated(readPressure, readLoad func() (float64, error), cpus int, getenv func(string) string) (bool, string) {
	if getenv(Override) == "1" {
		return true, fmt.Sprintf("INCONCLUSIVE: %s=1 set; budget not measured", Override)
	}
	if avg10, err := readPressure(); err == nil && avg10 >= PSIThreshold {
		return true, fmt.Sprintf("INCONCLUSIVE: host contended (cpu some avg10=%.2f >= %g); budget not measured", avg10, PSIThreshold)
	}
	load1, err := readLoad()
	if err != nil || load1 <= float64(cpus) {
		return false, ""
	}
	return true, fmt.Sprintf("INCONCLUSIVE: host saturated (load1=%.0f, cpus=%d); budget not measured", load1, cpus)
}

// readCPUPressure reads the host's CPU pressure. A missing file, or EOPNOTSUPP on a kernel
// without PSI, is an error, never zero pressure.
func readCPUPressure() (float64, error) {
	b, err := os.ReadFile("/proc/pressure/cpu")
	if err != nil {
		return 0, err
	}
	return parseCPUPressure(string(b))
}

// parseCPUPressure returns avg10 of the `some` line of /proc/pressure/cpu: the percent of
// the last 10 s at least one runnable task waited for a CPU.
func parseCPUPressure(text string) (float64, error) {
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "some" {
			continue
		}
		for _, f := range fields[1:] {
			if v, ok := strings.CutPrefix(f, "avg10="); ok {
				return strconv.ParseFloat(v, 64)
			}
		}
		return 0, fmt.Errorf("no avg10= in the some line of /proc/pressure/cpu: %q", line)
	}
	return 0, fmt.Errorf("no some line in /proc/pressure/cpu")
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
