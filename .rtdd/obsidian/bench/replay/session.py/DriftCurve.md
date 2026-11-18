---
id: "bench/replay/session.py::DriftCurve"
source: "bench/replay/session.py"
start: 73
end: 85
kind: "class"
is-test: false
tags: [rtdd/code]
---
# DriftCurve

#rtdd/code

## Tests

- [[bench/tests/test_cli.py/test_session_runs_the_drift_worktree_with_its_own_source_in_front]] (calls)
- [[bench/tests/test_cli.py/test_session_does_not_overwrite_the_replay_config_that_produced_the_table]] (calls)
- [[bench/tests/test_cli.py/test_session_threads_corpus_version_through_to_the_loader]] (calls)
- [[bench/tests/test_report.py/test_drift_is_written_only_when_a_curve_was_measured]] (calls)

## Callers

- [[bench/replay/session.py/run_drift]] (calls)

## Callees

- [[bench/replay/session.py/DriftCurve/to_dict]] (method)
- [[bench/results/flask/drift.json/_file]] (references)
- [[bench/results/httpie/drift.json/_file]] (references)
- [[bench/results/sqlfluff/drift.json/_file]] (references)
