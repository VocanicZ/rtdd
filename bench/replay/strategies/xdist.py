"""Baseline 5 — `pytest -n auto`.

The intervention a real team actually reaches for when the suite gets slow. It
selects nothing: every test still runs, only the execution changes. It is in the
set to occupy the wall-clock column — if `-n auto` on the full suite beats RTDD's
instrumented subset, that is the finding, and the results table has to show it
rather than compare RTDD only against a serial full run.

Because it runs everything, every cycle is an escalation by construction; the
parallelism lives in `Selection.exec_args`, which the runner turns into `-n auto`
so the comparison is like-for-like on the same machine.
"""

from __future__ import annotations

from replay.strategies.base import CommitContext, Selection, register


class Xdist:
    id = "xdist"
    needs_parent_state = False

    def prepare(self, ctx: CommitContext) -> None:
        return None

    def select(self, ctx: CommitContext) -> Selection:
        return Selection(
            tests=tuple(ctx.all_tests),
            escalated=True,
            reason="no selection; parallel execution",
            exec_args=("-n", "auto"),
        )


register(Xdist())
