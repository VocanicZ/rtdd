---
id: "bench/replay/records.py::CommitRecord"
source: "bench/replay/records.py"
start: 38
end: 85
kind: "class"
is-test: false
tags: [rtdd/code]
---
# CommitRecord

#rtdd/code

## Tests

- [[bench/tests/test_metrics.py/test_pooling_across_repos_is_refused]] (calls)
- [[bench/tests/test_metrics.py/test_a_selected_pre_existing_failure_is_not_a_detection]] (calls)
- [[bench/tests/test_records.py/test_commit_record_stratum_and_duration]] (calls)

## Callers

- [[bench/replay/replay.py/replay_repo]] (calls)
- [[bench/tests/test_cli.py/_published_repo]] (calls)
- [[bench/tests/test_derive.py/commit]] (calls)
- [[bench/tests/test_metrics.py/_commit]] (calls)
- [[bench/tests/test_metrics.py/_records_from_synth]] (calls)
- [[bench/tests/test_records.py/_commit_record]] (calls)
- [[bench/tests/test_report.py/_out]] (calls)
- [[bench/tests/test_report.py/_cheap_path_out]] (calls)
- [[bench/tests/test_report.py/_no_natural_detections]] (calls)
- [[bench/tests/test_strategy_rounds.py/_records]] (calls)

## Callees

- [[bench/replay/records.py/CommitRecord/stratum]] (method)
- [[bench/replay/records.py/CommitRecord/total_duration_ms]] (method)
- [[bench/replay/records.py/CommitRecord/to_dict]] (method)
- [[bench/replay/records.py/CommitRecord/from_dict]] (method)
