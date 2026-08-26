"""Baseline 6 — uniform random selection at the peer's ratio.

The control that shows selection beats chance. It samples exactly as many tests
as its peer (RTDD by default) selected at the same commit, so any recall gap
between the two is attributable to *which* tests were chosen and not to how many.

Reproducibility comes from the published config alone: the generator is seeded by
`(random_seed, repo_id, variant, commit)` via SHA-256, so re-running the
benchmark from `config.json` reproduces the sample exactly, and the two variants
of one commit are sampled independently.

A missing peer entry is a harness ordering bug — the orchestrator ran `random`
before its peer — not a degenerate zero-sample, so it raises.
"""

from __future__ import annotations

import hashlib
import random
import time

from replay.strategies.base import CommitContext, Selection, register


def seeded_rng(seed: int, *parts: str) -> random.Random:
    """A generator determined entirely by `seed` and `parts`.

    `Random(seed)` alone would give every commit the same stream; hashing the
    coordinates in makes each (repo, variant, commit) its own independent draw
    while keeping the whole benchmark reproducible from one published integer.
    """
    raw = "\0".join((str(seed), *parts)).encode("utf-8")
    return random.Random(int.from_bytes(hashlib.sha256(raw).digest()[:8], "big"))


class RandomRatio:
    id = "random"
    needs_parent_state = False

    def __init__(self, seed: int = 0, peer: str = "rtdd") -> None:
        self.seed = seed
        self.peer = peer

    def prepare(self, ctx: CommitContext) -> None:
        return None

    def select(self, ctx: CommitContext) -> Selection:
        start = time.perf_counter()
        if self.peer not in ctx.peer_sizes:
            raise RuntimeError(
                f"peer_sizes has no entry for {self.peer!r}; the orchestrator must run "
                f"{self.peer!r} before 'random' so the control matches its selection size"
            )
        k = min(ctx.peer_sizes[self.peer], len(ctx.all_tests))
        rng = seeded_rng(self.seed, ctx.repo_id, ctx.variant, ctx.commit)
        picked = tuple(sorted(rng.sample(list(ctx.all_tests), k))) if k else ()
        return Selection(
            tests=picked,
            escalated=False,
            reason=f"uniform sample at {self.peer}'s selection size",
            select_ms=int(round((time.perf_counter() - start) * 1000)),
        )


register(RandomRatio())
