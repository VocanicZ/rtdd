"""Baseline 2 — the naive path heuristic.

`tests/test_<module>.py` for a changed `src/<module>.py`: what a developer writes
in ten minutes with no map at all. This is the baseline PRD #4's pre-registered
decision criterion is measured against, so the rule is stated exactly and is not
quietly improved:

1. A changed path that is itself a test file contributes every collected test id in it.
2. A changed path `<anything>/<stem>.py` that is not a test file contributes every
   collected test id whose file basename is `test_<stem>.py` or `<stem>_test.py`,
   anywhere in the tree.
3. Nothing else. No package matching, no directory fallback, no import analysis.
4. A changed path that resolves to no test file contributes nothing — the heuristic
   under-selects, and that is the property being measured.
"""

from __future__ import annotations

import pathlib
import time
from collections.abc import Sequence

from replay.gitwork import is_test_path
from replay.strategies.base import CommitContext, Selection, register


def _file_of(test_id: str) -> str:
    return test_id.split("::", 1)[0]


def candidate_test_files(module_stem: str, all_tests: Sequence[str]) -> set[str]:
    """Collected test files whose *basename* names `module_stem`. Basename only."""
    wanted = {f"test_{module_stem}.py", f"{module_stem}_test.py"}
    return {f for f in {_file_of(t) for t in all_tests} if pathlib.PurePosixPath(f).name in wanted}


def tests_in_files(files: set[str], all_tests: Sequence[str]) -> tuple[str, ...]:
    return tuple(sorted(t for t in all_tests if _file_of(t) in files))


class PathHeuristic:
    id = "path"
    needs_parent_state = False

    def prepare(self, ctx: CommitContext) -> None:
        return None

    def select(self, ctx: CommitContext) -> Selection:
        start = time.perf_counter()
        files: set[str] = set()
        known = {_file_of(t) for t in ctx.all_tests}
        for ch in ctx.changed:
            if not ch.path.endswith(".py"):
                continue
            if is_test_path(ch.path, ctx.test_globs):
                if ch.path in known:
                    files.add(ch.path)
                continue
            files |= candidate_test_files(pathlib.PurePosixPath(ch.path).stem, ctx.all_tests)
        tests = tests_in_files(files, ctx.all_tests)
        reason = "sibling test files" if tests else "no sibling test file"
        return Selection(
            tests=tests,
            escalated=False,
            reason=reason,
            select_ms=int(round((time.perf_counter() - start) * 1000)),
        )


register(PathHeuristic())
