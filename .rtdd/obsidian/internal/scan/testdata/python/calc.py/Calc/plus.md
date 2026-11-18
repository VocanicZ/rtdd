---
id: "internal/scan/testdata/python/calc.py::Calc::plus"
source: "internal/scan/testdata/python/calc.py"
start: 19
end: 21
kind: "method"
is-test: false
tags: [rtdd/code]
---
# plus

#rtdd/code

## Tests

none

## Callers

- [[bench/tests/test_cli.py/_published_with_importgraph]] (calls)
- [[bench/tests/test_report.py/_static_out]] (calls)
- [[internal/scan/testdata/python/calc.py/Calc]] (method)
- [[internal/scan/testdata/python/calc.py/Calc/run/step]] (calls)
- [[internal/scan/testdata/python/test_calc.py/TestCalc/test_plus]] (calls)

## Callees

- [[internal/scan/testdata/python/calc.py/add]] (calls)
