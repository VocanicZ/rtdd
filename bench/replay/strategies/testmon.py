"""Baseline 1 — pytest-testmon.

Method-level AST checksums, shipping since ~2016 (audit A9): finer-grained than
RTDD v1's file-level map, and therefore the baseline RTDD is most likely to lose
to on precision. It is the first one a reviewer asks about, so it is the first
one the harness runs.

State lives in `.testmondata`, built at the parent, so this declares
`needs_parent_state`. `select` never executes a test: testmon deselects during
collection, so `pytest --testmon --collect-only -q` *is* the selection. When the
database is missing or testmon decides it cannot be trusted — a changed
environment invalidates it wholesale — testmon runs everything, and that is
recorded as an escalation rather than as a strategy that happened to pick the
full suite.
"""

from __future__ import annotations

import os
import pathlib
import subprocess
import sys
import time

from replay.strategies.base import CommitContext, Selection, register

_INVALIDATED_MARKERS = ("cannot be read", "new environment")


def _env() -> dict[str, str]:
    env = dict(os.environ)
    env["PYTHONDONTWRITEBYTECODE"] = "1"
    env["TESTMON_DATAFILE"] = ".testmondata"
    return env


def testmon_collect(work: pathlib.Path, python: str) -> tuple[tuple[str, ...], str]:
    """Return the ids testmon left selected, plus its raw output for diagnosis.

    The output is handed back rather than discarded because the difference
    between "testmon selected everything" and "testmon threw its database away"
    is only visible in what it printed.
    """
    proc = subprocess.run(
        [python, "-m", "pytest", "--testmon", "--collect-only", "-q", "--no-header"],
        cwd=work,
        capture_output=True,
        text=True,
        env=_env(),
    )
    ids: list[str] = []
    for line in proc.stdout.splitlines():
        line = line.strip()
        if "::" in line and not line.startswith(("=", "-", "<")):
            ids.append(line)
    return tuple(sorted(set(ids))), proc.stdout + proc.stderr


class Testmon:
    id = "testmon"
    needs_parent_state = True

    def prepare(self, ctx: CommitContext) -> None:
        """Build `.testmondata` by running the suite under testmon at the parent."""
        subprocess.run(
            [ctx.python or sys.executable, "-m", "pytest", "--testmon", "-q", "--no-header"],
            cwd=ctx.work,
            capture_output=True,
            text=True,
            env=_env(),
        )

    def select(self, ctx: CommitContext) -> Selection:
        start = time.perf_counter()
        if not (ctx.work / ".testmondata").exists():
            return Selection(
                tests=tuple(ctx.all_tests),
                escalated=True,
                reason="no .testmondata; testmon runs everything",
                select_ms=int(round((time.perf_counter() - start) * 1000)),
            )
        ids, log = testmon_collect(ctx.work, ctx.python or sys.executable)
        elapsed = int(round((time.perf_counter() - start) * 1000))
        lowered = log.lower()
        if any(marker in lowered for marker in _INVALIDATED_MARKERS):
            return Selection(
                tests=tuple(ctx.all_tests),
                escalated=True,
                reason="testmon invalidated its database (environment change)",
                select_ms=elapsed,
            )
        known = set(ctx.all_tests)
        selected = tuple(t for t in ids if t in known)
        return Selection(
            tests=selected,
            escalated=len(selected) == len(known) and len(known) > 0,
            reason="testmon method-level checksums",
            select_ms=elapsed,
        )


register(Testmon())
