---
id: "bench/replay/gitwork.py::working_changed_paths"
source: "bench/replay/gitwork.py"
start: 305
end: 326
kind: "func"
is-test: false
tags: [rtdd/code]
---
# working_changed_paths

#rtdd/code

## Tests

- [[bench/tests/test_gitwork.py/test_materialise_natural_keeps_head_at_parent_and_leaves_adds_untracked]] (calls)
- [[bench/tests/test_gitwork.py/test_materialise_natural_applies_deletions]] (calls)
- [[bench/tests/test_gitwork.py/test_working_changed_paths_is_repo_relative_and_slash_separated]] (calls)
- [[bench/tests/test_session.py/test_drift_never_moves_head_or_commits]] (calls)

## Callers

- [[bench/replay/replay.py/_material_changes]] (calls)
- [[bench/replay/session.py/run_drift]] (calls)
- [[bench/tests/test_session.py/test_drift_accumulates_the_changed_set_across_uncommitted_cycles/fake_select]] (calls)

## Callees

- [[bench/replay/gitwork.py/Change]] (calls)
- [[bench/replay/gitwork.py/git]] (calls)
