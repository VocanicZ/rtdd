---
id: "bench/tests/test_cli.py::_corpus_at"
source: "bench/tests/test_cli.py"
start: 81
end: 92
kind: "func"
is-test: false
tags: [rtdd/code]
---
# _corpus_at

#rtdd/code

## Tests

- [[bench/tests/test_cli.py/test_replay_refuses_an_unfrozen_corpus_before_doing_any_work]] (calls)
- [[bench/tests/test_cli.py/test_doctor_exits_non_zero_when_the_corpus_is_not_frozen]] (calls)

## Callers

- [[bench/tests/test_cli.py/bench]] (calls)

## Callees

- [[bench/corpus.lock/_file]] (references)
- [[bench/corpus.yaml/_file]] (references)
- [[bench/replay/corpus.py/freeze]] (calls)
