"""The four records that flow orchestrator → metrics → report.

These dataclasses are the diffable line format in `commits.jsonl`, so their field
names *are* the published schema: a rename here is a schema break, not a
refactor. Every record is frozen, every `to_dict` emits a `kind` discriminator,
and `to_jsonl_lines` is deterministic — the same records produce a byte-identical
file every run, so two benchmark runs diff to nothing when nothing changed.

`CommitRecord.f_full` is the *newly* failing set: pre-existing failures are held
separately so no metric can ever count a test that was already red as detected
work.
"""

from __future__ import annotations

import dataclasses
import json
from collections.abc import Iterable, Sequence
from typing import Any

STRATA: tuple[str, ...] = ("1", "2-5", "6-20", "21+")


def stratum_of(n: int) -> str:
    """Bucket a failure count. `0` is its own label — it is never a published stratum."""
    if n <= 0:
        return "0"
    if n == 1:
        return "1"
    if n <= 5:
        return "2-5"
    if n <= 20:
        return "6-20"
    return "21+"


@dataclasses.dataclass(frozen=True)
class CommitRecord:
    """One replayed commit: the ground truth every strategy is scored against."""

    repo_id: str
    commit: str
    parent: str
    variant: str
    all_tests: tuple[str, ...]
    durations_ms: dict[str, int]
    f_full: tuple[str, ...]
    pre_existing_failures: tuple[str, ...]
    changed: tuple[str, ...]

    def stratum(self) -> str:
        return stratum_of(len(self.f_full))

    def total_duration_ms(self) -> int:
        return sum(self.durations_ms.get(t, 0) for t in self.all_tests)

    def to_dict(self) -> dict:
        return {
            "kind": "commit",
            "repo_id": self.repo_id,
            "commit": self.commit,
            "parent": self.parent,
            "variant": self.variant,
            "n_tests": len(self.all_tests),
            "all_tests": list(self.all_tests),
            "durations_ms": dict(sorted(self.durations_ms.items())),
            "f_full": list(self.f_full),
            "pre_existing_failures": list(self.pre_existing_failures),
            "changed": list(self.changed),
            "stratum": self.stratum(),
        }

    @classmethod
    def from_dict(cls, d: dict) -> CommitRecord:
        return cls(
            repo_id=d["repo_id"],
            commit=d["commit"],
            parent=d["parent"],
            variant=d["variant"],
            all_tests=tuple(d["all_tests"]),
            durations_ms={k: int(v) for k, v in d["durations_ms"].items()},
            f_full=tuple(d["f_full"]),
            pre_existing_failures=tuple(d["pre_existing_failures"]),
            changed=tuple(d["changed"]),
        )


@dataclasses.dataclass(frozen=True)
class StrategyRecord:
    """What one strategy selected for one commit, and whether it gave up."""

    repo_id: str
    commit: str
    variant: str
    strategy: str
    selected: tuple[str, ...]
    escalated: bool
    reason: str
    select_ms: int

    def to_dict(self) -> dict:
        return {
            "kind": "strategy",
            "repo_id": self.repo_id,
            "commit": self.commit,
            "variant": self.variant,
            "strategy": self.strategy,
            "selected": list(self.selected),
            "n_selected": len(self.selected),
            "escalated": self.escalated,
            "reason": self.reason,
            "select_ms": self.select_ms,
        }

    @classmethod
    def from_dict(cls, d: dict) -> StrategyRecord:
        return cls(
            repo_id=d["repo_id"],
            commit=d["commit"],
            variant=d["variant"],
            strategy=d["strategy"],
            selected=tuple(d["selected"]),
            escalated=bool(d["escalated"]),
            reason=d["reason"],
            select_ms=int(d["select_ms"]),
        )


