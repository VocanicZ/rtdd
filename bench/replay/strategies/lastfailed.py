"""Baseline 3 — `pytest --lf`.

`--lf` looks free: it runs only what failed last time and costs nothing to adopt.
The reason it belongs in the baseline set is the other half of its behaviour — an
empty `lastfailed` cache runs the *whole* suite. That is real `--lf`, so it is
scored as an escalation rather than quietly dropped, and how often it degenerates
becomes a published number instead of an unstated assumption.

The cache is state carried from the parent commit, so this strategy declares
`needs_parent_state`: `prepare` runs the full suite in the clean parent tree with
pytest's cache provider left on (`runner.run_full(..., cacheprovider=True)`),
which is the one place in the harness where that flag is set.
"""

from __future__ import annotations

import json
import pathlib
import sys
import time

from replay.runner import run_full
from replay.strategies.base import CommitContext, Selection, register


def read_lastfailed(work: pathlib.Path) -> tuple[str, ...]:
    """Return the test ids pytest recorded as failing, or `()` if it recorded none.

    Only `::`-bearing keys are node ids; pytest also stores whole-file keys, and
    counting those as selections would credit `--lf` with tests it never named.
    An unreadable cache is an empty cache — the degenerate full-suite path below
    is the honest answer, not a crash.
    """
    p = work / ".pytest_cache" / "v" / "cache" / "lastfailed"
    if not p.exists():
        return ()
    try:
        data = json.loads(p.read_text(encoding="utf-8"))
    except json.JSONDecodeError:
        return ()
    if not isinstance(data, dict):
        return ()
    return tuple(sorted(k for k, v in data.items() if v and "::" in k))


class LastFailed:
    id = "lf"
    needs_parent_state = True

    def prepare(self, ctx: CommitContext) -> None:
        """Populate `.pytest_cache` by running the full suite in the clean parent tree.

        The harness calls this before materialising the child commit; the
        orchestrator caches the resulting `.pytest_cache` directory by
        (repo, parent).
        """
        run_full(ctx.work, python=ctx.python or sys.executable, cacheprovider=True)

    def select(self, ctx: CommitContext) -> Selection:
        start = time.perf_counter()
        failed = read_lastfailed(ctx.work)
        known = set(ctx.all_tests)
        selected = tuple(t for t in failed if t in known)
        elapsed = int(round((time.perf_counter() - start) * 1000))
        if not selected:
            return Selection(
                tests=tuple(ctx.all_tests),
                escalated=True,
                reason="--lf with no recorded failures runs the full suite",
                select_ms=elapsed,
            )
        return Selection(tests=selected, escalated=False, reason="--lf", select_ms=elapsed)


register(LastFailed())
