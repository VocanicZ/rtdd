package gitctx

import "testing"

func TestCommitDistance(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "a.txt", "1\n")
	first := commit(t, dir, "one")
	write(t, dir, "b.txt", "1\n")
	second := commit(t, dir, "two")
	write(t, dir, "c.txt", "1\n")
	third := commit(t, dir, "three")

	tests := []struct {
		name string
		sha  string
		want int
	}{
		{"HEAD is zero away from itself", third, 0},
		{"one commit back", second, 1},
		{"two commits back", first, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CommitDistance(dir, tc.sha)
			if err != nil {
				t.Fatalf("CommitDistance: %v", err)
			}
			if got != tc.want {
				t.Errorf("CommitDistance(%q) = %d, want %d", tc.sha, got, tc.want)
			}
		})
	}
}

// A rebase, squash, or shallow clone leaves a recorded `c` unreachable. -1 means
// UNKNOWN. A caller that reads it as 0 would report a possibly-ancient map as fresh.
func TestCommitDistanceUnreachableIsMinusOne(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "a.txt", "1\n")
	commit(t, dir, "one")

	tests := []struct {
		name string
		sha  string
	}{
		{"a sha that never existed", "deadbee"},
		{"an empty sha", ""},
		{"a full-length sha that never existed", "0123456789abcdef0123456789abcdef01234567"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CommitDistance(dir, tc.sha)
			if err != nil {
				t.Fatalf("CommitDistance returned an error for an unreachable sha: %v", err)
			}
			if got != -1 {
				t.Errorf("CommitDistance(%q) = %d, want -1 (unknown, never fresh)", tc.sha, got)
			}
		})
	}
}

func TestCommitDistanceAfterAnAmendMakesTheOldShaUnreachable(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "a.txt", "1\n")
	commit(t, dir, "one")
	write(t, dir, "b.txt", "1\n")
	stale := commit(t, dir, "two")

	write(t, dir, "b.txt", "2\n")
	gitRun(t, dir, "add", "-A")
	gitRun(t, dir, "commit", "-q", "--amend", "-m", "two-amended")

	got, err := CommitDistance(dir, stale)
	if err != nil {
		t.Fatalf("CommitDistance: %v", err)
	}
	// The amended-away commit is still in the reflog, so git can resolve it; what must
	// never happen is a report of 0, which would mean "fresh".
	if got == 0 {
		t.Errorf("CommitDistance(%q) = 0 after an amend; a rewritten commit is never fresh", stale)
	}
}

func TestIsMergeCommit(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "a.txt", "1\n")
	commit(t, dir, "one")

	if merge, err := IsMergeCommit(dir, "HEAD"); err != nil || merge {
		t.Errorf("IsMergeCommit on an ordinary commit = (%v, %v), want (false, nil)", merge, err)
	}

	gitRun(t, dir, "checkout", "-q", "-b", "side")
	write(t, dir, "side.txt", "1\n")
	commit(t, dir, "side")
	gitRun(t, dir, "checkout", "-q", "main")
	write(t, dir, "main.txt", "1\n")
	commit(t, dir, "main")
	gitRun(t, dir, "merge", "-q", "--no-ff", "-m", "merge side", "side")

	merge, err := IsMergeCommit(dir, "HEAD")
	if err != nil {
		t.Fatalf("IsMergeCommit: %v", err)
	}
	if !merge {
		t.Error("IsMergeCommit on a merge commit = false, want true")
	}
}

func TestOlder(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "a.txt", "1\n")
	first := commit(t, dir, "one")
	write(t, dir, "b.txt", "1\n")
	second := commit(t, dir, "two")

	older := Older(dir)

	tests := []struct {
		name string
		a, b string
		want string
	}{
		{"ancestor wins, a first", first, second, first},
		{"ancestor wins, b first", second, first, first},
		{"identical shas", first, first, first},
		{"unreachable a is treated as older", "deadbee", second, "deadbee"},
		{"unreachable b is treated as older", first, "deadbee", "deadbee"},
		{"empty a yields b", "", second, second},
		{"empty b yields a", first, "", first},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := older(tc.a, tc.b); got != tc.want {
				t.Errorf("Older()(%q, %q) = %q, want %q", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// Older is the comparator mapstore.LoadWith takes; it must be usable directly as one.
func TestOlderIsUsableAsTheLoadWithComparator(t *testing.T) {
	dir := newRepo(t)
	write(t, dir, "a.txt", "1\n")
	first := commit(t, dir, "one")
	write(t, dir, "b.txt", "1\n")
	second := commit(t, dir, "two")

	var cmp func(a, b string) string = Older(dir)
	if got := cmp(second, first); got != first {
		t.Errorf("comparator(%q, %q) = %q, want %q", second, first, got, first)
	}
}
