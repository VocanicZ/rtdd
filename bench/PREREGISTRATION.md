---
status: UNSIGNED
sample_size: 100
sample_seed: 20260826
instance_list_sha256:
model: Qwen3-Coder-30B-A3B-Instruct
arms: [vanilla, tdd, tdad, rtdd, rtdd_tdd]
stratified_recall_floor:
vanilla_equivalence_k: 2
signed_by:
signed_at:
---

# RTDD Axis 1 pre-registration

This file is the contract. `bench/swebench/preflight.py` refuses to launch any arm until
`status` is `SIGNED`, every field above has a value, and the commit that last touched this
file is reachable from the `prereg-m4` git tag.

Three fields ship deliberately empty — `stratified_recall_floor`, `signed_by`, `signed_at` —
plus `instance_list_sha256`, which a sibling task fills when it freezes the sample. The
harness is inert until a human writes them. No agent may write, guess, default, or infer the
kill criterion.

## Sample

100 instances drawn from SWE-bench Verified (500) with
`random.Random(20260826).sample(eligible, 100)`, where `eligible` is the sorted list of
instance ids whose `PASS_TO_PASS` set is non-empty. Instances with an empty `PASS_TO_PASS`
set are ineligible because the regression rate is undefined for them; the count of such
exclusions is published. n=100 matches TDAD's published sample size.

## Arms

| id | prompt composition | context tool |
|---|---|---|
| `vanilla` | BASE | none |
| `tdd` | BASE + TDD_PROSE | none |
| `tdad` | BASE + TDD_PROSE + `<test-context>` | `test_impact` |
| `rtdd` | BASE + `<test-context>` | `rtdd_which` |
| `rtdd_tdd` | BASE + TDD_PROSE + `<test-context>` | `rtdd_which` |

`tdad` reproduces TDAD's published best arm, which is graph **plus** TDD prose.
`rtdd` is the project's actual claim: context with no procedure.
`rtdd_tdd` is the parity arm — the exact counterpart of `tdad`, so the comparison is not
confounded by the presence or absence of the prose.

## Regression

A regression is a test id appearing in the SWE-bench evaluation report's
`tests_status.PASS_TO_PASS.failure` list. The denominator is the dataset's `PASS_TO_PASS`
cardinality, not the report's, so an evaluation that crashes cannot shrink it. An instance
whose patch is empty or fails to apply contributes zero regressions and zero resolutions and
stays in both denominators.

Both rates are published:
- test-level: sum of failing P2P tests / sum of P2P tests
- instance-level: fraction of instances with at least one failing P2P test

Resolution rate is printed beside the regression rate in every table.

## Vanilla equivalence band

The local `vanilla` arm reproduces TDAD's published 6.08% if `0.0608` lies within
`local_rate ± vanilla_equivalence_k × wilson_half_width(local_rate, n)`. If it does not,
every published table carries a `HARNESS DIVERGENCE` banner, TDAD's published figures are
removed from the comparison columns, and all claims are stated in local-relative form only.

## Kill criterion

`stratified_recall_floor` is the M3 Axis-2 stratified change-level recall on the
`|F_full| == 1` stratum, below which RTDD's selector is not published regardless of the
Axis 1 outcome. **This value must be written by a human before the run.** It is the number
that lets this project lose: a selector that misses the one test a change actually breaks is
not a selector, however well the Axis 1 arms score.

Three candidate floors, with the trade-off each buys:

- **0.90** — the permissive floor. Roughly one missed single-test change in ten is tolerated
  before the selector is withdrawn. Most likely of the three to leave RTDD publishable, and
  correspondingly the weakest claim: at 0.90 an agent trusting `rtdd which` on a
  single-target change is being told the wrong thing often enough to notice within a day's
  work, so the honest framing is "narrows the suite", not "safe to trust".
- **0.95** — the middle floor. Matches the recall level at which test-selection literature
  generally stops calling a technique unsafe, and is the value the Axis-2 harness was sized
  for: at n=100 the Wilson interval around 0.95 is roughly ±0.04, so the sample can
  distinguish 0.95 from 0.90 but not from 0.97. Choosing it means accepting that a result
  landing at 0.93 is a genuine coin-flip the pre-registration will decide, not the analyst.
- **0.99** — the strict floor. Effectively demands the selector never miss on the easiest
  stratum, which is the only stratum where a miss has no excuse. Strongest claim if it holds,
  and the most likely of the three to kill the selector on sample noise alone: a single miss
  in 100 puts the point estimate at the floor and the interval below it.

The floor is not the measured value. It is chosen *before* the measurement, and the
measurement is then allowed to fall below it.

## Model and budget

One model for every arm: `Qwen3-Coder-30B-A3B-Instruct`, served locally, greedy decoding
(`temperature=0`), 32768-token context, 4096 max output tokens per turn, 40 tool-call turns
per instance. Matching TDAD's model class is deliberate — their published numbers were
measured on Qwen3-Coder 30B Q4_K_M, and swapping in a frontier model would make the
comparison to their table meaningless.
