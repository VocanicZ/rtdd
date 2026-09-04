# `bench/results/` — committed, diffable benchmark output

This directory is **committed on purpose** and is not gitignored. A benchmark
number that lives only on the machine that produced it cannot be reviewed, so the
results are in git and a re-run at an identical config must produce identical
bytes: `git diff --stat bench/results/<repo_id>/` is the check.

`bench/replay/report.py` writes it. One directory per repo:

```
bench/results/<repo_id>/
  commits.jsonl   one JSON line per record, sorted by (repo_id, commit, variant, kind, strategy)
  summary.json    every published metric, sorted keys, newline-terminated
  summary.md      the published per-repo table, including the `verdict:` line
  config.json     the RunConfig, its digest and the Hardware that produced the other three
  drift.json      the cycles-since-commit curve, only when a drift session was run
bench/results/aggregate.md   the sole cross-repo file — duration-weighted and labelled as such
```

Three rules the layout enforces:

- **No result without its config.** `config.json` is written on every emission and
  carries the corpus digest, the tool versions, the strategy set, the identity of
  the `rtdd` binary under test and the hardware fingerprint.
- **Never pooled.** Recall is never pooled across repos, and `natural` is never
  pooled with `probe` — `probe` seeds map-based strategies at the child commit, so
  its rows are an upper bound and are labelled as one on the same line as the
  numbers.
- **The verdict is unconditional.** Every `summary.md` carries a `verdict:` line
  comparing RTDD against the naive path heuristic, printed win or lose.

`bench/results/.cache/` is the one thing here that *is* gitignored.

## The first published table

`bench/results/flask/` is the first real replay: 25 consecutive commits of `flask` at
the frozen pin, both variants, all eight strategies, on disclosed hardware off CI.
Reproduce it with

```bash
cd bench && uv run python -m replay.cli replay --repo flask --replay-commits 25 --wallclock-sample 10
```

and `git diff --stat bench/results/flask/` must come back empty — a cached, identical
config reproduces the same bytes, selection timings included.

Its verdict fell against RTDD, and
[`docs/results/axis2-first-replay.md`](../../docs/results/axis2-first-replay.md) records
that rather than the run being repeated with different settings: the `natural`
population had no detecting commits at this length, so the pre-registered criterion is
undecided there, and on the `probe` upper bound RTDD wins recall but not at equal or
better selected duration, which is what the criterion asks for.

## Re-freezing the corpus without orphaning what is already published

`corpus.yaml` is content-addressed: `corpus.lock` holds the sha256 of its bytes, and
`load_corpus` refuses to run against a file that has drifted from it. Every result here
stamps that digest in its `config.json`, so editing the corpus once meant invalidating
every published number — which is why v1 shipped a repo it should not have and could
not cheaply take it back (#184).

The lock is now a **history**, one line per version, and superseded versions are kept
verbatim under `bench/corpus.d/`:

```
# corpus.lock
1 95cd1505766b560d0a595ee874531eb98e95522efdd06fdf6a6a1680c40a9381
2 6145ea8f568d1c4d02b576f1d4d73058688a81f670ba1669088b36f4714fa432
```

A result published under an older digest stays reproducible against the corpus it was
produced under:

```bash
cd bench && uv run python -m replay.cli replay --repo flask --replay-commits 25 \
  --wallclock-sample 10 --corpus-version 1
```

To amend the corpus:

1. `cp bench/corpus.yaml bench/corpus.d/v<current>.yaml` — **before** editing. A version
   that is not archived at its exact bytes is a version whose results cannot be checked.
2. Edit `corpus.yaml` and bump its `corpus_version`.
3. Re-freeze: `freeze()` appends the new version to the lock and keeps every prior line.
   Deleting a line orphans every result published under it.
4. `uv run python -m replay.cli audit` — it must exit 0. A repo that misses a threshold
   in `admission:` fails the command; it does not warn.
5. `uv run python -m replay.cli report` to regenerate `aggregate.md` for the new corpus.

`tests/test_audit.py` asserts the contract end to end, including that every
`config.json` in this tree still names a digest the lock knows.

## Results for repos the corpus no longer admits

`results/sqlfluff/` is published under corpus v1 and is **kept**: deleting a number that
was published is worse than superseding it. It is not weighed in `aggregate.md`, which
spans the current corpus, and `replay.cli report` skips any results directory the corpus
does not admit. Why it was dropped, and what dropping it did to the aggregate, is in
[`docs/results/axis2-corpus-admission.md`](../../docs/results/axis2-corpus-admission.md).