@dataclasses.dataclass(frozen=True)
class WallClockRecord:
    """The timing half, kept apart from selection so a slow machine cannot flatter recall."""

    repo_id: str
    commit: str
    variant: str
    strategy: str
    hardware_fingerprint: str
    full_uninstrumented_ms: int | None
    subset_instrumented_ms: int | None
    subset_uninstrumented_ms: int | None
    isolation_violation: bool

    def to_dict(self) -> dict:
        return {
            "kind": "wallclock",
            "repo_id": self.repo_id,
            "commit": self.commit,
            "variant": self.variant,
            "strategy": self.strategy,
            "hardware_fingerprint": self.hardware_fingerprint,
            "full_uninstrumented_ms": self.full_uninstrumented_ms,
            "subset_instrumented_ms": self.subset_instrumented_ms,
            "subset_uninstrumented_ms": self.subset_uninstrumented_ms,
            "isolation_violation": self.isolation_violation,
        }

    @classmethod
    def from_dict(cls, d: dict) -> WallClockRecord:
        return cls(
            repo_id=d["repo_id"],
            commit=d["commit"],
            variant=d["variant"],
            strategy=d["strategy"],
            hardware_fingerprint=d["hardware_fingerprint"],
            full_uninstrumented_ms=_opt_int(d["full_uninstrumented_ms"]),
            subset_instrumented_ms=_opt_int(d["subset_instrumented_ms"]),
            subset_uninstrumented_ms=_opt_int(d["subset_uninstrumented_ms"]),
            isolation_violation=bool(d["isolation_violation"]),
        )


@dataclasses.dataclass(frozen=True)
class UncoveredRecord:
    """What the tool reported as uncovered, against the ground-truth coverage."""

    repo_id: str
    commit: str
    variant: str
    reported: tuple[tuple[str, int], ...]
    truth_covered_hits: int
    reported_count: int

    def to_dict(self) -> dict:
        return {
            "kind": "uncovered",
            "repo_id": self.repo_id,
            "commit": self.commit,
            "variant": self.variant,
            "reported": [[p, n] for p, n in self.reported],
            "truth_covered_hits": self.truth_covered_hits,
            "reported_count": self.reported_count,
        }

    @classmethod
    def from_dict(cls, d: dict) -> UncoveredRecord:
        return cls(
            repo_id=d["repo_id"],
            commit=d["commit"],
            variant=d["variant"],
            reported=tuple((p, int(n)) for p, n in d["reported"]),
            truth_covered_hits=int(d["truth_covered_hits"]),
            reported_count=int(d["reported_count"]),
        )


RECORD_KINDS: dict[str, Any] = {
    "commit": CommitRecord,
    "strategy": StrategyRecord,
    "wallclock": WallClockRecord,
    "uncovered": UncoveredRecord,
}


def _opt_int(v: object) -> int | None:
    return None if v is None else int(v)


def from_dict(d: dict) -> Any:
    """Rebuild the record a `to_dict` produced, dispatching on its `kind`."""
    kind = d.get("kind")
    if kind not in RECORD_KINDS:
        raise ValueError(f"unknown record kind {kind!r}; known: {', '.join(sorted(RECORD_KINDS))}")
    return RECORD_KINDS[kind].from_dict(d)


def _sort_key(d: dict) -> tuple[str, str, str, str, str]:
    return (d["repo_id"], d["commit"], d["variant"], d["kind"], d.get("strategy", ""))


def to_jsonl_lines(records: Sequence[object]) -> list[str]:
    """Serialise records into sorted, newline-terminated, byte-stable JSONL lines."""
    dicts = [r.to_dict() for r in records]  # type: ignore[attr-defined]
    dicts.sort(key=_sort_key)
    return [json.dumps(d, sort_keys=True, separators=(",", ":")) + "\n" for d in dicts]


def parse_jsonl_lines(lines: Iterable[str]) -> list[Any]:
    """Read back what `to_jsonl_lines` wrote. Blank lines are skipped, not an error."""
    return [from_dict(json.loads(line)) for line in lines if line.strip()]
