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

func TestBlobIDsChangeOnlyWhenACommitChangesTheFile(t *testing.T) {
	dir := gittest.Init(t)
	gittest.Write(t, dir, "a.py", "def a():\n    pass\n")
	gittest.Write(t, dir, "lib/b.py", "def b():\n    pass\n")
	gittest.Commit(t, dir, "one")
	first, err := gitctx.BlobIDs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 2 || len(first["a.py"]) != 40 || len(first["lib/b.py"]) != 40 {
		t.Fatalf("BlobIDs = %v, want 40-hex ids for a.py and lib/b.py", first)
	}
	gittest.Write(t, dir, "a.py", "def a():\n    return 1\n")
	if again, _ := gitctx.BlobIDs(dir); !reflect.DeepEqual(again, first) {
		t.Errorf("an uncommitted edit changed BlobIDs: %v -> %v (HEAD's tree is the source)", first, again)
	}
	gittest.Commit(t, dir, "two")
	second, _ := gitctx.BlobIDs(dir)
	if second["a.py"] == first["a.py"] || second["lib/b.py"] != first["lib/b.py"] {
		t.Errorf("after committing a.py: %v -> %v; want only a.py's id to change", first, second)
	}
}

func TestBlobIDsOnAnUnbornHeadIsEmpty(t *testing.T) {
	dir := gittest.Init(t)
	got, err := gitctx.BlobIDs(dir)
	if err != nil || len(got) != 0 {
		t.Errorf("BlobIDs(unborn) = %v, %v; want empty, nil", got, err)
	}
}
