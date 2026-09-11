"""The benchmark's single entry point, and the only place its guards are enforced.

Every Global Constraint the harness rests on — the corpus is frozen, the repo is
in it, wall-clock numbers never come off a runner — is checked here, once, before
any work happens. The library layers raise the same errors, but a guard that only
the library enforces is a guard the caller can route around by importing one level
deeper; this module is what a human and CI both actually run.
"""

from __future__ import annotations

import argparse
import dataclasses
import json
import os
import pathlib
import sys
from collections.abc import Sequence

from replay import rtddio
from replay.cache import Cache
from replay.config import TRACKED_TOOLS, RunConfig, canonical_json, tool_versions
from replay.corpus import CorpusError, audit, load_corpus
from replay.derive import DERIVED_ARMS, DerivationError, derive_static
from replay.envsetup import EnvError, activate, provision, tool_versions_for, with_source_path
from replay.gitwork import add_worktree, clone_pinned, remove_worktree, replay_points
from replay.hardware import CIWallClockRefused, Hardware, probe, require_wallclock
from replay.replay import ReplayOptions, ReplayOutput, replay_repo, strategy_order
from replay.records import (
    CommitRecord,
    StrategyRecord,
    UncoveredRecord,
    WallClockRecord,
    parse_jsonl_lines,
    to_jsonl_lines,
)
from replay import chart
from replay.report import build_summary, render_aggregate, render_markdown, write_results
from replay.session import run_drift
from replay.strategies import base as sbase
from replay.strategies import (  # noqa: F401  side-effect registration
    full as _full,
    importgraph as _ig,
    lastfailed as _lf,
    pathheuristic as _ph,
    randomratio as _rr,
    rtdd as _rtdd,
    testmon as _tm,
    xdist as _xd,
)

BENCH = pathlib.Path(__file__).resolve().parents[1]
CORPUS = BENCH / "corpus.yaml"
LOCK = BENCH / "corpus.lock"
WORK = BENCH / "work"
ENVS = BENCH / "work" / "envs"
CACHE = pathlib.Path(os.environ.get("RTDD_BENCH_CACHE") or BENCH / "cache").expanduser()
"""Ground truth is expensive enough to outlive the checkout that produced it.
The fleet reaps and recreates its worktree on every claim, which takes a
``bench/cache/`` down with it and makes every claim pay the ground truth again, so
``RTDD_BENCH_CACHE`` can point the store somewhere durable. Entries stay keyed by the
config digest either way, so a longer-lived cache is not a staler one."""
RESULTS = BENCH / "results"
FIGURES = BENCH.parent / "docs" / "results" / "figures"

DEFAULT_STRATEGIES = ("rtdd", "testmon", "path", "lf", "importgraph", "xdist", "random", "full")

#: A run that produced what it promised.
EXIT_OK = 0
#: A Global Constraint refused the run: an unfrozen corpus, or a repo outside it.
#: argparse uses this code for usage errors too, and that is the same class of
#: answer — the invocation was not one the harness is willing to honour.
EXIT_GUARD = 2
#: `doctor` only: the environment cannot publish a result, whatever the arguments.
EXIT_ENV = 3


def _corpus(version: int | None = None):
    return load_corpus(CORPUS, LOCK, version=version)


def _config(
    corpus_digest: str,
    args,
    strategies: Sequence[str],
    replay_commits: int,
    version: str | None = None,
    tools: Sequence[tuple[str, str]] | None = None,
) -> RunConfig:
    if version is None:
        try:
            version = rtddio.rtdd_version(args.rtdd_binary)
        except Exception:
            version = "absent"
    return RunConfig(
        corpus_digest=corpus_digest,
        rtdd_version=version,
        tool_versions=tuple(tools) if tools is not None else tool_versions(),
        strategies=tuple(sorted(strategies)),
        variants=(
            tuple(args.variants.split(",")) if getattr(args, "variants", None) else ("natural",)
        ),
        replay_commits=replay_commits,
        wallclock_sample=getattr(args, "wallclock_sample", 0),
        random_seed=getattr(args, "seed", 1),
    )


