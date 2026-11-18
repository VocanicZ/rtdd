---
id: "bench/replay/report.py::verdict_line"
source: "bench/replay/report.py"
start: 412
end: 421
kind: "func"
is-test: false
tags: [rtdd/code]
---
# verdict_line

#rtdd/code

## Tests

- [[bench/tests/test_report.py/test_verdict_line_states_the_pre_registered_criterion]] (calls)
- [[bench/tests/test_report.py/test_verdict_uses_the_failure_wording_verbatim_when_rtdd_costs_more_duration]] (calls)
- [[bench/tests/test_report.py/test_verdict_says_rtdd_wins_only_when_it_is_no_more_expensive]] (calls)
- [[bench/tests/test_report.py/test_verdict_refuses_to_compute_without_both_arms]] (calls)
- [[bench/tests/test_report.py/test_a_green_natural_population_still_publishes_the_probe_comparison]] (calls)

## Callers

- [[bench/replay/chart.py/_verdict_for]] (calls)
- [[bench/replay/report.py/render_markdown]] (calls)

## Callees

- [[bench/replay/report.py/_primary_verdict]] (calls)
