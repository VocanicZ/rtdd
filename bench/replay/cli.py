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
from replay.corpus import CorpusError, load_corpus
from replay.envsetup import EnvError, activate, provision, tool_versions_for
from replay.gitwork import add_worktree, clone_pinned, remove_worktree, replay_points
from replay.hardware import CIWallClockRefused, Hardware, probe, require_wallclock
from replay.replay import ReplayOptions, replay_repo, strategy_order
from replay.report import render_aggregate, write_results
from replay.session import run_drift
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
CACHE = BENCH / "cache"
RESULTS = BENCH / "results"

DEFAULT_STRATEGIES = ("rtdd", "testmon", "path", "lf", "importgraph", "xdist", "random", "full")

#: A run that produced what it promised.
EXIT_OK = 0
#: A Global Constraint refused the run: an unfrozen corpus, or a repo outside it.
#: argparse uses this code for usage errors too, and that is the same class of
#: answer — the invocation was not one the harness is willing to honour.
EXIT_GUARD = 2
#: `doctor` only: the environment cannot publish a result, whatever the arguments.
EXIT_ENV = 3


def _corpus():
    return load_corpus(CORPUS, LOCK)


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
    except CorpusError as exc:
        print(f"corpus: ERROR — {exc}")
        digest, ids, excluded = "unavailable", (), 0
        blockers.append("the corpus is not frozen or cannot be read")
    print(f"hardware: {hw.cpu_model}, {hw.cpu_count} cores, {hw.mem_total_kb // 1024} MiB")
    print(f"platform: {hw.platform} · Python {hw.python_version}")
    print(f"wall-clock: {'SUPPRESSED (CI: ' + hw.ci + ')' if hw.ci else 'permitted'}")
    print(f"corpus digest: {digest}")
    print(f"corpus repos: {', '.join(ids) or '(none)'} · excluded entries: {excluded}")
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


def cmd_replay(args) -> int:
    hw = probe()
    corpus = _corpus()
    try:
        spec = corpus.require(args.repo)
    except CorpusError as exc:
        print(str(exc), file=sys.stderr)
        return EXIT_GUARD
    wallclock_enabled = _wallclock_enabled(hw, args)
    strategies = tuple(args.strategies.split(",")) if args.strategies else DEFAULT_STRATEGIES
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
        ),
        corpus=corpus,
    )
    summary = write_results(RESULTS / spec.id, output, cfg, hw, strategy_order(strategies))
    print(
        f"wrote {RESULTS / spec.id} — {summary['n_commits']} commits, "
        f"{len(output.skipped)} skipped"
    )
    return EXIT_OK


def cmd_session(args) -> int:
    hw = probe()
    corpus = _corpus()
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
            f"selected {p.selected:5d}/{p.total_tests:5d}  ratio {p.ratio():.3f}  {p.tier}"
        )
    return EXIT_OK


def cmd_report(args) -> int:
    summaries = []
    for d in sorted(RESULTS.iterdir()) if RESULTS.exists() else []:
        f = d / "summary.json"
        if f.exists():
            summaries.append(json.loads(f.read_text(encoding="utf-8")))
    if not summaries:
        print("no per-repo summaries found; run `replay` first", file=sys.stderr)
        return EXIT_GUARD
    (RESULTS / "aggregate.md").write_text(render_aggregate(summaries), encoding="utf-8")
    print(f"wrote {RESULTS / 'aggregate.md'} from {len(summaries)} repos")
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
    r.add_argument("--no-wallclock", action="store_true")
    r.add_argument("--seed", type=int, default=1)
    r.set_defaults(func=cmd_replay)

    s = sub.add_parser("session", help="replay one uncommitted drift session")
    s.add_argument("--repo", required=True)
    s.add_argument("--cycles", type=int, default=25)
    s.add_argument("--seed", type=int, default=1)
    s.set_defaults(func=cmd_session)

    rep = sub.add_parser("report", help="regenerate aggregate.md from committed summaries")
    rep.set_defaults(func=cmd_report)
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
