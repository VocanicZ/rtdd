package gitctx

import (
	"reflect"
	"sort"
	"testing"
)

// byPath indexes a ChangedSet result for assertions.
func byPath(cs []Change) map[string]Change {
	out := make(map[string]Change, len(cs))
	for _, c := range cs {
		out[c.Path] = c
	}
	return out
}

func paths(cs []Change) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.Path)
	}
	sort.Strings(out)
	return out
}

// The regression that killed v1: a test file the agent just wrote and never added is
// invisible to `git diff`, so it was in no tier and never ran.
func TestChangedSetIncludesUntrackedFiles(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "src/auth.py", "def login():\n    return 1\n")
	commit(t, dir, "init")

	write(t, dir, "tests/test_auth.py", "def test_login():\n    assert login()\n")

	cs, err := ChangedSet(dir, "HEAD")
	if err != nil {
		t.Fatalf("ChangedSet: %v", err)
	}
	got := byPath(cs)
	c, ok := got["tests/test_auth.py"]
	if !ok {
		t.Fatalf("ChangedSet omitted the untracked file; got %v", paths(cs))
	}
	if c.Status != Added {
		t.Errorf("Status = %v, want added (ChangedSet reports untracked files as Added)", c.Status)
	}
	if !reflect.DeepEqual(c.Lines, []LineRange{{Start: 1, End: 2}}) {
		t.Errorf("Lines = %#v, want the whole file as one range {1,2}", c.Lines)
	}
}

func TestChangedSetRetainsDeletions(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "src/legacy.py", "x = 1\n")
	write(t, dir, "src/keep.py", "y = 1\n")
	commit(t, dir, "init")

	remove(t, dir, "src/legacy.py")

	cs, err := ChangedSet(dir, "HEAD")
	if err != nil {
		t.Fatalf("ChangedSet: %v", err)
	}
	c, ok := byPath(cs)["src/legacy.py"]
	if !ok {
		t.Fatalf("ChangedSet dropped the deleted path; got %v", paths(cs))
	}
	if c.Status != Deleted {
		t.Errorf("Status = %v, want deleted", c.Status)
	}
	if len(c.Lines) != 0 {
		t.Errorf("Lines = %#v, want empty for a deletion", c.Lines)
	}
}

func TestChangedSetHandlesRenames(t *testing.T) {
	dir := newRepo(t)
	body := "def a():\n    return 1\n\ndef b():\n    return 2\n\ndef c():\n    return 3\n"
	write(t, dir, "src/old.py", body)
	commit(t, dir, "init")

	remove(t, dir, "src/old.py")
	write(t, dir, "src/new.py", body)
	gitRun(t, dir, "add", "-A")

	cs, err := ChangedSet(dir, "HEAD")
	if err != nil {
		t.Fatalf("ChangedSet: %v", err)
	}
	c, ok := byPath(cs)["src/new.py"]
	if !ok {
		t.Fatalf("ChangedSet omitted the rename destination; got %v", paths(cs))
	}
	if c.Status != Renamed {
		t.Errorf("Status = %v, want renamed", c.Status)
	}
	if c.OldPath != "src/old.py" {
		t.Errorf("OldPath = %q, want %q (the old path must still select its tests)", c.OldPath, "src/old.py")
	}
}

func TestChangedSetLineRanges(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "src/auth.py", "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\n")
	commit(t, dir, "init")

	// Edit line 2 and lines 8-9; the two hunks must stay separate at --unified=0.
	write(t, dir, "src/auth.py", "l1\nEDITED2\nl3\nl4\nl5\nl6\nl7\nEDITED8\nEDITED9\nl10\n")

	cs, err := ChangedSet(dir, "HEAD")
	if err != nil {
		t.Fatalf("ChangedSet: %v", err)
	}
	c := byPath(cs)["src/auth.py"]
	want := []LineRange{{Start: 2, End: 2}, {Start: 8, End: 9}}
	if !reflect.DeepEqual(c.Lines, want) {
		t.Errorf("Lines = %#v, want %#v", c.Lines, want)
	}
	if c.Status != Modified {
		t.Errorf("Status = %v, want modified", c.Status)
	}
}

