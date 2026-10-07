package gitctx_test

import (
	"reflect"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

func TestDiffNamesSinceCoversCommittedStagedUnstagedAndDeleted(t *testing.T) {
	dir := gittest.Init(t)
	for _, f := range []string{"committed.py", "staged.py", "unstaged.py", "deleted.py", "same.py"} {
		gittest.Write(t, dir, f, "x = 1\n")
	}
	base := gittest.Commit(t, dir, "base")
	gittest.Write(t, dir, "committed.py", "x = 2\n")
	gittest.Commit(t, dir, "later")
	gittest.Write(t, dir, "staged.py", "x = 2\n")
	gittest.Run(t, dir, "add", "staged.py")
	gittest.Write(t, dir, "unstaged.py", "x = 2\n")
	gittest.Run(t, dir, "rm", "-q", "deleted.py")
	gittest.Write(t, dir, "untracked.py", "x = 1\n")

	got, err := gitctx.DiffNamesSince(dir, base)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"committed.py", "deleted.py", "staged.py", "unstaged.py"}; !reflect.DeepEqual(got, want) {
		t.Errorf("DiffNamesSince = %v, want %v", got, want)
	}
}