def _wallclock_enabled(hw: Hardware, args) -> bool:
    """Decide whether this run may time anything, and say so out loud if it may not.

    The refusal is :class:`CIWallClockRefused`, raised by the same library guard
    the timing code uses. The CLI catches it rather than dying on it: spec §10
    forbids *publishing* wall-clock from a runner, and every other metric — recall,
    selection ratio, isolation — is hardware-independent and still worth computing.
    """
    if getattr(args, "no_wallclock", False):
        return False
    try:
        require_wallclock(hw)
    except CIWallClockRefused as exc:
        print(str(exc), file=sys.stderr)
        return False
    return True


def cmd_doctor(args) -> int:
    hw = probe()
    blockers: list[str] = []
    try:
        corpus = _corpus()
        digest, ids, excluded = corpus.digest, corpus.ids(), len(corpus.excluded)
        version_of = corpus.version
        breaches = audit(corpus)
    except CorpusError as exc:
        print(f"corpus: ERROR — {exc}")
        digest, ids, excluded, version_of, breaches = "unavailable", (), 0, "?", ()
        blockers.append("the corpus is not frozen or cannot be read")
    print(f"hardware: {hw.cpu_model}, {hw.cpu_count} cores, {hw.mem_total_kb // 1024} MiB")
    print(f"platform: {hw.platform} · Python {hw.python_version}")
    print(f"wall-clock: {'SUPPRESSED (CI: ' + hw.ci + ')' if hw.ci else 'permitted'}")
    print(f"corpus digest: {digest} (version {version_of})")
    print(f"corpus repos: {', '.join(ids) or '(none)'} · excluded entries: {excluded}")
    print(f"corpus admission: {'CLEAN' if not breaches else str(len(breaches)) + ' breach(es)'}")
    for f in breaches:
        print(f"  breach: {f}")
    if breaches:
        blockers.append("an admitted repo breaches the corpus's own admission criteria")
    print(f"strategies: {', '.join(strategy_order(DEFAULT_STRATEGIES))}")
    try:
        version = rtddio.rtdd_version(args.rtdd_binary)
        print(f"rtdd binary: {version}")
    except Exception as exc:
        version = "absent"
        print(f"rtdd binary: MISSING ({exc})")
        blockers.append("the rtdd binary under test is not on PATH or on disk")
    for name, ver in tool_versions():
        print(f"tool: {name} {ver}")
    cfg = _config(digest, args, DEFAULT_STRATEGIES, 0, version=version)
    print(f"config digest: {cfg.digest()}")
    if blockers:
        print(f"publishable: no — {'; '.join(blockers)}")
        return EXIT_ENV
    # CI is not a blocker: it costs this environment the wall-clock table and
    # nothing else, and the suppression is disclosed on the line above.
    caveat = " (wall-clock suppressed on CI; every other metric stands)" if hw.ci else ""
    print(f"publishable: yes{caveat}")
    return EXIT_OK


def results_dir_for(repo_id: str, commit_selection: str) -> pathlib.Path:
    """Where a run's results go, keyed by which commits it chose to replay.

    A non-default commit selection writes to its own subtree and can never land on
    `bench/results/<repo>/`. That directory holds the PUBLISHED result — the one the
    README's tables, `aggregate.md` and `outcomes_test.go`'s derived verdict all read —
    and it was produced under the `recent` rule over 46 commits. A six-commit `paired`
    run overwriting it silently replaces the published corpus with a differently-selected
    population that happens to share a filename, which is exactly the substitution a
    pre-registration exists to prevent. Measured the hard way: it happened once, and the
    files had to be restored from git.
    """
    if commit_selection and commit_selection != "recent":
        return RESULTS / commit_selection / repo_id
    return RESULTS / repo_id