func TestChangedSetUnionsStagedWorktreeAndUntracked(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "src/a.py", "a = 1\n")
	write(t, dir, "src/b.py", "b = 1\n")
	commit(t, dir, "init")

	write(t, dir, "src/a.py", "a = 2\n")
	gitRun(t, dir, "add", "src/a.py")    // staged
	write(t, dir, "src/b.py", "b = 2\n") // unstaged
	write(t, dir, "src/c.py", "c = 1\n") // untracked

	cs, err := ChangedSet(dir, "HEAD")
	if err != nil {
		t.Fatalf("ChangedSet: %v", err)
	}
	want := []string{"src/a.py", "src/b.py", "src/c.py"}
	if !reflect.DeepEqual(paths(cs), want) {
		t.Errorf("paths = %v, want %v", paths(cs), want)
	}
}

func TestChangedSetAgainstAnEarlierBase(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "src/a.py", "a = 1\n")
	base := commit(t, dir, "one")
	write(t, dir, "src/b.py", "b = 1\n")
	commit(t, dir, "two")
	write(t, dir, "src/c.py", "c = 1\n")

	cs, err := ChangedSet(dir, base)
	if err != nil {
		t.Fatalf("ChangedSet: %v", err)
	}
	want := []string{"src/b.py", "src/c.py"}
	if !reflect.DeepEqual(paths(cs), want) {
		t.Errorf("paths = %v, want %v", paths(cs), want)
	}
}

func TestChangedSetIsSortedAndEmptyOnACleanTree(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "src/a.py", "a = 1\n")
	commit(t, dir, "init")

	cs, err := ChangedSet(dir, "HEAD")
	if err != nil {
		t.Fatalf("ChangedSet: %v", err)
	}
	if len(cs) != 0 {
		t.Errorf("ChangedSet on a clean tree = %v, want empty", paths(cs))
	}
}

func TestChangedSetOnARepoWithNoCommits(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "tests/test_a.py", "def test_a():\n    pass\n")

	cs, err := ChangedSet(dir, "HEAD")
	if err != nil {
		t.Fatalf("ChangedSet on a commitless repo returned %v, want nil", err)
	}
	if !reflect.DeepEqual(paths(cs), []string{"tests/test_a.py"}) {
		t.Errorf("paths = %v, want [tests/test_a.py]", paths(cs))
	}
}

// An empty base is the default, and the plan specifies that default as HEAD: the
// working tree against the current commit is what a TDD cycle actually edits.
func TestChangedSetDefaultBaseIsHEAD(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "src/a.py", "a = 1\n")
	commit(t, dir, "one")
	write(t, dir, "src/a.py", "a = 2\n")
	write(t, dir, "src/b.py", "b = 1\n")

	cs, err := ChangedSet(dir, "")
	if err != nil {
		t.Fatalf("ChangedSet with an empty base: %v", err)
	}
	want := []string{"src/a.py", "src/b.py"}
	if !reflect.DeepEqual(paths(cs), want) {
		t.Errorf("paths = %v, want %v", paths(cs), want)
	}

	head, err := ChangedSet(dir, "HEAD")
	if err != nil {
		t.Fatalf("ChangedSet(HEAD): %v", err)
	}
	if !reflect.DeepEqual(cs, head) {
		t.Errorf("ChangedSet with an empty base = %#v, want the same result as HEAD %#v", cs, head)
	}
}

// An unknown base ref is an error, not a silently empty changed set: an agent that gets
// back "nothing changed" for a typo'd ref runs no tests and believes it is safe.
func TestChangedSetUnknownBaseIsAnError(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "src/a.py", "a = 1\n")
	commit(t, dir, "one")

	if _, err := ChangedSet(dir, "no-such-ref"); err == nil {
		t.Fatal("ChangedSet against an unknown base returned nil error")
	}
}
