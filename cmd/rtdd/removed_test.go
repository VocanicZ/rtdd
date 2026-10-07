package main

import (
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// removedInV030 are the coverage-pipeline commands v0.3.0 removes (spec §8).
var removedInV030 = [][]string{{"seed"}, {"run"}, {"verify"}, {"status"}, {"map", "compact"}}

// PRD #410 AC5, AC6: invoking a removed command is a usage error (2) that names the
// release that removed it and what replaces it, and changes nothing.
func TestRemovedCommandsExit2NamingV030(t *testing.T) {
	dir := graphRepo(t)
	for _, args := range removedInV030 {
		code, out, errOut := rtdd(t, dir, args...)
		name := strings.Join(args, " ")
		if code != 2 {
			t.Errorf("rtdd %s: exit %d, want 2", name, code)
		}
		if out != "" {
			t.Errorf("rtdd %s wrote to stdout: %q", name, out)
		}
		for _, want := range []string{"rtdd " + name, "removed in v0.3.0", "rtdd which"} {
			if !strings.Contains(errOut, want) {
				t.Errorf("rtdd %s: stderr %q does not contain %q", name, errOut, want)
			}
		}
	}
	if _, err := os.Stat(dir + "/.rtdd"); err == nil {
		t.Errorf("a removed command created .rtdd/")
	}
}

// PRD #410 AC5: --help lists none of them, and no exit code is a test result.
func TestHelpListsNoRemovedCommand(t *testing.T) {
	_, help, _ := rtdd(t, graphRepo(t), "--help")
	for _, gone := range []string{"rtdd seed", "rtdd run", "rtdd verify", "rtdd status", "rtdd map", "a test failed"} {
		if strings.Contains(help, gone) {
			t.Errorf("--help still mentions %q:\n%s", gone, help)
		}
	}
}

// PRD #410 AC5: no source file in this package implements a removed command.
func TestNoSourceImplementsARemovedCommand(t *testing.T) {
	pkgs, err := parser.ParseDir(token.NewFileSet(), ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range pkgs {
		for name, f := range pkg.Files {
			for _, d := range f.Scope.Objects {
				switch d.Name {
				case "cmdSeed", "cmdRun", "cmdVerify", "cmdStatus", "cmdMap", "cmdMapCompact":
					t.Errorf("%s still declares %s", name, d.Name)
				}
			}
		}
	}
}

// rtdd init's closing line no longer sends anyone to a removed command.
func TestInitClosesByPointingAtWhichNotSeed(t *testing.T) {
	dir := newDetectableRepo(t)
	code, out, errOut := rtdd(t, dir, "init")
	if code != 0 {
		t.Fatalf("rtdd init = %d, stderr %q", code, errOut)
	}
	if !strings.Contains(out, "Next: edit code, then run `rtdd which` for the tests to run, in rounds.") || strings.Contains(out, "rtdd seed") {
		t.Errorf("rtdd init's closing line still points at a removed command, or not at rtdd which:\n%s", out)
	}
}
