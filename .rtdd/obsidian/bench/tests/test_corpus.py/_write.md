---
id: "bench/tests/test_corpus.py::_write"
source: "bench/tests/test_corpus.py"
start: 33
end: 39
kind: "func"
is-test: false
tags: [rtdd/code]
---
# _write

#rtdd/code

## Tests

- [[bench/tests/test_corpus.py/test_loads_when_lock_matches]] (calls)
- [[bench/tests/test_corpus.py/test_refuses_when_lock_does_not_match]] (calls)
- [[bench/tests/test_corpus.py/test_refuses_a_repo_not_in_the_frozen_list]] (calls)
- [[bench/tests/test_corpus.py/test_refuses_an_empty_excluded_table]] (calls)

## Callers

- [[bench/tests/test_replay.py/_repo_whose_red_survives_the_child]] (calls)

## Callees

- [[bench/corpus.lock/_file]] (references)
- [[bench/corpus.yaml/_file]] (references)
