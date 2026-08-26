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
from collections.abc import Sequence
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
    #: Ids the base tree collected and this commit no longer does — dropped from
    #: `tests` and published, so a rename never reads as selected work.
    stale_dropped: tuple[str, ...] = ()


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
    #: What the *base* tree collected. `--lf`, testmon and rtdd all answer out of
    #: state built there, so an id that vanished in this commit is stale, not
    #: invented, and the two are treated differently.
    base_tests: tuple[str, ...] = ()
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


def expand_file_selectors(
    tests: Sequence[str], all_tests: Sequence[str]
) -> tuple[str, ...]:
    """Replace a whole-file selector with the node ids that file collected.

    `pytest tests/test_basic.py` runs every test in the file, and `rtdd which`
    names exactly that selector for a changed test file, whose new tests its map
    has not seen yet. Scoring the bare path as one opaque id would both fail the
    known-id check and understate the work the selection causes, so it is expanded
    the same way pytest would expand it. A named file that collected nothing
    contributes nothing: the collected list is ground truth, and a test pytest
    never collected cannot fail.
    """
    by_file: dict[str, list[str]] = {}
    for t in all_tests:
        by_file.setdefault(t.split("::", 1)[0], []).append(t)
    out: list[str] = []
    for sel in tests:
        if "::" in sel:
            out.append(sel)
            continue
        out.extend(by_file.get(sel, ()))
    return tuple(dict.fromkeys(out))


def validate_selection(selection: Selection, ctx: CommitContext) -> Selection:
    """Expand file selectors, drop stale ids, and refuse an invented one.

    Three classes of id come back from a strategy. One the commit collected is
    kept. One the *base* tree collected and this commit does not is **stale** —
    every parent-state strategy answers out of state built at the base, so the
    first rename in real history produces them; it is dropped and published,
    because a test that no longer exists is neither work nor a possible failure.
    One that neither tree ever collected is invented, and that is still an error:
    it would put work in the numerator that no denominator can account for.
    """
    known = set(ctx.all_tests)
    expanded = expand_file_selectors(selection.tests, ctx.all_tests)
    stale = tuple(
        dict.fromkeys(t for t in expanded if t not in known and t in set(ctx.base_tests))
    )
    if stale:
        expanded = tuple(t for t in expanded if t not in set(stale))
    if expanded != selection.tests or stale:
        selection = dataclasses.replace(
            selection, tests=expanded, stale_dropped=selection.stale_dropped + stale
        )
    unknown = tuple(dict.fromkeys(t for t in selection.tests if t not in known))
    if unknown:
        raise UnknownTestError(
            f"{ctx.repo_id}@{ctx.commit}: selection contains "
            f"{len(unknown)} test id(s) not in the collected test list: "
            f"{', '.join(repr(t) for t in unknown)}"
        )
    return selection
