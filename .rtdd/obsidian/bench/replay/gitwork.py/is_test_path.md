---
id: "bench/replay/gitwork.py::is_test_path"
source: "bench/replay/gitwork.py"
start: 214
end: 223
kind: "func"
is-test: false
tags: [rtdd/code]
---
# is_test_path

#rtdd/code

## Tests

- [[bench/tests/test_gitwork.py/test_is_test_path]] (calls)

## Callers

- [[bench/replay/gitwork.py/paired_points]] (calls)
- [[bench/replay/gitwork.py/probe_candidates]] (calls)
- [[bench/replay/gitwork.py/materialise_probe]] (calls)
- [[bench/replay/strategies/pathheuristic.py/PathHeuristic/select]] (calls)

## Callees

- [[bench/replay/gitwork.py/_glob_variants]] (calls)