def cmd_replay(args) -> int:
    hw = probe()
    corpus = _corpus(getattr(args, "corpus_version", None))
    try:
        spec = corpus.require(args.repo)
    except CorpusError as exc:
        print(str(exc), file=sys.stderr)
        return EXIT_GUARD
    wallclock_enabled = _wallclock_enabled(hw, args)
    strategies = tuple(args.strategies.split(",")) if args.strategies else DEFAULT_STRATEGIES
    if "random" in strategies:
        peer = sbase.get("random").peer
        if peer not in strategies:
            print(
                f"'random' is ratio-matched against {peer!r}'s selection size; "
                f"{peer!r} must run in the same --strategies list. "
                f"got: {','.join(strategies)}",
                file=sys.stderr,
            )
            return EXIT_GUARD
    commits = args.replay_commits or spec.replay_commits
    spec = dataclasses.replace(spec, replay_commits=commits)
    repo = clone_pinned(spec.url, spec.pin, WORK / "repos" / spec.id)
    try:
        env = provision(spec, repo, ENVS)
    except EnvError as exc:
        print(str(exc), file=sys.stderr)
        return EXIT_ENV
    os.environ.update(activate(env))
    os.environ.pop("PYTHONHOME", None)
    cfg = _config(
        corpus.digest,
        args,
        strategies,
        commits,
        tools=tool_versions_for(env.python, TRACKED_TOOLS),
    )
    output = replay_repo(
        repo=repo,
        spec=spec,
        cfg=cfg,
        cache=Cache(CACHE, cfg.digest()),
        hw=hw,
        work_root=WORK / "trees",
        opts=ReplayOptions(
            variants=cfg.variants,
            strategy_ids=strategies,
            wallclock_sample=args.wallclock_sample,
            wallclock_enabled=wallclock_enabled,
            rtdd_binary=args.rtdd_binary,
            python=str(env.python),
            commit_selection=getattr(args, "commit_selection", "recent"),
        ),
        corpus=corpus,
    )
    out_dir = results_dir_for(spec.id, getattr(args, "commit_selection", "recent"))
    summary = write_results(
        out_dir,
        output,
        cfg,
        hw,
        strategy_order(strategies),
        wallclock_enabled=wallclock_enabled,
    )
    print(
        f"wrote {out_dir} — {summary['n_commits']} commits, "
        f"{len(output.skipped)} skipped"
    )
    return EXIT_OK


def cmd_session(args) -> int:
    hw = probe()
    corpus = _corpus(getattr(args, "corpus_version", None))
    try:
        spec = corpus.require(args.repo)
    except CorpusError as exc:
        print(str(exc), file=sys.stderr)
        return EXIT_GUARD
    repo = clone_pinned(spec.url, spec.pin, WORK / "repos" / spec.id)
    try:
        env = provision(spec, repo, ENVS)
    except EnvError as exc:
        print(str(exc), file=sys.stderr)
        return EXIT_ENV
    os.environ.update(activate(env))
    os.environ.pop("PYTHONHOME", None)
    points = replay_points(repo, spec.pin, args.cycles)
    work = WORK / "trees" / f"{spec.id}-drift"
    try:
        add_worktree(repo, points[0].parent, work)
        # Same reason as in the replay: the editable install pins the clone, and a
        # `.pth` entry sorts after `PYTHONPATH`, so without this the drift session
        # runs the pin's source against an older tree's tests.
        os.environ["PYTHONPATH"] = with_source_path(
            os.environ, work, spec.source_globs
        )["PYTHONPATH"]
        rtddio.seed(work, binary=args.rtdd_binary)
        curve = run_drift(
            repo, spec.id, work, points, python=str(env.python), binary=args.rtdd_binary
        )
    finally:
        remove_worktree(repo, work)
    cfg = _config(
        corpus.digest,
        args,
        DEFAULT_STRATEGIES,
        args.cycles,
        tools=tool_versions_for(env.python, TRACKED_TOOLS),
    )
    out_dir = RESULTS / spec.id
    out_dir.mkdir(parents=True, exist_ok=True)
    # The curve carries its own config rather than overwriting `config.json`: that
    # file stamps the replay that produced `summary.md`, and a drift session runs a
    # different one — its own cycle count, `rtdd` alone.
    (out_dir / "drift.json").write_text(
        canonical_json({**curve.to_dict(), "config": cfg.to_dict(), "hardware": hw.to_dict()}),
        encoding="utf-8",
    )
    config = out_dir / "config.json"
    if not config.exists():
        config.write_text(
            canonical_json({"config": cfg.to_dict(), "hardware": hw.to_dict()}), encoding="utf-8"
        )
    for p in curve.points:
        print(
            f"cycle {p.cycle:3d}  changed {p.changed_files:4d}  "
            f"selected {p.selected:5d}/{p.total_tests:5d}  "
            f"ratio {'n/a (did not collect)' if p.ratio() is None else format(p.ratio(), '.3f')}"
            f"  {p.tier}"
        )
    return EXIT_OK


