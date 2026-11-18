---
id: "bench/replay/gitwork.py::materialise_natural"
source: "bench/replay/gitwork.py"
start: 265
end: 275
kind: "func"
is-test: false
tags: [rtdd/code]
---
# materialise_natural

#rtdd/code

## Tests

- [[bench/tests/test_gitwork.py/test_materialise_natural_keeps_head_at_parent_and_leaves_adds_untracked]] (calls)
- [[bench/tests/test_gitwork.py/test_materialise_natural_applies_deletions]] (calls)

## Callers

- [[bench/replay/replay.py/replay_repo]] (calls)
- [[bench/replay/session.py/run_drift]] (calls)

## Callees

- [[bench/replay/chart.py/record_rounds_example/write]] (calls)
- [[bench/replay/gitwork.py/diff_changes]] (calls)
- [[bench/replay/gitwork.py/_write_blob]] (calls)
- [[bench/replay/gitwork.py/_remove]] (calls)
