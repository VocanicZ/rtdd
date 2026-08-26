"""The only module that shells out to ``rtdd`` or knows its ``--json`` schema.

Everything the benchmark learns about the system under test arrives through here,
so a schema change has exactly one place to be fixed. Three properties are
load-bearing:

**Shipped defaults only.** The harness invokes ``rtdd`` the way a user would —
``rtdd seed``, ``rtdd which --base HEAD --json``, ``rtdd run --base HEAD --json``
— and nothing else. There is deliberately no parameter for extra argv: a knob for
"just one tuning flag" is how a benchmark quietly stops measuring the shipped
tool, and :func:`test_no_entry_point_accepts_extra_rtdd_arguments` guards it.

**Never a silent empty selection.** A missing binary, a non-zero exit, malformed
JSON, an unknown ``schema`` value, or a document with no ``selection`` all raise
:class:`RtddError`. The one thing that must never happen is an empty selection
manufactured from a failure, because that scores as "RTDD chose to run nothing"
— a perfect recall miss attributed to the tool rather than to the harness.

**The consumed schema is v1**, as defined in ``docs/plans/00-interfaces.md`` and
emitted by ``cmd/rtdd/jsonout.go``. Task 9 of the M3 plan sketched a flatter
shape; the shipped M2 binary emits the nested one, and Task 9 says explicitly
that this file is where that difference is reconciled. Two consequences worth
naming:

* There is no top-level ``cycles`` in the document. ``cycles`` lives in
  ``.rtdd/meta.json``, so :func:`which` reads it from there — see
  :func:`read_cycles`.
* There is no per-test ``outcomes`` array. v1 names only the *failing* test ids
  (``run.failures``) plus pass/fail/skip/error counts, so
  :attr:`RunOutput.outcomes` carries exactly the failures and the counts carry
  the rest. Inventing ``pass`` rows for the unnamed remainder would be fabricated
  data in a measurement harness.

``uncovered.available: false`` is **not** an error on its own: an empty selection
executes nothing, so it has no fresh coverage and therefore no honest uncovered
report, and that is a legitimate — indeed interesting — measurement. It is an
error only when the document also claims ``run.executed: true``, which is a
contract violation rather than an outcome. :attr:`RunOutput.uncovered_available`
keeps "no report" distinguishable from "nothing uncovered" downstream.
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
from collections.abc import Iterable

SCHEMA_VERSION = 1

CLASS_UNCOVERED = "uncovered"
CLASS_IMPORT_TIME = "import-time"

# `rtdd run` exits 1 when a selected test failed — a result, not a harness fault.
# Every other non-zero code (2 usage/config, 3 fatal environment) is fatal here.
_RUN_OK_CODES = (0, 1)
_STRICT_OK_CODES = (0,)


class RtddError(RuntimeError):
    pass


@dataclasses.dataclass(frozen=True)
class WhichResult:
    tier: str
    reason: str
    tests: tuple[str, ...]
    direct: tuple[str, ...]
    changed: tuple[str, ...]
    cycles: int
    wall_ms: int
    complete: bool = True
    warnings: tuple[str, ...] = ()

    def escalated(self) -> bool:
        return self.tier == "T2"


@dataclasses.dataclass(frozen=True)
class RunOutput:
    tier: str
    tests: tuple[str, ...]
    outcomes: tuple[tuple[str, str, int], ...]
    uncovered: frozenset[tuple[str, int]]
    import_time: frozenset[tuple[str, int]]
    exit_code: int
    wall_ms: int
    executed: bool = True
    uncovered_available: bool = True
    unavailable_reason: str = ""
    passed: int = 0
    failed: int = 0
    skipped: int = 0
    errored: int = 0
    failures: tuple[str, ...] = ()
    duration_ms: int = 0


def expand_ranges(
    entries: Iterable[dict], classes: Iterable[str] | None = None
) -> frozenset[tuple[str, int]]:
    """Expand ``[{path, ranges: [{start, end, class}]}]`` into ``(path, line)`` pairs.

    Ranges are 1-indexed and inclusive on both ends. ``classes``, when given,
    keeps only ranges whose ``class`` is in it; a range with no ``class`` key is
    kept only when ``classes`` is ``None``, so an unclassified document can never
    be silently counted as uncovered.
    """
    wanted = None if classes is None else set(classes)
    out: set[tuple[str, int]] = set()
    for e in entries or ():
        if not isinstance(e, dict) or "path" not in e:
            raise RtddError(f"malformed file entry, no path: {e!r}")
        path = str(e["path"])
        for r in e.get("ranges") or ():
            if not isinstance(r, dict):
                raise RtddError(f"malformed range in {path}: {r!r}")
            if wanted is not None and r.get("class") not in wanted:
                continue
            try:
                span = range(int(r["start"]), int(r["end"]) + 1)
            except (KeyError, TypeError, ValueError) as exc:
                raise RtddError(f"malformed range in {path}: {r!r}") from exc
            for line in span:
                out.add((path, line))
    return frozenset(out)


def _require_schema(payload: object, command: str) -> dict:
    if not isinstance(payload, dict):
        raise RtddError(f"rtdd {command} --json emitted {type(payload).__name__}, not an object")
    if "schema" not in payload:
        raise RtddError(f"rtdd {command} --json carries no schema version; refusing to guess")
    if payload["schema"] != SCHEMA_VERSION:
        raise RtddError(
            f"rtdd {command} --json is schema {payload['schema']!r}, "
            f"this harness consumes schema {SCHEMA_VERSION}"
        )
    return payload


def _require(payload: dict, key: str, command: str) -> object:
    if key not in payload:
        raise RtddError(f"rtdd {command} --json has no {key!r} field")
    return payload[key]


def _strings(value: object, what: str, command: str) -> tuple[str, ...]:
    if not isinstance(value, list):
        raise RtddError(f"rtdd {command} --json: {what} is {type(value).__name__}, not a list")
    return tuple(str(v) for v in value)


def _changed_paths(changed: list) -> tuple[str, ...]:
    out: list[str] = []
    for c in changed:
        if not isinstance(c, dict) or "path" not in c:
            raise RtddError(f"rtdd which --json: malformed changed entry: {c!r}")
        out.append(str(c["path"]))
    return tuple(out)


def parse_which(payload: dict, wall_ms: int, cycles: int = 0) -> WhichResult:
    """Parse a schema-v1 ``rtdd which`` document.

    ``cycles`` is passed in rather than read from the document: v1 does not carry
    it, and :func:`which` supplies it from ``.rtdd/meta.json``.
    """
    payload = _require_schema(payload, "which")
    tier = _require(payload, "tier", "which")
    selection = _require(payload, "selection", "which")
    if not isinstance(selection, dict):
        raise RtddError("rtdd which --json: selection is not an object")

    changed = _require(payload, "changed", "which")
    if not isinstance(changed, list):
        raise RtddError("rtdd which --json: changed is not a list")

    return WhichResult(
        tier=str(tier),
        reason=str(payload.get("reason", "")),
        tests=_strings(_require(selection, "tests", "which"), "selection.tests", "which"),
        direct=_strings(_require(selection, "direct", "which"), "selection.direct", "which"),
        changed=_changed_paths(changed),
        cycles=int(cycles),
        wall_ms=int(wall_ms),
        complete=bool(payload.get("complete", True)),
        warnings=_strings(payload.get("warnings") or [], "warnings", "which"),
    )


def parse_run(payload: dict, wall_ms: int) -> RunOutput:
    """Parse a schema-v1 ``rtdd run`` document."""
    payload = _require_schema(payload, "run")
    tier = _require(payload, "tier", "run")
    selection = _require(payload, "selection", "run")
    if not isinstance(selection, dict):
        raise RtddError("rtdd run --json: selection is not an object")

    run = _require(payload, "run", "run")
    if not isinstance(run, dict):
        raise RtddError("rtdd run --json: run is not an object")
    executed = bool(run.get("executed", False))
    failures = _strings(run.get("failures") or [], "run.failures", "run")

    unc = _require(payload, "uncovered", "run")
    if not isinstance(unc, dict):
        raise RtddError("rtdd run --json: uncovered is not an object")
    available = bool(unc.get("available", False))
    if executed and not available:
        raise RtddError(
            "rtdd run --json: the run executed tests but carries no uncovered report "
            f"({unc.get('reason', 'no reason given')!r}) — the false-signal measurement "
            "would silently read as zero"
        )
    files = unc.get("files") or []

    return RunOutput(
        tier=str(tier),
        tests=_strings(_require(selection, "tests", "run"), "selection.tests", "run"),
        # v1 names only the failing tests per-test; see the module docstring.
        outcomes=tuple((t, "fail", 0) for t in failures),
        uncovered=expand_ranges(files, {CLASS_UNCOVERED}),
        import_time=expand_ranges(files, {CLASS_IMPORT_TIME}),
        exit_code=int(_require(payload, "exit_code", "run")),
        wall_ms=int(wall_ms),
        executed=executed,
        uncovered_available=available,
        unavailable_reason="" if available else str(unc.get("reason", "")),
        passed=int(run.get("passed", 0)),
        failed=int(run.get("failed", 0)),
        skipped=int(run.get("skipped", 0)),
        errored=int(run.get("errored", 0)),
        failures=failures,
        duration_ms=int(run.get("duration_ms", 0)),
    )


def read_cycles(work: pathlib.Path) -> int:
    """Return ``.rtdd/meta.json``'s ``cycles``, the drift-guard counter.

    Absent before the first ``rtdd seed``, which is why a missing file is 0 and
    not an error. A file that exists but will not parse *is* an error: silently
    reporting 0 would misattribute a drift-guard escalation.
    """
    meta = pathlib.Path(work) / ".rtdd" / "meta.json"
    if not meta.exists():
        return 0
    try:
        return int(json.loads(meta.read_text()).get("cycles", 0))
    except (OSError, ValueError, TypeError, AttributeError) as exc:
        raise RtddError(f"unreadable .rtdd/meta.json in {work}: {exc}") from exc


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

    ``rtdd --version`` is tried first so a future build that grows one is
    reported verbatim. The shipped CLI has no such subcommand — it exits 2 on an
    unknown one — so the fallback is the SHA-256 of the binary's own bytes, which
    pins the build at least as tightly as a version string would. A binary that
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


def seed(work: pathlib.Path, binary: str = "rtdd") -> None:
    """Build ``.rtdd/map.jsonl`` with one full instrumented run.

    Any non-zero exit is fatal: seeding is the only operation that may shrink a
    map, so a partial seed produces a narrowed selection at every later commit.
    """
    # Exit 1 means a test failed *while seeding*; the map is written either way,
    # and a base tree with a red test is measured and subtracted by
    # `clean_tree_failures`. Only 2 (usage/config) and 3 (fatal environment) are
    # faults here — treating a red suite as fatal would abandon the replay at the
    # first commit whose history happened to be red.
    _invoke(work, [binary, "seed"], _RUN_OK_CODES)


def which(work: pathlib.Path, binary: str = "rtdd", base: str = "HEAD") -> WhichResult:
    out, _, wall_ms = _invoke(work, [binary, "which", "--base", base, "--json"])
    return parse_which(_decode(out, "which"), wall_ms, cycles=read_cycles(work))


def run(work: pathlib.Path, binary: str = "rtdd", base: str = "HEAD") -> RunOutput:
    out, _, wall_ms = _invoke(work, [binary, "run", "--base", base, "--json"], _RUN_OK_CODES)
    return parse_run(_decode(out, "run"), wall_ms)
