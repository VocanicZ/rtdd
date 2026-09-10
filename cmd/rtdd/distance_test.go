package main

import "testing"

// The selector asks for the age of the commit on EVERY row of the T0 selection, and a
// freshly seeded map records the same sha on all of them. Uncached, a 437-row selection
// asked one question 437 times at three git subprocesses each — measured at 9774 ms for
// `rtdd which` on a real flask clone, against 2932 ms to run the whole suite.
func TestMemoizeAsksOncePerDistinctCommit(t *testing.T) {
	calls := map[string]int{}
	distance := memoize(func(sha string) int {
		calls[sha]++
		return len(sha)
	})

	for i := 0; i < 437; i++ {
		if got := distance("d73fa1c"); got != 7 {
			t.Fatalf("distance = %d, want 7", got)
		}
	}
	distance("other")

	if calls["d73fa1c"] != 1 {
		t.Errorf("asked %d times for one sha, want 1", calls["d73fa1c"])
	}
	if calls["other"] != 1 {
		t.Errorf("a second sha was asked %d times, want 1", calls["other"])
	}
}

// A cached miss is still a miss. Treating "unknown" as absent would re-ask every row.
func TestMemoizeCachesTheUnknownAnswerToo(t *testing.T) {
	calls := 0
	distance := memoize(func(string) int { calls++; return -1 })
	for i := 0; i < 10; i++ {
		if got := distance("gone"); got != -1 {
			t.Fatalf("distance = %d, want -1", got)
		}
	}
	if calls != 1 {
		t.Errorf("an unreachable commit was asked %d times, want 1", calls)
	}
}
