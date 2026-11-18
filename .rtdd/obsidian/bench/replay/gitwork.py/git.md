---
id: "bench/replay/gitwork.py::git"
source: "bench/replay/gitwork.py"
start: 57
end: 67
kind: "func"
is-test: false
tags: [rtdd/code]
---
# git

#rtdd/code

## Tests

- [[bench/tests/test_gitwork.py/test_clone_pinned_checks_out_the_pin_and_refuses_a_moved_clone]] (calls)
- [[bench/tests/test_gitwork.py/test_worktrees_are_detached_and_removed_without_disturbing_the_clone]] (calls)
- [[bench/tests/test_gitwork.py/test_materialise_natural_keeps_head_at_parent_and_leaves_adds_untracked]] (calls)
- [[bench/tests/test_gitwork.py/test_materialise_natural_applies_deletions]] (calls)
- [[bench/tests/test_gitwork.py/test_materialise_probe_keeps_child_tests_over_parent_source]] (calls)
- [[bench/tests/test_gitwork.py/test_paired_points_returns_real_commits_with_real_parents]] (calls)
- [[bench/tests/test_gitwork.py/test_paired_points_is_capped_and_ordered_oldest_first]] (calls)
- [[bench/tests/test_gitwork.py/test_paired_points_skips_merges_like_replay_points]] (calls)
- [[bench/tests/test_session.py/test_drift_never_moves_head_or_commits]] (calls)

## Callers

- [[bench/replay/gitwork.py/clone_pinned]] (calls)
- [[bench/replay/gitwork.py/replay_points]] (calls)
- [[bench/replay/gitwork.py/paired_points]] (calls)
- [[bench/replay/gitwork.py/diff_changes]] (calls)
- [[bench/replay/gitwork.py/add_worktree]] (calls)
- [[bench/replay/gitwork.py/remove_worktree]] (calls)
- [[bench/replay/gitwork.py/working_changed_paths]] (calls)
- [[bench/replay/session.py/run_drift]] (calls)
- [[bench/tests/test_gitwork.py/_commit]] (calls)
- [[bench/tests/test_gitwork.py/_history]] (calls)

## Callees

- [[bench/replay/gitwork.py/GitError]] (calls)
