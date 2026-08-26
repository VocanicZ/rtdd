"""The replay orchestrator: walk the points, build both variants, drive every strategy.

This is where the global constraints of `docs/plans/04-m3-replay-benchmark.md` stop
being prose and start being code:

* **The two populations never pool.** `natural` (a real commit replayed over its
  parent) and `probe` (the commit's tests kept, its source half reverted) are built
  from different base trees and every record carries its `variant`.
* **`F_full` is a delta, not a snapshot.** A repo whose suite is already red at the
  base tree would otherwise hand every strategy free recall, so the failures of the
  *clean base tree* are measured first and subtracted.
* **A broken historical commit is data, not an abort.** A commit that collects
  nothing is recorded in `skipped` and the replay walks on.
* **Everything expensive is cached, salted by the config digest.** A re-run under
  the same config is nearly free; a changed config is a miss, never a silent mix.
* **Wall-clock is measured three ways** — the full uninstrumented suite, the
  strategy's subset instrumented, and the same subset uninstrumented — so the
  instrumentation tax is a published number rather than an excuse.
* **Isolation violations are findings.** Running a subset can hide a failure the
  full suite catches (shared fixtures, module-level state). On the wall-clock
  sample the subset is really run and compared against `F_full ∩ selected`; a
  mismatch is recorded, never raised.
"""

from __future__ import annotations

import dataclasses
import pathlib
import shutil
import sys
from collections.abc import Sequence

from replay import rtddio
from replay.cache import Cache
from replay.config import RunConfig
from replay.corpus import Corpus, RepoSpec
from replay.covread import read_coverage
from replay.falsesignal import build_record as build_uncovered
from replay.gitwork import (
    Change,
    add_worktree,
    materialise_natural,
    materialise_probe,
    remove_worktree,
    replay_points,
    working_changed_paths,
)
from replay.hardware import Hardware
from replay.records import CommitRecord, StrategyRecord, UncoveredRecord, WallClockRecord
from replay.runner import RunResult, cached_run, collect, run_full, run_key, run_subset
from replay.strategies import base as sbase
from replay.strategies.randomratio import RandomRatio

# Imported for their registration side effect: the registry is populated by module
# import, and the orchestrator must never depend on who imported what first.
from replay.strategies import (  # noqa: F401
    full as _full,
    importgraph as _ig,
    lastfailed as _lf,
    pathheuristic as _ph,
    rtdd as _rtdd,
    testmon as _tm,
    xdist as _xd,
)

PARENT_STATE_DIRS: dict[str, tuple[str, ...]] = {
    "rtdd": (".rtdd",),
    "testmon": (".testmondata",),
    "lf": (".pytest_cache",),
}
"""What each `needs_parent_state` strategy leaves behind in the clean base tree.

Building this state costs a full suite run per strategy per base tree, so it is
snapshotted into the cache as a directory artifact keyed by
`(repo_id, base_sha, strategy)` and restored on the next pass.
"""

_NOISE_ROOTS = frozenset(
    name for names in PARENT_STATE_DIRS.values() for name in names
) | {"__pycache__"}
_NOISE_PREFIXES = (".coverage",)  # `.coverage`, plus `.coverage.<host>.<pid>` under xdist


def _is_noise(rel: str) -> bool:
    """True for a path that a strategy's own `prepare()` created.

    `.pytest_cache/` and friends land in the worktree as untracked files, so an
    unfiltered `git status` would hand the path heuristic and the import graph a
    "changed file" that no commit ever touched.
    """
    parts = rel.split("/")
    if _NOISE_ROOTS.intersection(parts):
        return True
    return parts[0].startswith(_NOISE_PREFIXES)


def _material_changes(work: pathlib.Path) -> tuple[Change, ...]:
    return tuple(c for c in working_changed_paths(work) if not _is_noise(c.path))


@dataclasses.dataclass
class ReplayOutput:
    commits: list[CommitRecord] = dataclasses.field(default_factory=list)
    strategies: list[StrategyRecord] = dataclasses.field(default_factory=list)
    wallclocks: list[WallClockRecord] = dataclasses.field(default_factory=list)
    uncovered: list[UncoveredRecord] = dataclasses.field(default_factory=list)
    skipped: list[dict] = dataclasses.field(default_factory=list)