def _rebuild_repo(d: pathlib.Path) -> dict:
    """Re-derive one repo's `summary.json`/`summary.md` from its committed records.

    Every per-cycle sample the benchmark ever measured is already in
    `commits.jsonl`; before this existed, changing how they are aggregated meant
    re-running the benchmark on hardware that may no longer exist, which in
    practice meant the aggregation was frozen forever. This reads the records and
    the run's own `config.json` back and re-renders — it re-derives, it never
    re-measures, and `config.json` itself is left exactly as the run wrote it.
    """
    records = parse_jsonl_lines((d / "commits.jsonl").read_text(encoding="utf-8").splitlines())
    stamp = json.loads((d / "config.json").read_text(encoding="utf-8"))
    cfg = RunConfig.from_dict(stamp["config"])
    hw = Hardware.from_dict(stamp["hardware"])

    out = ReplayOutput()
    for rec in records:
        if isinstance(rec, CommitRecord):
            out.commits.append(rec)
        elif isinstance(rec, StrategyRecord):
            out.strategies.append(rec)
        elif isinstance(rec, WallClockRecord):
            out.wallclocks.append(rec)
        elif isinstance(rec, UncoveredRecord):
            out.uncovered.append(rec)

    summary_path = d / "summary.json"
    prior = json.loads(summary_path.read_text(encoding="utf-8")) if summary_path.exists() else {}

    # A commit the harness could not replay, and a cycle where `rtdd run` refused,
    # leave nothing in `commits.jsonl` — there was no measurement to record. They
    # are carried across from the published summary rather than re-derived, or a
    # rebuild would quietly republish the run as having skipped and refused nothing.
    out.skipped = list(prior.get("skipped", ()))
    out.rtdd_run_errors = [{}] * int(prior.get("rtdd_run_errors", 0))

    # `--no-wallclock` was a judgement about the host that ran the benchmark — a
    # contended box whose timings would be inflated. A rebuild elsewhere has no
    # standing to overturn it, so a published refusal stays refused.
    wc = prior.get("wallclock", {})
    wallclock_enabled = not (wc.get("suppressed") and "--no-wallclock" in wc.get("reason", ""))

    # The arms the RECORDS carry, not only the arms the run's config lists. A derived
    # arm (`replay.derive`) is computed after the run and appended to `commits.jsonl`;
    # `config.json` stamps the run that produced the numbers and a rebuild has no
    # standing to rewrite it — that list also feeds `cfg.digest()`, which keys the
    # ground-truth cache, so adding `static` there would orphan every entry the
    # benchmark ever measured. Reading the ids out of the records is the more honest
    # rule for a measured arm too: a summary should publish what was recorded.
    recorded = {s.strategy for s in out.strategies}
    ids = strategy_order([*cfg.strategies, *sorted(recorded - set(cfg.strategies))])

    summary = build_summary(out, ids, hw, wallclock_enabled=wallclock_enabled)
    summary_path.write_text(canonical_json(summary), encoding="utf-8")
    (d / "summary.md").write_text(render_markdown(summary, cfg, hw), encoding="utf-8")
    return summary


def cmd_rebuild(args) -> int:
    """`report --rebuild`: re-derive every admitted repo's summary from its records."""
    ids = set(_corpus(getattr(args, "corpus_version", None)).ids())
    rebuilt = []
    for d in sorted(RESULTS.iterdir()) if RESULTS.exists() else []:
        if d.name not in ids or not (d / "commits.jsonl").exists():
            continue
        if not (d / "config.json").exists():
            print(
                f"{d.name}: commits.jsonl with no config.json — refusing to guess "
                "the run that produced it",
                file=sys.stderr,
            )
            return EXIT_GUARD
        _rebuild_repo(d)
        rebuilt.append(d.name)
        print(f"rebuilt {d.name} from {d / 'commits.jsonl'}")
    if not rebuilt:
        print("no per-repo records found; run `replay` first", file=sys.stderr)
        return EXIT_GUARD
    return EXIT_OK


