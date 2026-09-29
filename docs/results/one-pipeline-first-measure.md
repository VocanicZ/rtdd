# One pipeline, first measurement — 2026-09-29

A mechanical measurement of the one-pipeline binary against the release before it, on the
four rtdd-bench `rtdd` workspaces. It is not the agent A/B the spec asks for (that needs
human-driven sessions and has not been run); it answers two narrower questions: does every
row that used to run at the static tier now get a recorded map, and what does one edit
cycle cost.

## Method

For each project and each binary:

1. Fresh `git clone` of the rtdd-bench workspace (`examples/<project>/<lang>/rtdd`).
2. Delete `.rtdd/map.jsonl` and `.rtdd/meta.json`, commit, then `rtdd seed`.
3. Append one no-op function to one source file (`describe.go`, `report.go`,
   `cron/describe.py`, `ledger/cli.py`).
4. `rtdd which --json` (tier and selection size read from the document), then `rtdd run`.
5. The full suite for comparison: `go test -count=1 ./...` or `pytest -q -p no:cacheprovider`.

`old` is the release before the one-pipeline change (`main`); `new` is `feat/one-pipeline` at
296b21f, before the final review's fix wave. One run per cell, wall-clock milliseconds, one
Linux workstation. Nothing was repeated or averaged, so differences of a few tens of
milliseconds are noise.

## Results

| project | bin | seed ms | which ms | tier | selected | run ms | full suite ms |
|---|---|---:|---:|---|---|---:|---:|
| cron-parser/go | old | n/a (static: no map) | — | TS | whole package | 288 | 178 |
| cron-parser/go | new | 251 | 36 | T0 | 2 test files | 366 | 273 |
| ledger/go | old | n/a (static: no map) | — | TS | whole package | 299 | 167 |
| ledger/go | new | 311 | 34 | T0 | 4 test files | 458 | 267 |
| cron-parser/python | old | 641 | 27 | T0 | 15 test cases | 376 | 319 |
| cron-parser/python | new | 613 | 32 | T0 | 2 of 6 files | 497 | 296 |
| ledger/python | old | 681 | 30 | T0 | 17 test cases | 441 | 377 |
| ledger/python | new | 659 | 32 | T0 | 4 of 6 files | 593 | 370 |

## Reading

- **Go now gets a recorded map.** The old binary could not seed Go (static tier, no map) and
  ran the whole package. The new one seeds, selects at T0 from recorded coverage, and on
  cron-parser selects `cli_test.go`, which is not name-correspondent to the edited
  `describe.go` — the case the static tier missed.
- **On these tiny suites `rtdd run` is slower than the full suite, for both binaries.** The
  full suites take 170–380 ms; one process per test file costs more than that saves. The new
  Python run is about 30% slower than the old one (497 vs 376 ms, 593 vs 441 ms): the old
  binary ran the selected test cases in one pytest process, the new one starts a process per
  selected file.
- Seed times are about the same for Python and are new for Go.

This is published as measured: slower is slower. Whether the edit loop pays off on larger
suites, and in agent sessions, is still to be measured.
