"""Selection ratio as a function of cycles-since-commit — spec §5's drift axis.

Spec §5 states the degradation plainly: with ``--base HEAD`` the changed set
grows across a long uncommitted session, so selection ratio degrades toward 1.0
the longer an agent runs without committing. Spec §10 requires it measured as
its own axis rather than assumed, and this module is that measurement.

**Method.** Take consecutive commits ``C1..Ck`` from real history. The caller
builds a worktree at ``C1``'s parent and seeds RTDD there **once**; ``run_drift``
then applies ``C1``, ``C2``, … one after another *without committing anything*
and records the selection size after each application. Cycle ``i`` therefore
carries the accumulated changed set of ``i`` real commits, which is exactly the
uncommitted-session shape. HEAD never moves, so ``--base HEAD`` sees that whole
accumulation, and :func:`run_drift` refuses a worktree parked anywhere but
``points[0].parent`` rather than silently curve-fitting a different history.

**The changed set is measured, not accumulated bookkeeping.** It is
``git status`` against the unmoved HEAD, so a commit that restores a file to the
parent's exact bytes takes that file *out* of the set again. Growth is the
expected shape, not an invariant this module enforces: enforcing it would mean
publishing a number that no longer matches what ``rtdd`` was actually asked.

**The denominator moves with the tree.** Cycles that add test files enlarge the
suite, so the total is re-collected per cycle. A total frozen at the parent would
divide by a suite that no longer exists and overstate every later ratio.
"""

from __future__ import annotations

import dataclasses
import pathlib
from collections.abc import Callable, Sequence

from replay import rtddio
from replay.gitwork import ReplayPoint, git, materialise_natural, working_changed_paths
from replay.runner import collect
from replay.rtddio import WhichResult


@dataclasses.dataclass(frozen=True)
class DriftPoint:
    """One uncommitted cycle: what had accumulated, and what RTDD selected."""

    cycle: int
    changed_files: int
    selected: int
    total_tests: int
    tier: str
    #: pytest would not collect this tree. A long uncommitted session passes
    #: through intermediate states no commit ever had, and one of them will not
    #: collect; the denominator is then unknown, not one.
    collect_failed: bool = False

    def ratio(self) -> float | None:
        if self.collect_failed:
            return None
        return 0.0 if self.total_tests == 0 else self.selected / self.total_tests

    def to_dict(self) -> dict:
        return {
            "cycle": self.cycle,
            "changed_files": self.changed_files,
            "selected": self.selected,
            "total_tests": self.total_tests,
            "tier": self.tier,
            "collect_failed": self.collect_failed,
            "selection_ratio": self.ratio(),
        }


@dataclasses.dataclass(frozen=True)
class DriftCurve:
    """One session's curve, in the shape the report layer writes as ``drift.json``."""

    repo_id: str
    start_commit: str
    points: tuple[DriftPoint, ...]

    def to_dict(self) -> dict:
        return {
            "repo_id": self.repo_id,
            "start_commit": self.start_commit,
            "points": [p.to_dict() for p in self.points],
        }


def run_drift(
    repo: pathlib.Path,
    repo_id: str,
    work: pathlib.Path,
    points: Sequence[ReplayPoint],
    python: str,
    binary: str = "rtdd",
    select: Callable[[pathlib.Path], WhichResult] | None = None,
) -> DriftCurve:
    """Replay `points` into `work` as one uncommitted session and record the curve.

    `work` must already be checked out at ``points[0].parent`` with the strategy's
    state seeded there — seeding happens once, before cycle 1, and never again.
    Nothing is committed between cycles, so ``--base HEAD`` sees the accumulated
    set. `select` is injectable so the curve logic is testable without the binary;
    the CLI passes ``None`` and gets :func:`replay.rtddio.which` at ``--base HEAD``.
    """
    if not points:
        return DriftCurve(repo_id=repo_id, start_commit="", points=())

    head = git(work, "rev-parse", "HEAD")
    if head != points[0].parent:
        raise ValueError(
            f"{work} is at {head}, not at {points[0].parent}; seed the worktree at "
            "the first replay point's parent before running the drift session"
        )

    picker = select or (lambda w: rtddio.which(w, binary=binary, base="HEAD"))
    out: list[DriftPoint] = []
    for i, point in enumerate(points, start=1):
        materialise_natural(work, repo, point)
        changed = working_changed_paths(work)
        total = len(collect(work, python=python))
        result = picker(work)
        out.append(
            DriftPoint(
                cycle=i,
                changed_files=len(changed),
                selected=len(result.tests),
                # `rtdd` may name a test collection missed; the denominator can
                # never be smaller than the numerator, or the ratio exceeds 1.
                # A tree that collected *nothing* is a different case: the
                # denominator is unknown, and falling back to the selection size
                # would publish `1.000` — "RTDD ran everything" — for a cycle
                # where nothing was counted at all.
                total_tests=0 if total == 0 else max(total, len(result.tests)),
                tier=result.tier,
                collect_failed=total == 0,
            )
        )
    return DriftCurve(
        repo_id=repo_id, start_commit=points[0].parent, points=tuple(out)
    )
