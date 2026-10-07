package main

// v0.2 test helpers whose defining file went with a removed command (docs/plans/
// 10-rounds-cutover.md, "Old tests: who deletes what"). Task 7 deletes this file with
// the last caller.

import (
	"os"
	"testing"
)

// chdir moves into dir for the duration of the test. cmdSeed and cmdRun resolve
// the repo root from the working directory, exactly as the CLI does.
func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { os.Chdir(old) })
}
