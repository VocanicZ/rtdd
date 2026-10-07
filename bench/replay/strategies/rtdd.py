"""The system under test: RTDD itself, behind the same protocol as every baseline.

Two invocations, both shipped defaults: `rtdd graph` in the clean parent tree
during :meth:`Rtdd.prepare` — the instance's one graph build — and
`rtdd which --base HEAD --json` for :meth:`Rtdd.select`. There is no `rtdd seed`
(v0.3.0 has none) and deliberately no hook for extra argv — `rtddio` does not
accept any — because one tuning flag turns the published comparison into a tuned
RTDD against untuned baselines, which is not a result.

This arm is **Round 1**: the tests linked to a changed node. v0.3.0 has no tiers
and never escalates — Round 3 is the full suite, measured as the `full` strategy —
so `escalated` is always `False`. Round 1's schema-3 ids are expanded to collected
pytest ids by :func:`replay.graphids.expand_graph_ids`; a graph id pytest never
collected (a helper, a fixture) is dropped and counted in `Selection.reason`.

Failures propagate: `rtddio` raises rather than manufacturing an empty selection,
and this strategy does not catch it. An empty selection invented from a crashed
binary would score as a perfect recall miss attributed to RTDD.
"""

from __future__ import annotations

from replay import rtddio
from replay.graphids import expand_graph_ids
from replay.strategies.base import CommitContext, Selection, register


class Rtdd:
    id = "rtdd"
    needs_parent_state = True
    ROUNDS: tuple[int, ...] = (1,)

    def __init__(self, binary: str = "rtdd") -> None:
        self.binary = binary

    def prepare(self, ctx: CommitContext) -> None:
        """Build `.rtdd/graph.json` at the parent: one `rtdd graph` per instance."""
        rtddio.graph(ctx.work, binary=self.binary)

    def _graph_ids(self, w: rtddio.WhichResult) -> tuple[str, ...]:
        """The schema-3 test ids this arm selects."""
        return w.round1

    def select(self, ctx: CommitContext) -> Selection:
        w = rtddio.which(ctx.work, binary=self.binary, base="HEAD")
        ids = self._graph_ids(w)
        tests, unmatched = expand_graph_ids(ids, ctx.all_tests)
        label = "rounds" if len(self.ROUNDS) > 1 else "round"
        rounds = "+".join(str(n) for n in self.ROUNDS)
        return Selection(
            tests=tests,
            escalated=False,
            reason=(
                f"{label} {rounds}: {len(ids)} graph tests, {len(unmatched)} not collected"
            ),
            select_ms=w.wall_ms,
        )


class RtddRounds12(Rtdd):
    """The **Rounds 1+2** arm: Round 1 plus Round 2's tests of the changed nodes' neighbours.

    It builds no graph. The instance's one `rtdd graph` is the `rtdd` arm's
    :meth:`Rtdd.prepare`, at the parent tree; this arm answers from that same
    `.rtdd/graph.json` — the way `random` answers from `rtdd`'s selection size — so
    it declares `rtdd` its peer, runs straight after it, and a second build (which
    would double the measured cost and be a different experiment) never happens.

    Schema 3's Round 2 already excludes Round 1; the union is de-duplicated anyway
    so no test is charged twice to time in tests.
    """

    id = "rtdd-r12"
    needs_parent_state = False
    peer = "rtdd"
    ROUNDS: tuple[int, ...] = (1, 2)

    def prepare(self, ctx: CommitContext) -> None:
        """Nothing: the graph is the `rtdd` arm's."""
        return None

    def _graph_ids(self, w: rtddio.WhichResult) -> tuple[str, ...]:
        return w.round1 + w.round2


register(Rtdd())
register(RtddRounds12())
