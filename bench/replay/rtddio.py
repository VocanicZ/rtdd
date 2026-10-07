"""The only module that shells out to ``rtdd`` or knows its ``--json`` schema.

Everything the benchmark learns about the system under test arrives through here,
so a schema change has exactly one place to be fixed. Three properties are
load-bearing:

**Shipped defaults only.** The harness invokes ``rtdd`` the way a user would —
``rtdd graph`` and ``rtdd which --base HEAD --json`` — and nothing else. There is
deliberately no parameter for extra argv: a knob for "just one tuning flag" is how
a benchmark quietly stops measuring the shipped tool, and
:func:`test_no_entry_point_accepts_extra_rtdd_arguments` guards it.

**Never a silent empty selection.** A missing binary, a non-zero exit, malformed
JSON, a ``schema`` other than 3, or a document with no ``rounds`` all raise
:class:`RtddError`. The one thing that must never happen is an empty selection
manufactured from a failure, because that scores as "RTDD chose to run nothing"
— a perfect recall miss attributed to the tool rather than to the harness.

**The consumed schema is 3**, emitted by ``cmd/rtdd/which.go``: ``rounds`` holds
Round 1 (the tests linked to a changed node), Round 2 (the rest of their files,
excluding Round 1) and Round 3 (the full suite). Each test is ``{id, file, name}``
with ``id`` = ``<file>::<qualified name>``; :mod:`replay.graphids` turns those into
collected pytest ids. Round 3 is not read — the full suite is the ``full``
strategy, measured as ``/tdd``. v0.3.0 has no ``rtdd seed``, no ``rtdd run``, no
tiers and no uncovered report, so nothing here parses any of them.
"""

from __future__ import annotations

import dataclasses
import hashlib
import json
import os
import pathlib
import shutil
import subprocess
import time

SCHEMA_VERSION = 3

_STRICT_OK_CODES = (0,)


class RtddError(RuntimeError):
    pass


@dataclasses.dataclass(frozen=True)
class WhichResult:
    round1: tuple[str, ...]  # schema-3 test ids, document order
    round2: tuple[str, ...]  # schema-3 test ids; excludes round1 by construction (spec §7)
    changed: tuple[str, ...]  # every changed path, deleted ones included
    untested: tuple[str, ...]  # node ids
    source: str  # graph.source: "scanner" or "graphify"
    wall_ms: int  # measured around the subprocess, never read from the document
    warnings: tuple[str, ...] = ()


def _require_schema(payload: object, command: str) -> dict:
    if not isinstance(payload, dict):
        raise RtddError(f"rtdd {command} --json emitted {type(payload).__name__}, not an object")
    if "schema" not in payload:
        raise RtddError(f"rtdd {command} --json carries no schema version; refusing to guess")
    # `type(...) is int` so neither "3" nor True passes for 3.
    if type(payload["schema"]) is not int or payload["schema"] != SCHEMA_VERSION:
        raise RtddError(
            f"rtdd {command} --json is schema {payload['schema']!r}, "
            f"this harness consumes schema {SCHEMA_VERSION}"
        )
    return payload


def _require(payload: dict, key: str, command: str) -> object:
    if key not in payload:
        raise RtddError(f"rtdd {command} --json has no {key!r} field")
    return payload[key]


def _list(value: object, what: str) -> list:
    if not isinstance(value, list):
        raise RtddError(f"rtdd which --json: {what} is {type(value).__name__}, not a list")
    return value


def _strings(value: object, what: str) -> tuple[str, ...]:
    return tuple(str(v) for v in _list(value, what))


def _changed_paths(changed: object) -> tuple[str, ...]:
    out: list[str] = []
    for c in _list(changed, "changed"):
        if not isinstance(c, dict) or "path" not in c:
            raise RtddError(f"rtdd which --json: malformed changed entry: {c!r}")
        out.append(str(c["path"]))
    return tuple(out)


