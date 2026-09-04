# Outcome READMEs — the negative branch, ready to swap

Issue #200 (spec §15, plan Task 23). RTDD publishes either way: if the M4 SWE-bench Verified
result shows TDAD's static graph matching or beating RTDD, the repo says so and stops
recommending itself, rather than quietly shipping a losing tool. **Which branch is live is a
human call (plan Task 14 / Task 24) — this directory only makes the swap mechanical once that
call is made.**

## Files

| file | what it is |
|---|---|
| `README.positive.md` | Copy of the root `README.md` as of #199/#209 — RTDD wins or the run is still pending. |
| `README.negative.md` | Full negative-outcome README from plan Task 23 — RTDD does not beat TDAD's static graph. Its result numbers are the literal placeholder `<!-- PASTE bench/results/swebench/tables.md HERE -->`, never invented ahead of the run. |
| `SELECTED` | One word, `positive` or `negative` — the single source of truth for which outcome is currently active at the repo root. |

The root `README.md` must always be byte-identical to `README.<SELECTED>.md`. `outcomes_test.go`
(repo root, `go test ./...`, runs in CI) asserts this on every push and PR, so the two READMEs
cannot silently diverge.

## What ships if the negative branch activates

| artifact | ships? | why |
|---|---|---|
| `docs/results/*-swebench-verified.md` | **yes** | The result is the deliverable. |
| `bench/` in full — harness, pre-registration, raw results | **yes** | A negative result nobody can rerun is not a result. |
| `bench/PREREGISTRATION.md` + `prereg-m4` tag | **yes** | It is what makes the negative credible. |
| `protocol/PROTOCOL.md`, `dist/`, `rtdd-gen` | **yes**, in-repo | Reusable, and the front-end generator is independently useful. |
| GoReleaser binaries, `install.sh`, a tagged release | **no** | Do not ship a competitor to a tool that beat you. |
| `rtdd init` as a recommended workflow | **no** | The README stops recommending it. |
| A PR to `pepealonso95/TDAD` adding a coverage-derived backend | **yes** | The contribution goes where it measured better — see "Upstream contribution" below. |

## How to activate the negative branch (human steps)

This repo will not do any of this on its own — issue #200 changes no repository state that is
a human decision. No tag, no visibility change, no unilateral outcome declaration, and the root
`README.md` is not swapped by an agent.

1. Fill `README.negative.md`'s `<!-- PASTE bench/results/swebench/tables.md HERE -->` placeholder
   with the actual contents of `bench/results/swebench/tables.md` once the M4 run (plan Task 13)
   has completed and Task 14's human decision selected this branch.
2. `cp docs/outcomes/README.negative.md README.md`.
3. Write `negative` to `docs/outcomes/SELECTED` (overwriting `positive`).
4. Commit all three changes together — `outcomes_test.go` fails the build otherwise.
5. Push. `.github/workflows/release.yml`'s `check-outcome` job reads `SELECTED`; its
   `goreleaser` job carries `if: needs.check-outcome.outputs.selected != 'negative'`, so a `v*`
   tag pushed after this point builds no binaries. Re-enable only after a new, separately
   pre-registered benchmark says otherwise.

To swap back to the positive branch, reverse the same three steps against `README.positive.md`
and write `positive` to `SELECTED`.

## Upstream contribution

If the negative branch activates, the §15 commitment is not just publishing the result — it is
contributing the part of RTDD that might still be additive back to the tool that won: a PR to
[`pepealonso95/TDAD`](https://github.com/pepealonso95/TDAD) adding a coverage-derived impact
strategy (reading `.coverage` SQLite directly for per-test contexts) alongside its four existing
strategies, plus the import-time classification RTDD had to build to handle Python's
collection-time attribution gap. That PR is a human/agent action taken *at* activation time, not
something this issue opens — no PR to an external repo is opened here.
