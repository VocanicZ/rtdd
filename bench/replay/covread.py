"""Read a corpus repo's ``.coverage`` store into the false-signal ground truth.

The store *is* the bipartite test↔line relation (spec §4), and the whole point of
reading it here is that it distinguishes two things a coverage percentage cannot:

* **per-test lines** — executed while a dynamic context was set, i.e. while some
  named test was running;
* **import-time lines** — executed during collection, before any context was set,
  and therefore attributed to no test at all.

:attr:`CoverageTruth.covered` is the union over the per-test sets **only**. A line
that ran solely at import was executed but asserted on by nobody; RTDD reports it
as ``import-time`` rather than ``uncovered``, and counting it as "in fact
adequately tested" would manufacture exactly the false signal the metric exists to
measure.

The reader mirrors ``internal/coverage`` on the Go side deliberately, including
the two shapes that silently produce an empty map if they are not handled: a
branch-mode store (``[run] branch = True``) writes **zero** ``line_bits`` rows and
puts every executed line in ``arc``, and non-positive line numbers in either table
are scope sentinels rather than source lines.
"""

from __future__ import annotations

import dataclasses
import pathlib
import sqlite3

_PHASES = ("run", "setup", "teardown")

_LINE_BITS_QUERY = (
    "SELECT DISTINCT f.path, c.context, lb.numbits FROM line_bits lb "
    "JOIN file f ON f.id = lb.file_id JOIN context c ON c.id = lb.context_id"
)

_ARC_QUERY = (
    "SELECT DISTINCT f.path, c.context, a.fromno, a.tono FROM arc a "
    "JOIN file f ON f.id = a.file_id JOIN context c ON c.id = a.context_id"
)


@dataclasses.dataclass(frozen=True)
class CoverageTruth:
    by_test: dict[str, frozenset[tuple[str, int]]]
    import_time: frozenset[tuple[str, int]]
    covered: frozenset[tuple[str, int]]


def numbits(blob: bytes) -> list[int]:
    """Decode coverage.py's packed bitmap: bit *n* set means line *n* executed."""
    lines: list[int] = []
    for i, byte in enumerate(blob):
        if not byte:
            continue
        for j in range(8):
            if byte & (1 << j):
                lines.append(i * 8 + j)
    return lines


def normalize_context(ctx: str) -> tuple[str, bool]:
    """Split ``"tests/test_a.py::test_x|run"`` into ``("tests/test_a.py::test_x", True)``.

    An empty context is import-time coverage and returns ``is_test=False``. The
    split is on the LAST ``|`` and only when the suffix is a known phase: a
    parametrised id can itself contain a pipe, and a static context set by the
    host repo has no phase suffix at all.
    """
    if not ctx:
        return ("", False)
    head, sep, tail = ctx.rpartition("|")
    if not sep or tail not in _PHASES:
        return (ctx, True)
    if not head:
        return ("", False)
    return (head, True)


def _rel(work_root: pathlib.Path, raw: str) -> str | None:
    root = work_root.resolve()
    p = pathlib.Path(raw)
    if not p.is_absolute():
        p = root / p
    try:
        return p.resolve().relative_to(root).as_posix()
    except ValueError:
        # site-packages, the stdlib, a sibling checkout: not this repo's code.
        return None


def _has_arcs(conn: sqlite3.Connection) -> bool:
    try:
        row = conn.execute("SELECT value FROM meta WHERE key = 'has_arcs'").fetchone()
    except sqlite3.Error:
        return False
    if row is None:
        return False
    # coverage.py has written both the int and the repr of the bool over its life.
    return str(row[0]) in ("1", "True", "true")


def _rows(conn: sqlite3.Connection) -> list[tuple[str, str, list[int]]]:
    if _has_arcs(conn):
        return [
            (path, ctx or "", [fromno, tono])
            for path, ctx, fromno, tono in conn.execute(_ARC_QUERY)
        ]
    return [
        (path, ctx or "", numbits(blob or b""))
        for path, ctx, blob in conn.execute(_LINE_BITS_QUERY)
    ]


def read_coverage(db_path: pathlib.Path, work_root: pathlib.Path) -> CoverageTruth:
    conn = sqlite3.connect(f"file:{db_path}?mode=ro", uri=True)
    try:
        rows = _rows(conn)
    finally:
        conn.close()

    by_test: dict[str, set[tuple[str, int]]] = {}
    import_time: set[tuple[str, int]] = set()
    for raw_path, raw_ctx, lines in rows:
        rel = _rel(work_root, raw_path)
        if rel is None:
            continue
        # Line 0 is the synthetic module frame and negatives are arc scope
        # sentinels; neither is a line a test could assert on.
        pairs = {(rel, ln) for ln in lines if ln > 0}
        if not pairs:
            continue
        test_id, is_test = normalize_context(raw_ctx)
        if is_test:
            # The |setup, |run and |teardown phases of one test are one entry.
            by_test.setdefault(test_id, set()).update(pairs)
        else:
            import_time.update(pairs)

    covered: set[tuple[str, int]] = set()
    for pairs in by_test.values():
        covered.update(pairs)
    return CoverageTruth(
        by_test={k: frozenset(v) for k, v in by_test.items()},
        import_time=frozenset(import_time),
        covered=frozenset(covered),
    )
