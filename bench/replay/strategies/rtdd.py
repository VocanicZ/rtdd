"""The system under test: RTDD itself, behind the same protocol as every baseline.

Two invocations, both shipped defaults: `rtdd seed` in the clean parent tree
during :meth:`Rtdd.prepare`, and `rtdd which --base HEAD --json` for
:meth:`Rtdd.select`. There is deliberately no hook for extra argv — `rtddio` does
not accept any — because one tuning flag turns the published comparison into a
tuned RTDD against untuned baselines, which is not a result.

The tier is carried into `Selection.reason` so a reader of `strategies.jsonl` can
tell *why* a commit selected what it did, and a T2 escalation is reported as
`escalated = True` rather than as a strategy that happened to pick the whole
suite. That distinction is what lets escalation be published as its own number.

Failures propagate: `rtddio` raises rather than manufacturing an empty selection,
and this strategy does not catch it. An empty selection invented from a crashed
binary would score as a perfect recall miss attributed to RTDD.
"""

from __future__ import annotations

from replay import rtddio
from replay.strategies.base import CommitContext, Selection, register


class Rtdd:
    id = "rtdd"
    needs_parent_state = True

    def __init__(self, binary: str = "rtdd") -> None:
        self.binary = binary

    def prepare(self, ctx: CommitContext) -> None:
        """Build `.rtdd/map.jsonl` with one full instrumented run at the parent."""
        rtddio.seed(ctx.work, binary=self.binary)

    def select(self, ctx: CommitContext) -> Selection:
        w = rtddio.which(ctx.work, binary=self.binary, base="HEAD")
        reason = w.tier if not w.reason else f"{w.tier}: {w.reason}"
        return Selection(
            tests=w.tests,
            escalated=w.escalated(),
            reason=reason,
            select_ms=w.wall_ms,
        )


register(Rtdd())