@dataclasses.dataclass(frozen=True)
class ReplayOptions:
    variants: tuple[str, ...] = ("natural", "probe")
    strategy_ids: tuple[str, ...] = (
        "rtdd",
        "testmon",
        "path",
        "lf",
        "importgraph",
        "xdist",
        "random",
        "full",
    )
    wallclock_sample: int = 20
    wallclock_enabled: bool = True
    rtdd_binary: str = "rtdd"


def strategy_order(ids: Sequence[str]) -> list[str]:
    """`rtdd` first, `random` then `full` last.

    `random` is ratio-matched against RTDD's selection size, so RTDD has to have
    answered before it runs. `full` goes last because it is the most expensive and
    its answer never depends on a peer.
    """
    rest = [i for i in ids if i not in ("rtdd", "random", "full")]
    out = [i for i in ids if i == "rtdd"] + sorted(rest)
    out += [i for i in ids if i == "random"]
    out += [i for i in ids if i == "full"]
    return out


def isolation_violation(
    observed_failing: frozenset[str],
    selected: Sequence[str],
    f_full: Sequence[str],
    pre_existing: frozenset[str],
) -> bool:
    """Did really running `selected` reproduce the failures the full suite saw?

    Both sides are taken net of the base tree's pre-existing reds, exactly as
    `F_full` is: otherwise a repo that ships one broken test would report a
    violation on every cycle that happened to select it.
    """
    observed = set(observed_failing) - set(pre_existing)
    expected = (set(f_full) & set(selected)) - set(pre_existing)
    return observed != expected


def _snapshot_state(work: pathlib.Path, names: Sequence[str], dest: pathlib.Path) -> None:
    dest.mkdir(parents=True, exist_ok=True)
    for name in names:
        src = work / name
        if not src.exists():
            continue
        target = dest / name
        if src.is_dir():
            shutil.copytree(src, target, dirs_exist_ok=True)
        else:
            shutil.copy2(src, target)


def _prepare_state(
    strategy_id: str, ctx: sbase.CommitContext, cache: Cache, repo_id: str, base_sha: str
) -> None:
    """Put the strategy's parent-tree state into `ctx.work`, from cache if possible."""
    names = PARENT_STATE_DIRS.get(strategy_id, ())
    if not names:
        # A strategy that needs parent state but names no artifact directory has to
        # rebuild it every time; there is nothing to snapshot.
        sbase.get(strategy_id).prepare(ctx)
        return
    key = cache.key(repo_id, base_sha, "state", strategy_id)
    if cache.restore_artifact(key, ctx.work):
        return
    sbase.get(strategy_id).prepare(ctx)
    staging = ctx.work.parent / f".state-{strategy_id}-{base_sha[:8]}"
    shutil.rmtree(staging, ignore_errors=True)
    try:
        _snapshot_state(ctx.work, names, staging)
        cache.store_artifact(key, staging)
    finally:
        shutil.rmtree(staging, ignore_errors=True)


def _cached_full(
    cache: Cache,
    key: str,
    work: pathlib.Path,
    python: str,
    instrumented: bool,
    source_globs: Sequence[str],
) -> RunResult:
    return cached_run(
        cache,
        key,
        lambda: run_full(
            work, python=python, instrumented=instrumented, source_globs=source_globs
        ),
    )


def clean_tree_failures(
    work: pathlib.Path, python: str, cache: Cache, key: str
) -> tuple[frozenset[str], dict[str, int]]:
    """The base tree's own reds, before anything is materialised over it.

    Returns `(failing, durations_ms)`. The failures are what `F_full` subtracts;
    the durations are kept because a repo that is red at the base is also the one
    whose timings a reader will want to check by hand.
    """
    res = _cached_full(cache, key, work, python, False, ())
    return res.failing(), res.durations()


