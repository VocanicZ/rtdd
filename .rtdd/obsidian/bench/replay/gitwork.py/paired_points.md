---
id: "bench/replay/gitwork.py::paired_points"
source: "bench/replay/gitwork.py"
start: 116
end: 169
kind: "func"
is-test: false
tags: [rtdd/code]
---
# paired_points

#rtdd/code

## Tests

- [[bench/tests/test_gitwork.py/test_paired_points_keeps_only_commits_touching_source_and_tests]] (calls)
- [[bench/tests/test_gitwork.py/test_paired_points_returns_real_commits_with_real_parents]] (calls)
- [[bench/tests/test_gitwork.py/test_paired_points_is_capped_and_ordered_oldest_first]] (calls)
- [[bench/tests/test_gitwork.py/test_paired_points_skips_merges_like_replay_points]] (calls)

## Callers

- [[bench/replay/replay.py/replay_repo]] (calls)

## Callees

- [[bench/replay/gitwork.py/ReplayPoint]] (calls)
- [[bench/replay/gitwork.py/git]] (calls)
- [[bench/replay/gitwork.py/is_test_path]] (calls)
