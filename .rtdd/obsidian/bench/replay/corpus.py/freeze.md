---
id: "bench/replay/corpus.py::freeze"
source: "bench/replay/corpus.py"
start: 109
end: 129
kind: "func"
is-test: false
tags: [rtdd/code]
---
# freeze

#rtdd/code

## Tests

- [[bench/tests/test_cli.py/test_audit_fails_and_names_the_repo_that_breaches]] (calls)

## Callers

- [[bench/tests/test_cli.py/_corpus_at]] (calls)
- [[bench/tests/test_results_static_arm.py/offline_bench]] (calls)

## Callees

- [[bench/corpus.lock/_file]] (references)
- [[bench/replay/corpus.py/_digest]] (calls)
- [[bench/replay/corpus.py/_parse_lock]] (calls)
- [[bench/replay/corpus.py/_declared_version]] (calls)