def _cached_uncovered(
    cache: Cache,
    key: str,
    work: pathlib.Path,
    binary: str,
) -> tuple[frozenset[tuple[str, int]], int]:
    def build() -> dict:
        out = rtddio.run(work, binary=binary, base="HEAD")
        return {
            "uncovered": sorted([f, line] for f, line in out.uncovered),
            "wall_ms": out.wall_ms,
        }

    payload = cache.json_or_build(key, build)
    return (
        frozenset((f, int(line)) for f, line in payload["uncovered"]),
        int(payload["wall_ms"]),
    )


def _cached_coverage_truth(
    cache: Cache,
    key: str,
    work: pathlib.Path,
    python: str,
    source_globs: Sequence[str],
) -> frozenset[tuple[str, int]]:
    """The instrumented full run's covered set, cached as data rather than as a file.

    Caching the *run* alone would not do: the second pass would skip it, `.coverage`
    would never be written into the worktree, and `read_coverage` would fail on a
    file that a cache hit had made unnecessary.
    """

    def build() -> dict:
        run_full(work, python=python, instrumented=True, source_globs=source_globs)
        truth = read_coverage(work / ".coverage", work)
        return {"covered": sorted([f, line] for f, line in truth.covered)}

    payload = cache.json_or_build(key, build)
    return frozenset((f, int(line)) for f, line in payload["covered"])


def _cached_collect(cache: Cache, key: str, work: pathlib.Path, python: str) -> tuple[str, ...]:
    payload = cache.json_or_build(key, lambda: {"tests": list(collect(work, python=python))})
    return tuple(payload["tests"])


