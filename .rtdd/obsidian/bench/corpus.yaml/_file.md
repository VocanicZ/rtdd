---
id: "bench/corpus.yaml"
source: "bench/corpus.yaml"
start: 1
end: 82
kind: "file"
is-test: false
tags: [rtdd/file]
---
# corpus.yaml

#rtdd/file

## Tests

- [[bench/tests/test_audit.py/test_the_shipped_corpus_passes_its_own_audit]] (references)
- [[bench/tests/test_audit.py/test_the_shipped_corpus_no_longer_admits_sqlfluff]] (references)
- [[bench/tests/test_audit.py/test_every_admitted_repo_records_the_hardware_it_was_measured_on]] (references)
- [[bench/tests/test_audit.py/test_the_superseded_version_still_loads_at_its_published_digest]] (references)
- [[bench/tests/test_audit.py/test_asking_for_a_version_that_was_never_archived_refuses]] (references)
- [[bench/tests/test_cli.py/test_replay_refuses_an_unfrozen_corpus_before_doing_any_work]] (references)
- [[bench/tests/test_cli.py/test_doctor_exits_non_zero_when_the_corpus_is_not_frozen]] (references)
- [[bench/tests/test_corpus.py/test_shipped_corpus_is_frozen_and_loadable]] (references)
- [[bench/tests/test_corpus.py/test_shipped_corpus_states_mechanical_criteria_and_a_reason_per_exclusion]] (references)
- [[bench/tests/test_graph_ids.py/test_every_corpus_language_has_an_expansion_case]] (references)

## Callers

- [[DEVELOPMENT.md/_file]] (references)
- [[bench/replay/cli.py/_file]] (references)
- [[bench/replay/corpus.py/Corpus/require]] (references)
- [[bench/replay/corpus.py/load_corpus]] (references)
- [[bench/results/README.md/_file]] (references)
- [[bench/tests/test_cli.py/_corpus_at]] (references)
- [[bench/tests/test_cli.py/bench]] (references)
- [[bench/tests/test_corpus.py/_write]] (references)
- [[bench/tests/test_results_corpus_coverage.py/corpus_ids]] (references)
- [[bench/tests/test_results_n4_rounds.py/corpus_ids]] (references)
- [[bench/tests/test_results_static_arm.py/offline_bench]] (references)
- [[docs/plans/04-m3-replay-benchmark.md/_file]] (references)
- [[docs/plans/05-m4-m5-swebench-release.md/_file]] (references)
- [[docs/plans/12-bench-rounds.md/_file]] (references)
- [[docs/results/axis2-corpus-admission.md/_file]] (references)
- [[docs/results/n4-bench-rounds.md/_file]] (references)
- [[docs/specs/2026-08-26-rtdd-design.md/_file]] (references)

## Callees

- [[docs/results/axis2-corpus-admission.md/_file]] (references)
