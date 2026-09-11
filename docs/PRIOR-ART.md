# Prior art

RTDD did not invent test impact analysis. It is a late entry in a long line, and the
comparison against that line is the point of the project rather than a marketing frame.

- **[TDAD](https://arxiv.org/abs/2603.17973)** — *Test-Driven Agentic Development*, Alonso,
  Yovine and Braberman, March 2026 — is the closest work and the direct baseline. It builds a
  **static** source↔test dependency graph by parsing Python syntax trees, ships it to the
  agent as a skill file, and measured regressions on SWE-bench Verified falling from **6.08%
  to 1.82%**. Its reference implementation is at
  [pepealonso95/TDAD](https://github.com/pepealonso95/TDAD) and a TypeScript port is at
  [fmguerreiro/tdad-ts](https://github.com/fmguerreiro/tdad-ts). RTDD runs TDAD's own
  implementation as an arm of its benchmark rather than citing its number.

  **TDAD's second result is why this tool looks the way it does.** Adding TDD *procedural*
  instructions without targeted test context raised regressions to **9.94% — worse than no
  intervention at all**. Their conclusion is that surfacing contextual information beats
  prescribing procedural workflows, and RTDD is built on that conclusion: it reports, and it
  has no opinion about your patch.

- **[pytest-testmon](https://www.testmon.org/blog/determining-affected-tests/)** has done
  coverage-derived Python test selection since around 2016, with **method-level AST
  checksums** — finer-grained than RTDD's file-level rows. If you want test selection for a
  human's editor loop, use testmon. RTDD is not an improvement on it and does not claim to be.

- **[Wallaby.js](https://wallabyjs.com/docs/features/ai/)** already ships an MCP server
  exposing `wallaby_coveredLinesForTest` and `wallaby_allTestsForFileAndLine` to Claude Code
  and Cursor. Agent-facing coverage context is not a new idea; it is a shipping commercial
  product for JavaScript.

- **[Infinitest](https://github.com/infinitest/infinitest)** (2007) marketed classpath impact
  analysis for "tight TDD cycles" nearly two decades ago.

- **[Ekstazi](https://users.ece.utexas.edu/~gligoric/papers/GligoricETAL15Ekstazi.pdf)**
  (Gligoric, Eloussi and Marinov, 2015) is the reference result for file-level regression
  test selection, and the source of the most sobering number in this README: selecting a
  small fraction of tests still yielded only about **32% average end-to-end reduction**. That
  gap between "2% of tests selected" and "32% faster" is real, and RTDD measures itself
  against it rather than reporting selection ratio alone.

- **[SonarQube's new-code coverage gate](https://docs.sonarsource.com/sonarqube-server/latest/user-guide/clean-as-you-code/)**,
  Codecov's patch status, and `diff-cover` are prior art for "changed code that no test
  covers is a problem". RTDD's uncovered-change report is the same idea moved from CI into
  the inner loop, and reported rather than enforced.

- **[Bazel](https://bazel.build/query/guide)**, Google TAP, Microsoft's Azure DevOps Test
  Impact Analysis, `jest --findRelatedTests`, NCrunch, and
  **[Meta's predictive test selection](https://arxiv.org/abs/1810.05286)** are also in the
  lineage and are not claimed as novel here.

**What is new here, if anything, is one measurement.** Every tool above builds its map
either statically or from per-file coverage aggregates. TDAD published a static-graph number
on SWE-bench Verified with a reproducible harness. RTDD substitutes a **dynamic coverage**
map — which sees dynamic dispatch, dependency injection, plugin registries and monkeypatching
that a call graph reports as zero callers, and is blind in return to any path no test has
ever taken — and reruns the same benchmark. So the question *does execution-derived context
beat a call graph for agent regressions?* has an answer instead of an argument.

The answer is below, whichever way it fell.

Back to the [README](../README.md).
