"""Baseline 7 — the full suite: run everything, every time.

This is the ground-truth arm. It cannot miss a failure, so its recall is 1.00 by
construction and its cost is the whole suite. It is also, by definition, a 100%
escalation: `escalated=True` on every cycle, so the escalation column stays
honest rather than special-casing the control.
"""

from __future__ import annotations

import time

from replay.strategies.base import CommitContext, Selection, register


class FullSuite:
    id = "full"
    needs_parent_state = False

    def prepare(self, ctx: CommitContext) -> None:
        return None

    def select(self, ctx: CommitContext) -> Selection:
        start = time.perf_counter()
        return Selection(
            tests=tuple(ctx.all_tests),
            escalated=True,
            reason="full suite",
            select_ms=int(round((time.perf_counter() - start) * 1000)),
        )


register(FullSuite())