def _round_tests(rounds: list, n: int) -> tuple[str, ...]:
    for r in rounds:
        if isinstance(r, dict) and r.get("round") == n:
            break
    else:
        raise RtddError(f"rtdd which --json has no round {n}")
    ids: list[str] = []
    for t in _list(_require(r, "tests", "which"), f"round {n} tests"):
        if not isinstance(t, dict) or "id" not in t:
            raise RtddError(f"rtdd which --json: a test in round {n} has no id")
        ids.append(str(t["id"]))
    return tuple(ids)


def parse_which(payload: object, wall_ms: int) -> WhichResult:
    """Parse a schema-3 ``rtdd which`` document into what the bench scores."""
    payload = _require_schema(payload, "which")
    rounds = _list(_require(payload, "rounds", "which"), "rounds")
    graph = _require(payload, "graph", "which")
    if not isinstance(graph, dict):
        raise RtddError("rtdd which --json: graph is not an object")
    return WhichResult(
        round1=_round_tests(rounds, 1),
        round2=_round_tests(rounds, 2),
        changed=_changed_paths(_require(payload, "changed", "which")),
        untested=_strings(_require(payload, "untested", "which"), "untested"),
        source=str(_require(graph, "source", "which")),
        wall_ms=int(wall_ms),
        warnings=_strings(payload.get("warnings") or [], "warnings"),
    )


def _invoke(
    work: pathlib.Path, argv: list[str], ok_codes: tuple[int, ...] = _STRICT_OK_CODES
) -> tuple[str, int, int]:
    env = dict(os.environ)
    start = time.perf_counter()
    try:
        proc = subprocess.run(argv, cwd=work, capture_output=True, text=True, env=env)
    except (FileNotFoundError, NotADirectoryError, PermissionError) as exc:
        raise RtddError(f"rtdd binary not found or not executable: {argv[0]!r} ({exc})") from exc
    wall_ms = int(round((time.perf_counter() - start) * 1000))
    if proc.returncode not in ok_codes:
        raise RtddError(
            f"{' '.join(argv)} exited {proc.returncode} in {work}: {proc.stderr.strip()[-2000:]}"
        )
    return proc.stdout, proc.returncode, wall_ms


def _decode(out: str, command: str) -> dict:
    try:
        return json.loads(out)
    except json.JSONDecodeError as exc:
        raise RtddError(f"rtdd {command} --json emitted non-JSON: {out[:500]!r}") from exc


def rtdd_version(binary: str = "rtdd") -> str:
    """Return a reproducible identity for the binary, for :class:`RunConfig`.

    ``rtdd --version`` is tried first and v0.3.0 answers it, so its version
    string is recorded verbatim. A build that does not answer it — it exits
    non-zero on an unknown flag — falls back to the SHA-256 of the binary's own
    bytes, which pins the build at least as tightly as a version string would. A binary that
    cannot be found at all is an error: recording "" would make two different
    builds digest identically in ``RunConfig``.
    """
    resolved = shutil.which(binary) or (binary if os.path.isfile(binary) else None)
    if resolved is None:
        raise RtddError(f"rtdd binary not found on PATH or on disk: {binary!r}")
    try:
        proc = subprocess.run([resolved, "--version"], capture_output=True, text=True)
    except OSError as exc:
        raise RtddError(f"rtdd binary not found or not executable: {binary!r} ({exc})") from exc
    if proc.returncode == 0 and proc.stdout.strip():
        return proc.stdout.strip()
    digest = hashlib.sha256(pathlib.Path(resolved).read_bytes()).hexdigest()
    return f"sha256:{digest}"


def graph(work: pathlib.Path, binary: str = "rtdd") -> None:
    """Build (or refresh) ``.rtdd/graph.json`` at ``work``: the instance's one graph.

    Any non-zero exit is fatal — 2 is a configuration error and 3 a graph that
    cannot be built, and neither is a measurement.
    """
    _invoke(work, [binary, "graph"])


def which(work: pathlib.Path, binary: str = "rtdd", base: str = "HEAD") -> WhichResult:
    out, _, wall_ms = _invoke(work, [binary, "which", "--base", base, "--json"])
    return parse_which(_decode(out, "which"), wall_ms)
