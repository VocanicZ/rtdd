---
id: "bench/replay/gitwork.py::materialise_probe"
source: "bench/replay/gitwork.py"
start: 278
end: 302
kind: "func"
is-test: false
tags: [rtdd/code]
---
# materialise_probe

#rtdd/code

## Tests

- [[bench/tests/test_gitwork.py/test_materialise_probe_keeps_child_tests_over_parent_source]] (calls)

## Callers

- [[bench/replay/replay.py/replay_repo]] (calls)

## Callees

- [[bench/replay/gitwork.py/Change]] (calls)
- [[bench/replay/gitwork.py/diff_changes]] (calls)
- [[bench/replay/gitwork.py/is_test_path]] (calls)
- [[bench/replay/gitwork.py/_write_blob]] (calls)
- [[bench/replay/gitwork.py/_remove]] (calls)
