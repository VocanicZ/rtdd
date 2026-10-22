# Round 1 and Rounds 1+2 against /tdd, per corpus row — 2026-10-22

The v0.3.0 node graph on rtdd-bench: for every row of `bench/corpus.yaml`, how much of the
full suite's time `rtdd` spends in tests when it runs Round 1 alone (`rtdd`) and when it
runs Rounds 1+2 (`rtdd-r12`), and how many of the full suite's failures each one still
catches. `/tdd` — the full suite every cycle — is the `full` arm. It answers PRD #412's
question for this corpus and nothing wider: it is a mechanical replay of real commits,
not an agent session, and with two corpus rows it is two measurements, not a trend. Both
rows are published as they fell; neither was tuned, dropped or re-run.

## Method

- **Binary:** `rtdd 0.3.0-dev (commit 7c4721065202961a19cdd4e87dfa81ef65b72c48, built 2026-10-07T23:55:22Z)`
  — `main` with Round 1 and Rounds 1+2 as bench arms (#485), recorded verbatim in each
  row's `config.json`.
- **Corpus:** `bench/corpus.yaml` v2, digest `6145ea8f568d`; every row, in corpus order —
  `httpie` (17 commits, the corpus's measured ceiling) and `flask` (25 commits). The
  corpus's own commit counts and both variants: `natural` (map seeded at the parent, as
  an agent would have it) and `probe` (map seeded at the child commit — an **upper
  bound** on selection quality, never pooled with `natural`).
- **Arms:** `rtdd` (Round 1), `rtdd-r12` (Rounds 1+2), `path` (the pre-registered
  `tests/test_<module>.py` heuristic) and `full` (`/tdd`). Shipped defaults only.
- **Hardware:** Intel(R) Xeon(R) CPU E5-2697A v4 @ 2.60GHz, 32 cores, 32768 MiB,
  Linux-7.0.0-3-pve-x86_64-with-glibc2.41, Python 3.12.13, fingerprint `65b8fad78747d6ef`,
  off CI (wall-clock permitted).
- **Recall** is over the cycles where the full suite failed (`F_full` non-empty): change
  recall counts cycles where the arm selected at least one failing test, test recall
  (micro) counts failing tests. **Time in tests** is the selected tests' summed
  durations, from the same commit's full-suite run; **time vs /tdd** is that over the
  full suite's, summed over cycles.
- Each row was run once. `flask` skipped 4 commit/variant pairs whose tree would not
  collect (`collect-error-or-empty`, `no-tests-collected`); they are listed under
  `skipped` in its `summary.json`. `httpie` skipped none.

The records are `bench/results/n4/<repo_id>/`; every figure below is copied from that
row's `summary.json` as `report.fmt` renders it, and `bench/tests/test_results_n4_rounds.py`
fails if the two drift. The published v0.2 results under `bench/results/<repo_id>/` are
untouched.

## Results

### `natural`

| row | arm | change recall | test recall (micro) | time in tests (ms) | time vs /tdd |
|---|---|---|---|---:|---|
| `httpie` | `rtdd` — Round 1 | 0.000 (0/2) | 0.000 (0/2) | 49875 | 0.022 (49875/2.24895e+06) |
| `httpie` | `rtdd-r12` — Rounds 1+2 | 0.000 (0/2) | 0.000 (0/2) | 54611 | 0.024 (54611/2.24895e+06) |
| `httpie` | `full` — `/tdd` | 1.000 (2/2) | 1.000 (2/2) | 2248947 | 1.000 (2.24895e+06/2.24895e+06) |
| `flask` | `rtdd` — Round 1 | n/a (0/0) | n/a (0/0) | 6443 | 0.178 (6443/36239) |
| `flask` | `rtdd-r12` — Rounds 1+2 | n/a (0/0) | n/a (0/0) | 11896 | 0.328 (11896/36239) |
| `flask` | `full` — `/tdd` | n/a (0/0) | n/a (0/0) | 36239 | 1.000 (36239/36239) |

### `probe` — upper bound (map seeded at the child commit)

| row | arm | change recall | test recall (micro) | time in tests (ms) | time vs /tdd |
|---|---|---|---|---:|---|
| `httpie` (probe, upper bound) | `rtdd` — Round 1 | 0.000 (0/3) | 0.000 (0/3) | 0 | 0.000 (0/2.25641e+06) |
| `httpie` (probe, upper bound) | `rtdd-r12` — Rounds 1+2 | 0.000 (0/3) | 0.000 (0/3) | 2239 | 0.001 (2239/2.25641e+06) |
| `httpie` (probe, upper bound) | `full` — `/tdd` | 1.000 (3/3) | 1.000 (3/3) | 2256413 | 1.000 (2.25641e+06/2.25641e+06) |
| `flask` (probe, upper bound) | `rtdd` — Round 1 | 0.667 (2/3) | 0.667 (2/3) | 1428 | 0.036 (1428/39557) |
| `flask` (probe, upper bound) | `rtdd-r12` — Rounds 1+2 | 0.667 (2/3) | 0.667 (2/3) | 6829 | 0.173 (6829/39557) |
| `flask` (probe, upper bound) | `full` — `/tdd` | 1.000 (3/3) | 1.000 (3/3) | 39557 | 1.000 (39557/39557) |

### Pre-registered verdicts, as each row's `summary.md` states them

`httpie`:

> verdict: change-level recall rtdd=0.000 vs path heuristic=0.000; selected-duration fraction rtdd=0.022 vs path=0.023 — rtdd does NOT clearly beat the naive path heuristic — per the pre-registered criterion in docs/plans/04-m3-replay-benchmark.md the map is not justified and this must be stated in the README

> verdict (probe, upper bound — map seeded at the child commit, never pooled with `natural`): change-level recall rtdd=0.000 vs path heuristic=0.000; selected-duration fraction rtdd=0.000 vs path=0.000 — rtdd does NOT clearly beat the naive path heuristic — per the pre-registered criterion in docs/plans/04-m3-replay-benchmark.md the map is not justified and this must be stated in the README

`flask`:

> verdict: not computable (no detecting commits)

> verdict (probe, upper bound — map seeded at the child commit, never pooled with `natural`): change-level recall rtdd=0.667 vs path heuristic=0.333; selected-duration fraction rtdd=0.036 vs path=0.009 — rtdd does NOT clearly beat the naive path heuristic — per the pre-registered criterion in docs/plans/04-m3-replay-benchmark.md the map is not justified and this must be stated in the README

## Reading

- **`httpie`: both arms cut time in tests to about 2% of `/tdd`, and both missed every
  failure.** Round 1 spent 0.022 of the full suite's time, Rounds 1+2 0.024 — Round 2
  added 4736 ms over 17 cycles. Neither caught either `natural` detecting cycle, nor any
  of the three `probe` ones; `/tdd` caught all of them. Every one of those five cycles
  has the same single failing test, `tests/test_uploads.py::test_stdin_read_warning`,
  on commits that changed only docs, the changelog, man pages, the fish completion
  script or `httpie/__init__.py`. The `full` arm also recorded 6 isolation violations in
  34 runs (its subset run did not reproduce `F_full`), and the wall-clock p50 of the
  full suite is 137 s. Those facts are recorded beside the zero; they do not replace
  it: on this row, against `/tdd`, both arms missed what the full suite caught.
- **`flask`, `natural`: both arms cut time in tests; recall is not measured.** None of
  the 23 replayed cycles had a failing test, so there is no recall denominator, for
  either arm or for `/tdd`. Round 1 spent 0.178 of `/tdd`'s time in tests, Rounds 1+2
  0.328 — Round 2 nearly doubles Round 1's time and still runs about a third of the
  full suite's.
- **`flask`, `probe` (upper bound): both arms caught two of three, and Round 2 caught
  nothing Round 1 missed.** Round 1 spent 0.036 of `/tdd`'s time, Rounds 1+2 0.173. The
  miss is the same commit for both: `89992954ec` changed `src/flask/sansio/scaffold.py`,
  Round 1 selected no tests at all, and Rounds 1+2 selected 252 that did not include
  the failing `tests/test_basic.py::test_method_route[query]`. `/tdd` caught all three.
- **Round 2's price.** On both rows Rounds 1+2 cost more time than Round 1 alone
  (`httpie` 0.024 vs 0.022 natural; `flask` 0.328 vs 0.178 natural, 0.173 vs 0.036 probe)
  and bought no recall on any detecting cycle in this corpus.
- **Against the pre-registered criterion,** neither row's verdict is a pass: `httpie`
  says rtdd does not clearly beat the path heuristic on either variant, and `flask`'s
  `natural` verdict is not computable while its `probe` verdict says rtdd does not
  clearly beat the path heuristic either. `bench/PREREGISTRATION.md` is left unsigned:
  this page reports the measurement, it does not decide the criterion's floor.

This is published as measured: two rows, one run each, nothing re-run to move a number.

## Reproduce

Build the binary under test from its commit and put it first on `PATH`; the harness
itself (with `--results-label`) is this page's checkout:

```bash
git worktree add /tmp/rtdd-7c47210 7c4721065202961a19cdd4e87dfa81ef65b72c48
bin="$(mktemp -d)" && (cd /tmp/rtdd-7c47210 && go build -o "$bin/rtdd" ./cmd/rtdd)
export PATH="$bin:$PATH" && rtdd --version
```

Then, from `bench/`, once per corpus row in corpus order — no `--replay-commits`, no
`--commit-selection`, no `--variants`:

```bash
cd bench
uv run python -m replay.cli replay --repo httpie --strategies rtdd,rtdd-r12,path,full --results-label n4
uv run python -m replay.cli replay --repo flask --strategies rtdd,rtdd-r12,path,full --results-label n4
```

`--results-label n4` writes to `bench/results/n4/<repo_id>/`, beside the published
v0.2 results and never on them. A re-run at the same config on the same hardware
reproduces `bench/results/n4/` byte for byte from `bench/cache/`:
`git diff --stat bench/results/n4/` comes back empty.
