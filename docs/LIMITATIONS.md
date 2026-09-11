# Limitations

- **Only Python gets execution-derived selection.** Per-test attribution does not exist in
  the JavaScript or Go ecosystems — Istanbul and v8 carry aggregate counters with no test
  dimension ([vitest#6735](https://github.com/vitest-dev/vitest/issues/6735), open since
  October 2024), and Go's `-coverprofile` has none either. Other languages get *static*
  selection: declared correspondence and imports, nothing instrumented.
  Passing it is weaker evidence, and it was pre-registered against a naive path heuristic
  and did not beat it, so it does not carry its weight as a distinct tier.
  See [the comparison](docs/results/axis2-selection-baselines.md), and the records in
  [`bench/results/flask/summary.json`](bench/results/flask/summary.json) and
  [`bench/results/httpie/summary.json`](bench/results/httpie/summary.json).
- **It does not enforce anything.** No gate, no policy exit code, no expected-phase flag.
- **It does not replace CI.** Run the full suite there.
- **It is not sound program analysis.** Selection can be wrong; the tier and the uncovered
  report are how you see when.
- **It does not reduce token cost.** No model calls in the hot path.
- **Coverage misses what no test ran.** Code executed at import time is attributed to no
  test and reported separately, not as a coverage gap.
- **`COVERAGE_CORE=ctrace` is forced**, costing roughly 2× tracing overhead. Under
  coverage.py's `sysmon` core — the default on Python 3.14+ — dynamic contexts are dropped
  with a warning and a zero exit, producing a near-empty map. RTDD treats that as fatal.

Back to the [README](../README.md).
