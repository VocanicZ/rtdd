# rtdd — a negative result

**This repository is a published measurement, not a tool to install.**

RTDD tested one hypothesis: that a **dynamic coverage** map, derived from real test
execution, would beat a **static dependency graph** at reducing the regressions an AI coding
agent introduces. It does not, on this benchmark, at this scale. The numbers are below, the
harness that produced them is in `bench/`, and the pre-registration that fixed the sample,
the arms, and the kill criterion before the run is in
[`bench/PREREGISTRATION.md`](bench/PREREGISTRATION.md) at git tag `prereg-m4`.

That hypothesis is about one of two tiers, and only one of them is measured coverage.
*Execution-derived* selection — the tier tested here — is Python only: per-test attribution
does not exist in the JavaScript or Go ecosystems, where coverage carries aggregate counters
with no test dimension. Every other language gets *static* selection instead: declared file
correspondence and imports, with nothing instrumented. Static selection never watched a test
run, so it can miss a test an execution-derived selection would have caught, and passing it
is weaker evidence. The result below is a verdict on the measured tier only; the static tier
was never the thing on trial, and losing on the tier that instruments is not a reason to
trust the tier that does not.

The static tier carries a losing pre-registration of its own, and it is separate from this
one. Spec §7 fixed, before the arm existed, that the `static` arm had to beat the naive
`tests/test_<module>.py` path heuristic on change-level recall at comparable or better
selected-duration fraction. On the Axis 2 replay corpus it did not: on flask's `probe`
population — the only one of the four with any detecting commits at all — `static` and the
path heuristic both score 0.333 change-level recall at a 0.010 selected-duration fraction,
and on httpie's `natural` population, where recall has no denominator to be scored against,
`static` spends 0.426 of the suite's duration against the heuristic's 0.032. A tie is not a
beat, so **the static tier does not carry its weight as a distinct tier** either. It is a
fallback for repositories nothing can instrument, not a second result. The numbers are in
[`bench/results/flask/summary.json`](bench/results/flask/summary.json) and
[`bench/results/httpie/summary.json`](bench/results/httpie/summary.json), derived from
records the benchmark had already committed — no benchmark was re-run to produce them.

The binaries are not released. Use **[TDAD](https://github.com/pepealonso95/TDAD)** instead.

## The result

All five arms ran on one harness with one model (Qwen3-Coder-30B-A3B-Instruct, greedy, 40
turns) over a pre-registered 100-instance sample of SWE-bench Verified.

<!-- PASTE bench/results/swebench/tables.md HERE -->

A regression is defined mechanically in
[`bench/swebench/metrics.py`](bench/swebench/metrics.py): a test id in the evaluation
report's `tests_status.PASS_TO_PASS.failure` list. The RTDD arm's prompt is byte-for-byte the
vanilla arm's prompt plus one `<test-context>` block, asserted in
[a test](bench/swebench/tests/test_prompts.py), so the arm cannot be dismissed as a
procedural-prompt confound in either direction.

## Why, as far as we can tell

Stated as hypotheses, because a losing arm does not license a confident post-hoc story:

- **Coverage only knows paths some test already took.** A static graph sees a call edge the
  moment it is written; a coverage map sees it only after a test has run through it. For a
  benchmark of one-shot patches to unfamiliar code, the graph's speculative edges appear to
  be worth more than the coverage map's verified ones.
- **Import-time attribution is a real hole, not a footnote.** Python attributes every line
  executed during collection to no test at all, so dataclasses, enums, config modules, ORM
  models, decorators, and `__init__.py` re-exports carry no coverage edge. That class of file
  is enormous, and the static import scan that RTDD falls back to for it is exactly what the
  static graph does natively for everything.
- **File-level granularity may be the binding constraint**, not the static/dynamic axis.
  pytest-testmon has done method-level checksums since around 2016; a fair rerun of this
  hypothesis would test dynamic-and-method-level against static-and-method-level, and this
  benchmark did not.
- **The 2× tracing tax is a real cost with no measured benefit here.** `COVERAGE_CORE=ctrace`
  is forced because coverage.py's faster `sysmon` core silently drops dynamic contexts, so
  every instrumented run pays for a map that did not pay for itself.

## What is still worth taking from here

- **`bench/swebench/`** — a five-arm SWE-bench Verified regression harness with a mechanical
  `PASS_TO_PASS` metric, a pre-registration gate that refuses to run unsigned, and an arm
  composition that makes procedural-prompt contamination structurally impossible. It is
  reusable for any test-context intervention, not just this one.
- **`protocol/` and `cmd/rtdd-gen`** — one source rendered into a Claude Code `SKILL.md`, an
  `AGENTS.md` block, and a Cursor `.mdc`, with per-target section sets, byte budgets, and
  per-target validity assertions, plus a CI check that catches both staleness and
  wrong-for-this-target content.
- **The uncovered-change report** — the line-granular covered / uncovered / import-time split
  has no equivalent in a static-graph tool, and it was not what this benchmark measured. It
  remains untested as a standalone intervention.

## Prior art

RTDD did not invent test impact analysis, and this result is a reason to say so more loudly
rather than less.

- **[TDAD](https://arxiv.org/abs/2603.17973)** — Alonso, Yovine and Braberman, March 2026 —
  builds the static source↔test dependency graph this project tried and failed to beat, and
  published 6.08% → 1.82% on SWE-bench Verified. Reference implementation:
  [pepealonso95/TDAD](https://github.com/pepealonso95/TDAD). TypeScript port:
  [fmguerreiro/tdad-ts](https://github.com/fmguerreiro/tdad-ts). **Use it.**
- **[pytest-testmon](https://www.testmon.org/blog/determining-affected-tests/)** — method-level
  AST checksums since around 2016, finer-grained than anything here.
- **[Wallaby.js](https://wallabyjs.com/docs/features/ai/)** — ships an MCP server exposing
  `wallaby_coveredLinesForTest` and `wallaby_allTestsForFileAndLine` to Claude Code and Cursor
  today.
- **[Infinitest](https://github.com/infinitest/infinitest)** (2007) — classpath impact
  analysis marketed for "tight TDD cycles" nearly two decades ago.
- **[Ekstazi](https://users.ece.utexas.edu/~gligoric/papers/GligoricETAL15Ekstazi.pdf)**
  (2015) — the reference result for file-level regression test selection, and the source of
  the 32%-end-to-end-reduction figure that should temper anyone's expectations here.
- **[SonarQube's new-code coverage gate](https://docs.sonarsource.com/sonarqube-server/latest/user-guide/clean-as-you-code/)**,
  Codecov's patch status, and `diff-cover` — prior art for "changed code that no test covers
  is a problem".

Also in the lineage: [Bazel](https://bazel.build/query/guide), Google TAP, Azure DevOps TIA,
`jest --findRelatedTests`, NCrunch,
[Meta's predictive test selection](https://arxiv.org/abs/1810.05286).

## Documentation

- [Full result](docs/results/) — the standalone write-up.
- [Design](docs/specs/2026-08-26-rtdd-design.md) — the specification, including §15's
  commitment to publishing this outcome.
- [Design audit](docs/audits/2026-08-26-design-audit.md).

## License

MIT. See [LICENSE](LICENSE).
