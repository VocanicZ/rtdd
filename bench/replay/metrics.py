"""The vocabulary every published number is expressed in.

A benchmark number is meaningless without its denominator, so no metric here ever
returns a bare float: :class:`Ratio` carries `num` and `den` and answers `None`
rather than `0.0` when `den == 0`, because "no commit qualified" and "every
qualifying commit scored zero" are different findings and a float cannot tell
them apart.

:func:`assert_single_repo` enforces the plan's never-pool rule at the only place
it can be enforced — the function that turns records into a number. A pooled
recall figure is dominated by whichever repo has the most commits, which is a
property of the corpus rather than of the tool; cross-repo figures exist only as
explicitly duration-weighted aggregates, printed in their own labelled table.

Recall is deliberately two numbers. Change-level ("did the selection catch this
regression at all?") is the one a developer feels, and it is computed over the
*detecting* population only — a commit whose full run is green has no ground truth
to be recalled against, so counting it would let a corpus of quiet commits inflate
any strategy towards 1.0. Test-level (`|F_sel ∩ F_full| / |F_full|`) is the one that
exposes a strategy which reliably finds one failure of twenty. Audit A8 killed
publishing only the flattering one, so both micro and macro are emitted.

Cost is likewise two numbers, and the second is the honest one: `selection_ratio`
counts tests, `selected_duration_fraction` counts milliseconds. A suite whose slowest
test is most of its wall clock can be cut to a tenth of its cases and save nothing,
and only the duration figure says so.

The false-signal metrics live in ``replay.falsesignal``.
"""

from __future__ import annotations

import dataclasses
from collections.abc import Sequence

from replay.records import STRATA, CommitRecord, StrategyRecord

Pairs = list[tuple[CommitRecord, StrategyRecord]]


class PoolingError(RuntimeError):
    """Raised when records from more than one `repo_id` reach one aggregate."""


@dataclasses.dataclass(frozen=True)
class Ratio:
    num: float
    den: float

    def value(self) -> float | None:
        return None if self.den == 0 else self.num / self.den

    def to_dict(self) -> dict:
        return {"num": self.num, "den": self.den, "value": self.value()}


def assert_single_repo(records: Sequence) -> str:
    """Return the one `repo_id` these records share, or refuse. Empty is `""`."""
    ids = {r.repo_id for r in records}
    if len(ids) > 1:
        raise PoolingError(
            f"refusing to pool across repos: {sorted(ids)}. Spec §10 forbids pooled "
            f"recall; use the duration-weighted aggregate instead."
        )
    return next(iter(ids)) if ids else ""


def detected(rec: CommitRecord, sr: StrategyRecord) -> bool:
    """Did the selection contain at least one newly-failing test?

    `f_full` already excludes the tests that were red at the clean base tree, so an
    unrelated flaky failure sitting inside the selection can never satisfy this.
    """
    return bool(set(sr.selected) & set(rec.f_full))


def pair_up(
    commits: Sequence[CommitRecord],
    strategy_records: Sequence[StrategyRecord],
    strategy: str,
) -> list[tuple[CommitRecord, StrategyRecord]]:
    """Join commits to one strategy's selections on `(commit, variant)`.

    A commit the strategy never ran is dropped rather than scored as a miss: an
    absent cycle is missing data, and charging it to recall would punish a crashed
    run exactly as hard as a wrong answer.
    """
    assert_single_repo([*commits, *strategy_records])
    index = {(s.commit, s.variant): s for s in strategy_records if s.strategy == strategy}
    pairs = []
    for c in commits:
        s = index.get((c.commit, c.variant))
        if s is not None:
            pairs.append((c, s))
    return pairs


def _detecting(pairs: Pairs) -> Pairs:
    """The commits with ground truth to recall. The green ones score cost, not recall."""
    return [(c, s) for c, s in pairs if c.f_full]


def change_level_recall(pairs: Pairs) -> Ratio:
    """Commits where the selection caught something, over commits that had something."""
    det = _detecting(pairs)
    return Ratio(num=sum(1 for c, s in det if detected(c, s)), den=len(det))


def test_level_recall_micro(pairs: Pairs) -> Ratio:
    """`Σ|selected ∩ f_full| / Σ|f_full|` — weighted towards the wide-blast commits."""
    det = _detecting(pairs)
    return Ratio(
        num=sum(len(set(s.selected) & set(c.f_full)) for c, s in det),
        den=sum(len(c.f_full) for c, s in det),
    )


def test_level_recall_macro(pairs: Pairs) -> Ratio:
    """Mean of the per-commit ratio — every commit weighs the same, wide or narrow."""
    det = _detecting(pairs)
    total = sum(len(set(s.selected) & set(c.f_full)) / len(c.f_full) for c, s in det)
    return Ratio(num=total, den=len(det))


def by_stratum(pairs: Pairs) -> dict[str, Pairs]:
    """Split the detecting pairs by `|f_full|`, in `STRATA` order so `1` comes first.

    The `|f_full| == 1` stratum is the hard one — there is exactly one test to find
    and no partial credit — and it is never merged into a pooled figure.
    """
    out: dict[str, Pairs] = {}
    for c, s in _detecting(pairs):
        out.setdefault(c.stratum(), []).append((c, s))
    return {k: out[k] for k in STRATA if k in out}


def selection_ratio(pairs: Pairs) -> Ratio:
    """Selected tests over all tests, across every cycle including the green ones."""
    num = sum(len(s.selected) for c, s in pairs)
    den = sum(len(c.all_tests) for c, s in pairs)
    return Ratio(num=num, den=den)


def selected_duration_fraction(pairs: Pairs) -> Ratio:
    """Selected milliseconds over all milliseconds — the cost figure that can be felt."""
    num = sum(sum(c.durations_ms.get(t, 0) for t in s.selected) for c, s in pairs)
    den = sum(c.total_duration_ms() for c, s in pairs)
    return Ratio(num=num, den=den)


def escalation_rate(strategy_records: Sequence[StrategyRecord]) -> Ratio:
    """Escalated cycles over *every* cycle. A strategy that gives up often is not free."""
    assert_single_repo(strategy_records)
    return Ratio(num=sum(1 for s in strategy_records if s.escalated), den=len(strategy_records))


def summarise(
    commits: Sequence[CommitRecord],
    strategy_records: Sequence[StrategyRecord],
    strategy: str,
) -> dict:
    """One strategy's whole scorecard for one repo, ready to be written to the report."""
    pairs = pair_up(commits, strategy_records, strategy)
    mine = [s for s in strategy_records if s.strategy == strategy]
    detecting = len(_detecting(pairs))
    return {
        "strategy": strategy,
        "repo_id": assert_single_repo([*commits, *strategy_records]),
        "cycles": len(pairs),
        "detecting_commits": detecting,
        "green_commits": len(pairs) - detecting,
        "change_level_recall": change_level_recall(pairs).to_dict(),
        "test_level_recall_micro": test_level_recall_micro(pairs).to_dict(),
        "test_level_recall_macro": test_level_recall_macro(pairs).to_dict(),
        "selection_ratio": selection_ratio(pairs).to_dict(),
        "selected_duration_fraction": selected_duration_fraction(pairs).to_dict(),
        "escalation_rate": escalation_rate(mine).to_dict(),
        "strata": {
            k: {
                "n": len(v),
                "change_level_recall": change_level_recall(v).to_dict(),
                "test_level_recall_micro": test_level_recall_micro(v).to_dict(),
            }
            for k, v in by_stratum(pairs).items()
        },
    }