def cmd_derive(args) -> int:
    """`derive`: compute the derived arms from each admitted repo's own records.

    This is the whole of how the `static` arm reaches a published summary without a
    benchmark re-run (PRD #233, spec §7). `bench/results/<repo>/commits.jsonl` already
    holds, per replayed commit, every field the derivation consumes, so this reads that
    file, drops any stale copy of a derived arm, recomputes it from the MEASURED records
    alone and rewrites the file through the same sorted, byte-stable serialiser the run
    used. Re-deriving therefore supersedes its own previous answer instead of appending
    beside it, and a second invocation diffs to nothing.

    It clones nothing, provisions nothing, materialises no worktree, executes no test,
    and never reads or writes the ground-truth cache — the arm executed nothing, so it
    gets no `WallClockRecord` and none is invented. Run `report --rebuild` afterwards to
    re-render the summaries from the records this leaves behind.

    Only repos the corpus currently admits are touched: a results directory for a repo a
    later corpus version dropped stays exactly as it was published.
    """
    ids = set(_corpus(getattr(args, "corpus_version", None)).ids())
    done = []
    for d in sorted(RESULTS.iterdir()) if RESULTS.exists() else []:
        if d.name not in ids or not (d / "commits.jsonl").exists():
            continue
        path = d / "commits.jsonl"
        records = parse_jsonl_lines(path.read_text(encoding="utf-8").splitlines())
        # Stale derived records are dropped BEFORE the derivation, so its answer never
        # depends on whether this file was derived into once already.
        kept = [
            r
            for r in records
            if not (isinstance(r, StrategyRecord) and r.strategy in DERIVED_ARMS)
        ]
        try:
            derived = derive_static(kept)
        except DerivationError as exc:
            print(f"{d.name}: {exc}", file=sys.stderr)
            return EXIT_GUARD
        path.write_text("".join(to_jsonl_lines([*kept, *derived])), encoding="utf-8")
        done.append(d.name)
        print(f"derived static for {d.name} from {path}")
    if not done:
        print("no per-repo records found; run `replay` first", file=sys.stderr)
        return EXIT_GUARD
    return EXIT_OK


def cmd_report(args) -> int:
    if getattr(args, "rebuild", False):
        rc = cmd_rebuild(args)
        if rc != EXIT_OK:
            return rc
    # Only the repos the corpus currently admits. A results directory for a repo a
    # later version dropped is kept — it was published, and deleting a published
    # number is worse than superseding it — but it is not weighed here, or the
    # aggregate would span a corpus that no longer exists.
    ids = set(_corpus(getattr(args, "corpus_version", None)).ids())
    summaries = []
    for d in sorted(RESULTS.iterdir()) if RESULTS.exists() else []:
        f = d / "summary.json"
        if d.name in ids and f.exists():
            summaries.append(json.loads(f.read_text(encoding="utf-8")))
    if not summaries:
        print("no per-repo summaries found; run `replay` first", file=sys.stderr)
        return EXIT_GUARD
    (RESULTS / "aggregate.md").write_text(render_aggregate(summaries), encoding="utf-8")
    print(f"wrote {RESULTS / 'aggregate.md'} from {len(summaries)} repos")
    return EXIT_OK


def cmd_chart(args) -> int:
    """Re-render the committed figures from the committed summaries.

    The figures are what a README reader sees before they read a table, so they are
    generated rather than drawn: `tests/test_chart.py` re-renders them here and fails if
    the committed bytes differ, which makes a figure that disagrees with `summary.json`
    a build failure rather than a thing a reviewer has to notice.
    """
    ids = set(_corpus(getattr(args, "corpus_version", None)).ids())
    summaries = []
    for d in sorted(RESULTS.iterdir()) if RESULTS.exists() else []:
        f = d / "summary.json"
        if d.name in ids and f.exists():
            summaries.append(json.loads(f.read_text(encoding="utf-8")))
    if not summaries:
        print("no per-repo summaries found; run `replay` first", file=sys.stderr)
        return EXIT_GUARD
    written = chart.write_figures(summaries, FIGURES)
    for path in written:
        print(f"wrote {path}")
    print(f"{len(written)} figures from {len(summaries)} repos")
    return EXIT_OK


