---
id: "bench/replay/records.py::CommitRecord::stratum"
source: "bench/replay/records.py"
start: 51
end: 52
kind: "method"
is-test: false
tags: [rtdd/code]
---
# stratum

#rtdd/code

## Tests

- [[bench/tests/test_records.py/test_commit_record_stratum_and_duration]] (calls)
- [[bench/tests/test_replay.py/test_replay_over_the_synth_repo_reproduces_the_hand_computed_ground_truth]] (calls)

## Callers

- [[bench/replay/metrics.py/by_stratum]] (calls)
- [[bench/replay/records.py/CommitRecord]] (method)
- [[bench/replay/records.py/CommitRecord/to_dict]] (calls)
- [[bench/replay/report.py/render_markdown]] (calls)

## Callees

- [[bench/replay/records.py/stratum_of]] (calls)
