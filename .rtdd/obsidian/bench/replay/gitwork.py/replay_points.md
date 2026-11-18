---
id: "bench/replay/gitwork.py::replay_points"
source: "bench/replay/gitwork.py"
start: 97
end: 113
kind: "func"
is-test: false
tags: [rtdd/code]
---
# replay_points

#rtdd/code

## Tests

- [[bench/tests/test_gitwork.py/test_replay_points_are_oldest_first_with_parents]] (calls)
- [[bench/tests/test_gitwork.py/test_replay_points_skip_the_root_commit]] (calls)
- [[bench/tests/test_gitwork.py/test_probe_candidates_touch_both_source_and_test_files]] (calls)
- [[bench/tests/test_gitwork.py/test_materialise_natural_keeps_head_at_parent_and_leaves_adds_untracked]] (calls)
- [[bench/tests/test_gitwork.py/test_materialise_probe_keeps_child_tests_over_parent_source]] (calls)

## Callers

- [[bench/replay/cli.py/cmd_session]] (calls)
- [[bench/replay/replay.py/replay_repo]] (calls)

## Callees

- [[bench/replay/gitwork.py/ReplayPoint]] (calls)
- [[bench/replay/gitwork.py/git]] (calls)
