---
id: "bench/swebench/budget.py::Budget::check"
source: "bench/swebench/budget.py"
start: 62
end: 73
kind: "method"
is-test: false
tags: [rtdd/code]
---
# check

#rtdd/code

## Tests

- [[bench/swebench/tests/test_budget.py/test_check_refuses_a_resume_that_is_already_over_the_cap]] (calls)
- [[bench/swebench/tests/test_budget.py/test_check_is_silent_while_both_ceilings_hold]] (calls)

## Callers

- [[bench/swebench/budget.py/Budget]] (method)
- [[bench/swebench/budget.py/Budget/add]] (calls)
- [[bench/swebench/run_arm.py/run_arm]] (calls)

## Callees

- [[bench/swebench/budget.py/BudgetExceeded]] (calls)
