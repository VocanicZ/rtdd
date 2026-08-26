# Methodology caveat: TDAD's arm is cited, not reproduced

**This document applies only when arm C could not be run locally.** The harness runs TDAD's
reference implementation at a pinned commit
(`bench/swebench/providers/tdad.py`, `TDAD_PIN`), and that is the intended path. If
`ensure_installed` or `index` raises `TdadUnavailable` — the pin has rotted, the package
will not install, or `tdad index` produces no static map — the arm is **not** run without its
map. An arm C with no context is arm A wearing arm C's name, and publishing it as a measured
result would report TDAD's approach as no better than vanilla when what actually happened is
that TDAD never ran. The run stops, and this is what gets published instead.

Under the contingency, arm C leaves the locally-measured table entirely and TDAD's published
**1.82%** regression rate appears in a separate, clearly labelled column, quoted from their
paper rather than measured here. What follows is what that makes unfair, stated so a reader
does not have to reconstruct it.

- **Different harness.** TDAD's numbers come from their own agent scaffold
  (`claudecode_n_codex_swebench/`), not ours. Tool surface, turn limits, context clipping,
  edit granularity, and shell access all differ, and all of them move regression rates
  independently of the map under test.
- **Possibly different instances.** TDAD sampled 100 instances of SWE-bench Verified. Unless
  their instance list is published and matched exactly, the two 100-instance samples are
  different draws from a 500-instance pool, and the sampling variance is on the order of the
  effect being measured.
- **Different inference stack.** Quantization (Q4_K_M), serving engine, sampling parameters,
  and context length all differ unless matched, and quantization in particular is known to
  change agentic tool-calling behaviour.
- **Our vanilla arm is the only bridge.** Any claim of the form "RTDD beats TDAD" reduces to
  "RTDD's delta against our vanilla exceeds TDAD's published delta against their vanilla".
  That is a comparison of two deltas measured on two harnesses, which is weaker than a
  head-to-head and must be described as such in the README, not in a footnote.
- **Direction only.** With the arm cited rather than run, the honest reportable claim is a
  direction and a rough magnitude, never a ranking. If the two deltas are within a factor of
  two of each other, the conclusion published is "no distinguishable difference".

The contingency also changes the ship decision: with arm C cited rather than measured, the
"RTDD wins clearly" row of the Ship / Don't ship table is unreachable, and the outcome is
handled under the "ties" row.

## What must be recorded if this document is in force

- The `TdadUnavailable` message — which step failed, on which instance, and the captured
  stderr — in the run's raw results, so "could not install" is distinguishable from "indexed
  nothing".
- The pin that was attempted, so a later reader can tell whether the pin rotted or the
  environment was at fault.
- A link to this file from the results section of the README, next to the table that carries
  the cited column.
