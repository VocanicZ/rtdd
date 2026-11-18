---
id: "bench/replay/gitwork.py::add_worktree"
source: "bench/replay/gitwork.py"
start: 189
end: 193
kind: "func"
is-test: false
tags: [rtdd/code]
---
# add_worktree

#rtdd/code

## Tests

- [[bench/tests/test_gitwork.py/test_worktrees_are_detached_and_removed_without_disturbing_the_clone]] (calls)
- [[bench/tests/test_gitwork.py/test_materialise_natural_keeps_head_at_parent_and_leaves_adds_untracked]] (calls)
- [[bench/tests/test_gitwork.py/test_materialise_natural_applies_deletions]] (calls)
- [[bench/tests/test_gitwork.py/test_materialise_probe_keeps_child_tests_over_parent_source]] (calls)
- [[bench/tests/test_gitwork.py/test_working_changed_paths_is_repo_relative_and_slash_separated]] (calls)
- [[bench/tests/test_session.py/test_drift_accumulates_the_changed_set_across_uncommitted_cycles]] (calls)
- [[bench/tests/test_session.py/test_drift_denominator_grows_with_the_tests_the_cycles_add]] (calls)
- [[bench/tests/test_session.py/test_drift_never_moves_head_or_commits]] (calls)
- [[bench/tests/test_session.py/test_curve_serialises_as_the_report_layer_writes_drift_json]] (calls)
- [[bench/tests/test_session.py/test_no_points_is_an_empty_curve_and_never_collects]] (calls)
- [[bench/tests/test_session.py/test_default_selector_is_the_rtdd_binary_against_base_head]] (calls)
- [[bench/tests/test_session.py/test_run_drift_rejects_a_worktree_that_is_not_at_the_first_parent]] (calls)

## Callers

- [[bench/replay/cli.py/cmd_session]] (calls)
- [[bench/replay/replay.py/replay_repo]] (calls)

## Callees

- [[bench/replay/gitwork.py/git]] (calls)
