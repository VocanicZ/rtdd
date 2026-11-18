---
id: "bench/replay/corpus.py::load_corpus"
source: "bench/replay/corpus.py"
start: 132
end: 202
kind: "func"
is-test: false
tags: [rtdd/code]
---
# load_corpus

#rtdd/code

## Tests

- [[bench/tests/test_audit.py/test_the_shipped_corpus_passes_its_own_audit]] (calls)
- [[bench/tests/test_audit.py/test_the_shipped_corpus_no_longer_admits_sqlfluff]] (calls)
- [[bench/tests/test_audit.py/test_every_admitted_repo_records_the_hardware_it_was_measured_on]] (calls)
- [[bench/tests/test_audit.py/test_the_superseded_version_still_loads_at_its_published_digest]] (calls)
- [[bench/tests/test_audit.py/test_asking_for_a_version_that_was_never_archived_refuses]] (calls)
- [[bench/tests/test_corpus.py/test_loads_when_lock_matches]] (calls)
- [[bench/tests/test_corpus.py/test_refuses_when_lock_does_not_match]] (calls)
- [[bench/tests/test_corpus.py/test_refuses_a_repo_not_in_the_frozen_list]] (calls)
- [[bench/tests/test_corpus.py/test_refuses_an_empty_excluded_table]] (calls)
- [[bench/tests/test_corpus.py/test_shipped_corpus_is_frozen_and_loadable]] (calls)
- [[bench/tests/test_corpus.py/test_shipped_corpus_states_mechanical_criteria_and_a_reason_per_exclusion]] (calls)
- [[bench/tests/test_graph_ids.py/test_every_corpus_language_has_an_expansion_case]] (calls)

## Callers

- [[bench/replay/cli.py/_corpus]] (calls)
- [[bench/tests/test_results_corpus_coverage.py/corpus_ids]] (calls)
- [[bench/tests/test_results_n4_rounds.py/corpus_ids]] (calls)

## Callees

- [[bench/corpus.yaml/_file]] (references)
- [[bench/replay/corpus.py/CorpusError]] (calls)
- [[bench/replay/corpus.py/CorpusNotFrozenError]] (calls)
- [[bench/replay/corpus.py/RepoSpec]] (calls)
- [[bench/replay/corpus.py/Corpus]] (calls)
- [[bench/replay/corpus.py/_digest]] (calls)
- [[bench/replay/corpus.py/archive_path]] (calls)
- [[bench/replay/corpus.py/_parse_lock]] (calls)
- [[bench/replay/corpus.py/_declared_version]] (calls)
