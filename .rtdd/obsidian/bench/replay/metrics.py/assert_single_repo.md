---
id: "bench/replay/metrics.py::assert_single_repo"
source: "bench/replay/metrics.py"
start: 58
end: 66
kind: "func"
is-test: false
tags: [rtdd/code]
---
# assert_single_repo

#rtdd/code

## Tests

- [[bench/tests/test_metrics.py/test_pooling_across_repos_is_refused]] (calls)
- [[bench/tests/test_metrics.py/test_assert_single_repo_returns_the_shared_id_and_empty_for_no_records]] (calls)

## Callers

- [[bench/replay/falsesignal.py/_fired]] (calls)
- [[bench/replay/falsesignal.py/summarise]] (calls)
- [[bench/replay/metrics.py/pair_up]] (calls)
- [[bench/replay/metrics.py/escalation_rate]] (calls)
- [[bench/replay/metrics.py/summarise]] (calls)
- [[bench/replay/report.py/build_summary]] (calls)

## Callees

- [[bench/replay/metrics.py/PoolingError]] (calls)
