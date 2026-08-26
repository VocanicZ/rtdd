"""The one protocol every selection strategy sits behind.

A strategy sees a `CommitContext` — everything the harness materialised for one
replayed commit — and answers with a `Selection`. Every `Selection` carries
`escalated`, so a strategy that gives up and runs the whole suite still produces a
record and escalation can be published as its own number instead of hiding inside
a recall figure.
"""

from __future__ import annotations

import dataclasses
import pathlib
from typing import Protocol, runtime_checkable

from replay.gitwork import Change


class UnknownTestError(ValueError):
    """A strategy selected a test id the repo never collected.

    Scoring a test id that is not in `ctx.all_tests` would silently inflate recall:
    the id can never match ground truth, but a careless denominator would still
    count it as work done. It is an error at selection time, not a metrics problem.
    """


@dataclasses.dataclass(frozen=True)
class Selection:
    tests: tuple[str, ...]
    escalated: bool
    reason: str
    select_ms: int = 0
    exec_args: tuple[str, ...] = ()


@dataclasses.dataclass(frozen=True)
class CommitContext:
    repo_id: str
    variant: str
    commit: str
    parent: str
    work: pathlib.Path
    changed: tuple[Change, ...]
    all_tests: tuple[str, ...]
    source_globs: tuple[str, ...]
    test_globs: tuple[str, ...]
    python: str
    seed_ms: int = 0
    peer_sizes: dict[str, int] = dataclasses.field(default_factory=dict)

    def changed_paths(self) -> tuple[str, ...]:
        return tuple(sorted(c.path for c in self.changed))


@runtime_checkable
class Strategy(Protocol):
    id: str
    needs_parent_state: bool

    def prepare(self, ctx: CommitContext) -> None: ...
    def select(self, ctx: CommitContext) -> Selection: ...


REGISTRY: dict[str, Strategy] = {}


def register(s: Strategy) -> Strategy:
    REGISTRY[s.id] = s
    return s


def get(strategy_id: str) -> Strategy:
    if strategy_id not in REGISTRY:
        raise KeyError(f"unknown strategy {strategy_id!r}; known: {', '.join(sorted(REGISTRY))}")
    return REGISTRY[strategy_id]


def all_ids() -> tuple[str, ...]:
    return tuple(sorted(REGISTRY))


def validate_selection(selection: Selection, ctx: CommitContext) -> Selection:
    """Return `selection` unchanged, or refuse a test id the repo never collected."""
    known = set(ctx.all_tests)
    unknown = tuple(dict.fromkeys(t for t in selection.tests if t not in known))
    if unknown:
        raise UnknownTestError(
            f"{ctx.repo_id}@{ctx.commit}: selection contains "
            f"{len(unknown)} test id(s) not in the collected test list: "
            f"{', '.join(repr(t) for t in unknown)}"
        )
    return selection