def cmd_audit(args) -> int:
    """Check the corpus against its own admission criteria. Fails; never warns."""
    corpus = _corpus(getattr(args, "corpus_version", None))
    findings = audit(corpus)
    print(f"corpus version {corpus.version} · digest {corpus.digest[:12]} · repos {', '.join(corpus.ids())}")
    for f in findings:
        print(f"BREACH {f}")
    if findings:
        print(f"{len(findings)} breach(es) — the corpus does not meet its own criteria")
        return EXIT_GUARD
    print("clean — every admitted repo meets every criterion a threshold can decide")
    return EXIT_OK


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(prog="replay", description="RTDD Axis 2 replay benchmark")
    p.add_argument("--rtdd-binary", default="rtdd")
    sub = p.add_subparsers(dest="cmd", required=True)

    d = sub.add_parser("doctor", help="report the resolved environment and whether it can publish")
    d.set_defaults(func=cmd_doctor)

    r = sub.add_parser("replay", help="replay one frozen repo across every strategy")
    r.add_argument("--repo", required=True)
    r.add_argument("--variants", default="natural,probe")
    r.add_argument("--strategies", default="")
    r.add_argument("--wallclock-sample", type=int, default=20, dest="wallclock_sample")
    r.add_argument(
        "--replay-commits",
        type=int,
        default=0,
        dest="replay_commits",
        help=(
            "replay this many commits instead of the corpus's own count; "
            "the number used is published in config.json and summary.md"
        ),
    )
    r.add_argument(
        "--corpus-version",
        type=int,
        default=None,
        dest="corpus_version",
        help=(
            "replay against a superseded corpus version kept in bench/corpus.d/ — "
            "how a result published under an older corpus_digest is reproduced"
        ),
    )
    r.add_argument("--no-wallclock", action="store_true")
    r.add_argument(
        "--commit-selection",
        choices=("recent", "paired"),
        default="recent",
        dest="commit_selection",
        help=(
            "which commits to replay: `recent` (the shipped rule, and what every published "
            "result used) or `paired` — only commits touching BOTH a test file and a "
            "non-test file, the one shape `probe` can detect anything from. `paired` biases "
            "the population toward commits that changed code and tests together; publish "
            "that bias beside any number it produces"
        ),
    )
    r.add_argument("--seed", type=int, default=1)
    r.set_defaults(func=cmd_replay)

    s = sub.add_parser("session", help="replay one uncommitted drift session")
    s.add_argument("--repo", required=True)
    s.add_argument("--cycles", type=int, default=25)
    s.add_argument("--seed", type=int, default=1)
    s.add_argument(
        "--corpus-version",
        type=int,
        default=None,
        dest="corpus_version",
        help=(
            "replay against a superseded corpus version kept in bench/corpus.d/ — "
            "how a result published under an older corpus_digest is reproduced"
        ),
    )
    s.set_defaults(func=cmd_session)

    au = sub.add_parser("audit", help="check the corpus against its own admission criteria")
    au.add_argument("--corpus-version", type=int, default=None, dest="corpus_version")
    au.set_defaults(func=cmd_audit)

    dv = sub.add_parser(
        "derive",
        help="compute the derived arms from committed records; runs no benchmark",
    )
    dv.add_argument("--corpus-version", type=int, default=None, dest="corpus_version")
    dv.set_defaults(func=cmd_derive)

    rep = sub.add_parser("report", help="regenerate aggregate.md from committed summaries")
    rep.add_argument("--corpus-version", type=int, default=None, dest="corpus_version")
    rep.add_argument(
        "--rebuild",
        action="store_true",
        help=(
            "first re-derive each admitted repo's summary.json/summary.md from its own "
            "committed commits.jsonl and config.json — how a published table is "
            "re-rendered without re-running the benchmark"
        ),
    )
    rep.set_defaults(func=cmd_report)

    ch = sub.add_parser("chart", help="regenerate docs/results/figures/ from committed summaries")
    ch.add_argument("--corpus-version", type=int, default=None, dest="corpus_version")
    ch.set_defaults(func=cmd_chart)
    return p


def main(argv: Sequence[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    try:
        return int(args.func(args))
    except CorpusError as exc:
        print(str(exc), file=sys.stderr)
        return EXIT_GUARD


if __name__ == "__main__":
    raise SystemExit(main())