def replay_repo(
    repo: pathlib.Path,
    spec: RepoSpec,
    cfg: RunConfig,
    cache: Cache,
    hw: Hardware,
    work_root: pathlib.Path,
    opts: ReplayOptions,
    corpus: Corpus | None = None,
) -> ReplayOutput:
    """Replay `spec.replay_commits` commits of `repo`, one record per cycle.

    Pass `corpus` to have the frozen list enforced here rather than trusted from
    the caller: a repo outside it raises `UnknownRepoError` before a single
    worktree is created.
    """
    if corpus is not None:
        frozen = corpus.require(spec.id)
        if frozen.pin != spec.pin:
            raise ValueError(
                f"{spec.id!r} is in the frozen corpus at {frozen.pin}, but this run was "
                f"handed {spec.pin}. Re-freeze the corpus rather than replaying an "
                f"unfrozen pin."
            )

    # `random` is seeded from the config and `rtdd` is bound to the binary under
    # test, so both are re-registered with this run's parameters.
    sbase.register(RandomRatio(seed=cfg.random_seed))
    sbase.register(_rtdd.Rtdd(binary=opts.rtdd_binary))

    out = ReplayOutput()
    python = sys.executable
    points = replay_points(repo, spec.pin, spec.replay_commits)
    order = strategy_order(opts.strategy_ids)
    wall_every = (
        max(1, len(points) // opts.wallclock_sample)
        if opts.wallclock_enabled and opts.wallclock_sample and hw.wallclock_allowed()
        else 0
    )

    for index, point in enumerate(points):
        for variant in opts.variants:
            work = work_root / f"{spec.id}-{variant}-{point.commit[:8]}"
            # `natural` replays the child over the parent; `probe` keeps the child's
            # tests and reverts its source half, so its clean base is the child.
            base_sha = point.parent if variant == "natural" else point.commit
            try:
                add_worktree(repo, base_sha, work)
                pre_existing, _ = clean_tree_failures(
                    work, python, cache, cache.key(spec.id, base_sha, "clean")
                )

                seed_ctx = sbase.CommitContext(
                    repo_id=spec.id,
                    variant=variant,
                    commit=point.commit,
                    parent=point.parent,
                    work=work,
                    changed=(),
                    all_tests=(),
                    source_globs=spec.source_globs,
                    test_globs=spec.test_globs,
                    python=python,
                )
                for sid in order:
                    if getattr(sbase.get(sid), "needs_parent_state", False):
                        _prepare_state(sid, seed_ctx, cache, spec.id, base_sha)

                if variant == "natural":
                    materialise_natural(work, repo, point)
                else:
                    materialise_probe(work, repo, point, spec.test_globs)

                all_tests = _cached_collect(
                    cache, cache.key(spec.id, point.commit, variant, "collect"), work, python
                )
                if not all_tests:
                    out.skipped.append(
                        {
                            "repo_id": spec.id,
                            "commit": point.commit,
                            "variant": variant,
                            "reason": "collect-error-or-empty",
                        }
                    )
                    continue

                gt = _cached_full(
                    cache,
                    run_key(cache, "full", spec.id, point.commit, variant, "groundtruth"),
                    work,
                    python,
                    False,
                    spec.source_globs,
                )
                f_full = tuple(sorted(gt.failing() - pre_existing))
                changes = _material_changes(work)

                out.commits.append(
                    CommitRecord(
                        repo_id=spec.id,
                        commit=point.commit,
                        parent=point.parent,
                        variant=variant,
                        all_tests=all_tests,
                        durations_ms=gt.durations(),
                        f_full=f_full,
                        pre_existing_failures=tuple(sorted(pre_existing)),
                        changed=tuple(sorted(c.path for c in changes)),
                    )
                )

                ctx = dataclasses.replace(seed_ctx, changed=changes, all_tests=all_tests)
                peer_sizes: dict[str, int] = {}
                selections: dict[str, sbase.Selection] = {}
                for sid in order:
                    ctx = dataclasses.replace(ctx, peer_sizes=dict(peer_sizes))
                    sel = sbase.validate_selection(sbase.get(sid).select(ctx), ctx)
                    selections[sid] = sel
                    peer_sizes[sid] = len(sel.tests)
                    out.strategies.append(
                        StrategyRecord(
                            repo_id=spec.id,
                            commit=point.commit,
                            variant=variant,
                            strategy=sid,
                            selected=sel.tests,
                            escalated=sel.escalated,
                            reason=sel.reason,
                            select_ms=sel.select_ms,
                        )
                    )

                rtdd_wall_ms: int | None = None
                if "rtdd" in order:
                    reported, rtdd_wall_ms = _cached_uncovered(
                        cache,
                        cache.key(spec.id, point.commit, variant, "rtdd-run"),
                        work,
                        opts.rtdd_binary,
                    )
                    covered = _cached_coverage_truth(
                        cache,
                        cache.key(spec.id, point.commit, variant, "covered"),
                        work,
                        python,
                        spec.source_globs,
                    )
                    out.uncovered.append(
                        build_uncovered(spec.id, point.commit, variant, reported, covered)
                    )

                if wall_every and index % wall_every == 0:
                    for sid in order:
                        tests = selections[sid].tests
                        sub = cached_run(
                            cache,
                            run_key(cache, "subset", spec.id, point.commit, variant, sid, tests),
                            lambda tests=tests: run_subset(
                                work, tests, python=python, source_globs=spec.source_globs
                            ),
                        )
                        if sid == "rtdd" and rtdd_wall_ms is not None:
                            inst_ms: int | None = rtdd_wall_ms
                        else:
                            inst_ms = cached_run(
                                cache,
                                run_key(
                                    cache,
                                    "subset-instrumented",
                                    spec.id,
                                    point.commit,
                                    variant,
                                    sid,
                                    tests,
                                ),
                                lambda tests=tests: run_subset(
                                    work,
                                    tests,
                                    python=python,
                                    instrumented=True,
                                    source_globs=spec.source_globs,
                                ),
                            ).wall_ms
                        out.wallclocks.append(
                            WallClockRecord(
                                repo_id=spec.id,
                                commit=point.commit,
                                variant=variant,
                                strategy=sid,
                                hardware_fingerprint=hw.fingerprint(),
                                full_uninstrumented_ms=gt.wall_ms,
                                subset_instrumented_ms=inst_ms,
                                subset_uninstrumented_ms=sub.wall_ms,
                                isolation_violation=isolation_violation(
                                    sub.failing(), tests, f_full, pre_existing
                                ),
                            )
                        )
            finally:
                remove_worktree(repo, work)
    return out
