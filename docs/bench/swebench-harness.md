# The Axis 1 harness: how to reproduce it, and what should refuse

`bench/swebench/` is the SWE-bench Verified harness for spec §3's question — does a dynamic
coverage map beat a static dependency graph at reducing AI-agent regressions? This page is
for the reviewer who wants to check the harness **without running the benchmark**.

The harness is finished when it is complete, tested, and correctly refusing. It is not
finished when results exist, and there are no results: the run is gated on a number a human
has not yet written. The correct terminal state of everything described here is
`preflight.py` declining to launch.

## Reproduce it locally

`uv` must be on `PATH` (see [DEVELOPMENT.md](../../DEVELOPMENT.md)). Nothing below needs a
model, a GPU, a network or Docker.

```bash
cd bench/swebench
uv sync
uv run pytest -q                       # the whole bench suite
uv run pytest tests/test_acceptance.py -q   # the acceptance gate on its own
uv run python preflight.py             # MUST refuse: exit 3
uv run python run_arm.py --dry-run     # 5 arms x 100 instances, no model: exit 0
uv run python run_arm.py rtdd          # MUST refuse: exit 3, before any model call
```

Or run the whole gate the way CI does, from the repository root:

```bash
scripts/ci-prereg.sh     # the bench half
scripts/ci-local.sh      # the authoritative gate: Go build/vet/test + the bench half
```

## What a reviewer should expect to see refuse

`bench/PREREGISTRATION.md` ships `status: UNSIGNED` with `stratified_recall_floor:`
deliberately valueless. That is the kill criterion — the Axis-2 stratified change-level
recall below which RTDD's selector is not published, whatever Axis 1 says. No agent may
write, guess, default or infer it, so the harness is inert until a human does.

So on the repository as committed:

```
$ uv run python preflight.py ; echo "exit $?"
PREFLIGHT REFUSED: .../bench/PREREGISTRATION.md: missing or blank pre-registered fields:
['signed_at', 'signed_by', 'stratified_recall_floor']
exit 3
```

and every one of the five arms refuses the same way, **before** a workspace is prepared, a
prompt is built, or a token is spent:

```
$ uv run python run_arm.py rtdd ; echo "exit $?"
PREFLIGHT REFUSED: ... ['signed_at', 'signed_by', 'stratified_recall_floor']
exit 3
```

`run_arm.py` calls `preflight` on its first line for exactly this reason: the gate cannot be
something the driver remembers to check. `tests/test_acceptance.py` runs both commands as
subprocesses and asserts the exit code, the message, that no raw record was written, and
that no cost file was charged.

There are five ways the pre-registration can be wrong, and each gets its own message:

| state | what the gate says |
|---|---|
| `stratified_recall_floor` absent | `missing or blank pre-registered fields: ['stratified_recall_floor']` |
| present and blank (**the shipped state**) | the same list — a key with no value is not a value |
| present but not a number (`TBD`) | `stratified_recall_floor must be numeric and chosen before the run, got 'TBD'` |
| complete but `status: UNSIGNED` | `status is 'UNSIGNED', not SIGNED — a human must sign this` |
| signed but not tagged `prereg-m4` | `the tag prereg-m4 does not exist — the pre-registration has not been frozen` |

A sixth state passes: signed, tagged, and describing the very `instances.txt` about to be
run — verified by sha256 and by count, so the sample cannot be redrawn after signing.

## What the dry run proves

`run_arm.py --dry-run` assembles all five arms' prompts for all 100 frozen instances with no
model call and no network, and asserts the structural guarantee on the bytes an instance
would actually be handed: every context arm is **its own control plus exactly one
`<test-context>` block, byte for byte**, and that block survives `prompts.lint_context`'s
imperative blocklist.

That guarantee is what stops the RTDD arm from quietly becoming a TDD-protocol arm. TDAD
measured procedural TDD prose at 9.94% regressions against a 6.08% vanilla baseline — worse
than no intervention — so an RTDD arm carrying a smuggled procedure would be measuring their
result and publishing it as ours.

It is load-bearing, and `tests/fixtures/` ships the proof as two deliberately failing
inputs:

- `procedural_rtdd_context.md` — the RTDD context with a TDD protocol written *into* the
  block. The imperative lint fails on it (`follow a`, `workflow`, `test-driven`, `do not
  skip`, …) and `run_arm.py --dry-run` exits 5.
- `procedural_rtdd_trailing.md` — a procedural sentence appended *after* the closing tag,
  where no lint of the block could see it. The byte-equality assertion fails instead:
  `rtdd: assembled prompt is not vanilla plus one context block, byte for byte`.

Between them, prose cannot enter the RTDD arm through either door.

## How a regression number is produced

Mechanically, and only mechanically. A regression is a test id in the official harness
report's `tests_status.PASS_TO_PASS.failure` list; the denominator is the **dataset's**
`PASS_TO_PASS` cardinality, never the report's, so an evaluation that crashed or lost tests
cannot shrink what the rate divides by. Resolution rate is printed beside the regression rate
in every row, so an arm cannot look good by doing nothing.

The pipeline is `raw/<arm>/<id>.json` → `predictions/<arm>.jsonl` → `evaluate.collect` →
`analyze.analyze` → `report.render` → `tables.md`, and the acceptance suite walks it end to
end over fixture reports — no Docker, no model — then traces every published figure back to
the failure entries it came from.

## Scope: what this harness does not do

`docs/plans/05-m4-m5-swebench-release.md` is the plan. Tasks 1 and 3–12 are landed, each as
its own commit:

| plan task | what it landed | commit |
|---|---|---|
| Task 1 | pre-registration schema and the preflight gate | `3c26820` (#137) |
| Task 3 | the frozen instance sample at seed 20260826 | `164f18b` (#138) |
| Task 4 | "regression" as a `PASS_TO_PASS` failure | `7e997ba` (#139) |
| Task 5 | arm composition and the non-procedural guarantee | `0128cff` (#140) |
| Task 6 | workspace worktrees, tool set, agent loop | `cf6aebc` (#141) |
| Task 7 | TDAD provider and its not-installable contingency | `e53cc3d` (#142) |
| Task 8 | RTDD provider: cached seeds and `rtdd_which` | `6ee0e47` (#143) |
| Task 9 | arm driver, budget ceilings, resumability, dry run | `b424889` (#144) |
| Task 10 | evaluation via the official SWE-bench Docker harness | `c5c14b5` (#145) |
| Task 11 | rates, CIs, equivalence band, kill criterion | `ff9054c` (#146) |
| Task 12 | results tables with resolution beside regression | `828c515` (#147) |

Three tasks are **out of scope** for this PRD, all three because they are human-in-the-loop:

- **Task 2 — choose and sign the kill criterion.** A human picks `stratified_recall_floor`
  (the plan lays out 0.90 / 0.95 / 0.99 and what each buys), writes `signed_by` and
  `signed_at`, sets `status: SIGNED`, commits, and runs `git tag prereg-m4`. *Unblocks:*
  everything below. Until then `preflight.py` exits 3 and that is the harness working.
- **Task 13 — run the benchmark.** Needs Task 2's signature, a locally served
  `Qwen3-Coder-30B-A3B-Instruct`, and Docker for the official evaluation harness.
  *Unblocked by:* Task 2.
- **Task 14 — go/no-go on publishing.** A human reads the tables, the equivalence band and
  the kill-criterion verdict, and decides. *Unblocked by:* Task 13.

No agent may close the gap by writing the number itself. That is the one edit which would
make every figure downstream unpublishable, and it is the only thing this harness refuses to
do for you.
