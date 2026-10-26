package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// Issue #505: two shell test scripts with stub functions (git, tmux, gh twice, a stub
// nested in a stub) run only as whole files. Editing the code they drive selects each
// script by its path, once — no `::git`, no `::tmux`, no `@line`.
func TestWhichWholeFileTestsListOnlyTheirPaths(t *testing.T) {
	fixture, err := filepath.Abs(filepath.Join("testdata", "bash-wholefile"))
	if err != nil {
		t.Fatal(err)
	}
	dir := gittest.Init(t)
	copyTree(t, fixture, dir)
	gittest.Commit(t, dir, "fixture")
	e2eEdit(t, dir, "lib/drive.sh", "  tmux new-session -d -s \"$1\"\n", "  tmux new-session -d -s \"$1\" -c \"$1\"\n")
	e2eEdit(t, dir, "lib/drive.sh", "--label bug", "--label bug --assignee me")
	w, text := runE2EWhich(t, dir)

	assertIDs(t, "changed_nodes", w.changed(), []string{"lib/drive.sh::spawn_impl", "lib/drive.sh::file_bug"})
	assertIDs(t, "Round 1", w.round(1), []string{"test/test_bug_flow.sh", "test/test_drive.sh"})
	assertIDs(t, "untested", w.Untested, []string{})
	for i := 1; i <= 2; i++ {
		for _, id := range w.round(i) {
			if strings.Contains(id, "::") || strings.Contains(id, "@") {
				t.Errorf("Round %d lists %q, which is not a runnable test file", i, id)
			}
		}
	}
	if !strings.Contains(text, "Round 1 — run these first:\n  test/test_bug_flow.sh\n  test/test_drive.sh\n") {
		t.Errorf("text Round 1 is not the two script paths:\n%s", text)
	}

	// Editing a script's top level or one of its stubs selects that script alone.
	if err := os.WriteFile(filepath.Join(dir, "lib", "drive.sh"), mustRead(t, filepath.Join(fixture, "lib", "drive.sh")), 0o644); err != nil {
		t.Fatal(err)
	}
	e2eEdit(t, dir, "test/test_drive.sh", "  echo \"tmux $*\" >>\"$LOG\"\n", "  echo \"tmux: $*\" >>\"$LOG\"\n")
	e2eEdit(t, dir, "test/test_bug_flow.sh", "test_files_a_bug && echo ok\n", "test_files_a_bug && echo pass\n")
	w, _ = runE2EWhich(t, dir)
	assertIDs(t, "changed_nodes", w.changed(), []string{"test/test_bug_flow.sh::<module>", "test/test_drive.sh::tmux"})
	assertIDs(t, "Round 1", w.round(1), []string{"test/test_bug_flow.sh", "test/test_drive.sh"})
	assertIDs(t, "untested", w.Untested, []string{})
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
