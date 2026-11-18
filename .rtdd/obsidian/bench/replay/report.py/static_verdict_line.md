---
id: "bench/replay/report.py::static_verdict_line"
source: "bench/replay/report.py"
start: 437
end: 453
kind: "func"
is-test: false
tags: [rtdd/code]
---
# static_verdict_line

#rtdd/code

## Tests

- [[bench/tests/test_report.py/test_the_static_verdict_fires_the_kill_condition_on_a_tie]] (calls)
- [[bench/tests/test_report.py/test_the_static_verdict_reports_a_win_when_there_is_one]] (calls)
- [[bench/tests/test_report.py/test_static_recall_bought_with_more_time_is_not_a_win]] (calls)
- [[bench/tests/test_report.py/test_a_static_population_with_no_ground_truth_is_not_computable_not_a_loss]] (calls)
- [[bench/tests/test_report.py/test_the_static_verdict_refuses_to_compute_without_both_arms]] (calls)

## Callers

- [[bench/replay/report.py/render_markdown]] (calls)

## Callees

- [[bench/replay/report.py/_primary_verdict]] (calls)
