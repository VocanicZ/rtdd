"""How often the uncovered report fires on a change that is in fact tested.

Spec §10 asks for this number mechanically, and given §6's import-time class it is
the number that decides whether anyone leaves the feature switched on: a report
that cries wolf gets muted, after which its true positives are worth nothing.

The definitions are fixed and leave no room for judgement:

* **reported** — the ``(path, line)`` pairs ``rtdd run --json`` classes
  **Uncovered**. Never the ``import_time`` ones: RTDD reports those separately by
  design, so counting them here would score a distinction the tool draws
  deliberately as a mistake.
* **truth** — :attr:`replay.covread.CoverageTruth.covered`, the pairs executed
  under at least one **non-empty** test context in the full instrumented run *at
  the same tree*. Nothing is inferred from RTDD's own map: a metric that asked
  the tool whether the tool was right would measure self-consistency.
* A **line-level false signal** is a reported pair that is in truth.
* A **change-level false signal** is a commit where the report fired and *every*
  reported pair is in truth — it fired and was wholly wrong. A report that names
  four lines of which two really are untested has done its job.

Denominators are commits where the report fired; commits with an empty report are
`silent` and are counted by :func:`fire_rate` instead, so a tool that never speaks
cannot score a perfect false-signal rate.
"""

from __future__ import annotations

from collections.abc import Iterable, Sequence

from replay.metrics import Ratio, assert_single_repo
from replay.records import UncoveredRecord


def build_record(
    repo_id: str,
    commit: str,
    variant: str,
    reported: Iterable[tuple[str, int]],
    truth: Iterable[tuple[str, int]],
) -> UncoveredRecord:
    """Score one commit's report against the instrumented run's coverage.

    `reported` is sorted so the record's JSONL line is byte-stable across runs.
    """
    rep = tuple(sorted(reported))
    truth_set = set(truth)
    return UncoveredRecord(
        repo_id=repo_id,
        commit=commit,
        variant=variant,
        reported=rep,
        truth_covered_hits=sum(1 for p in rep if p in truth_set),
        reported_count=len(rep),
    )


def _fired(records: Sequence[UncoveredRecord]) -> list[UncoveredRecord]:
    assert_single_repo(records)
    return [r for r in records if r.reported_count > 0]


def change_false_signal_rate(records: Sequence[UncoveredRecord]) -> Ratio:
    """Fired-and-wholly-wrong commits over fired commits."""
    fired = _fired(records)
    wholly_wrong = sum(1 for r in fired if r.truth_covered_hits == r.reported_count)
    return Ratio(num=wholly_wrong, den=len(fired))


def line_false_signal_rate(records: Sequence[UncoveredRecord]) -> Ratio:
    """Reported-but-tested lines over all reported lines, across fired commits."""
    fired = _fired(records)
    return Ratio(
        num=sum(r.truth_covered_hits for r in fired),
        den=sum(r.reported_count for r in fired),
    )


def fire_rate(records: Sequence[UncoveredRecord], total_cycles: int) -> Ratio:
    """How often the report speaks at all — the context both rates need."""
    return Ratio(num=len(_fired(records)), den=total_cycles)


def summarise(records: Sequence[UncoveredRecord], total_cycles: int) -> dict:
    fired = _fired(records)
    return {
        "repo_id": assert_single_repo(records),
        "cycles": total_cycles,
        "fired": len(fired),
        "silent": len(records) - len(fired),
        "fire_rate": fire_rate(records, total_cycles).to_dict(),
        "change_false_signal_rate": change_false_signal_rate(records).to_dict(),
        "line_false_signal_rate": line_false_signal_rate(records).to_dict(),
    }
