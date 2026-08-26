"""Arm D/E context source: RTDD's own dynamic coverage map.

Seeding runs the instance's full suite once, instrumented, inside that instance's
SWE-bench Docker image at ``base_commit``. Seeds are cached by ``(repo, base_commit)``
and shared by the ``rtdd`` and ``rtdd_tdd`` arms, so a five-arm run over 100 instances
pays for ~100 seeds rather than 500. This is the expensive part of M4.

Pre-registered rule, stated here because it is the one a later edit is most likely to
soften: an instance whose seed fails is **NOT dropped**. It runs with an empty map, which
makes ``rtdd which`` report an unseeded map and select nothing, and the failure is
published as a ``seed_failed`` count. Dropping the hard instances would bias the arm
upward, and the hard instances are exactly the large suites — the case a coverage map is
supposed to earn its keep on.

A harness fault is not an instance result. A missing ``rtdd`` binary raises
:class:`SeedFailed` and is never cached: caching it would publish 100 identical
"seed_failed" rows that are a bug in the runner, not a property of the corpus.
"""

from __future__ import annotations

import hashlib
import json
import os
import shutil
import subprocess
from pathlib import Path

from tools import Tool, _clip
from workspace import Workspace

#: Wall-clock ceiling on one instrumented full-suite seed run, in seconds.
SEED_TIMEOUT_S = 5400

#: Overridable so tests (and any pinned toolchain) can point elsewhere.
DOCKER_BIN_ENV = "RTDD_BENCH_DOCKER_BIN"
RTDD_BIN_ENV = "RTDD_BENCH_RTDD_BIN"

#: Seed statuses. Anything other than ``SEEDED`` is counted and published as a
#: seed failure; the instance still runs.
SEEDED = "seeded"
SEED_FAILED = "seed_failed"
SEED_TIMEOUT = "seed_timeout"

#: SWE-bench's published per-instance image naming.
IMAGE_TEMPLATE = "swebench/sweb.eval.x86_64.{slug}:latest"

_MAX_STDERR_BYTES = 40_000


class SeedFailed(RuntimeError):
    """The instrumented seed run did not produce a usable map.

    ``status`` is the published seed status; ``detail`` is the captured evidence.
    """

    def __init__(self, status: str, detail: str = "") -> None:
        super().__init__(detail or status)
        self.status = status
        self.detail = detail


def _docker_bin() -> str:
    return os.environ.get(DOCKER_BIN_ENV) or "docker"


def _rtdd_bin() -> str:
    return os.environ.get(RTDD_BIN_ENV) or "rtdd"


def _rtdd_path() -> Path:
    """The rtdd binary to mount into the image, resolved on the host."""
    name = _rtdd_bin()
    found = shutil.which(name)
    if found is None and Path(name).is_file():
        found = name
    if found is None:
        raise SeedFailed(
            "rtdd_missing",
            f"the rtdd binary {name!r} is not on PATH — seeding cannot run, and this is a "
            f"harness fault rather than a property of any instance",
        )
    return Path(found).resolve()


def seed_cache_key(row: dict) -> str:
    """A stable digest of ``repo@base_commit``.

    Content-addressed, not name-addressed: two instances that share a base commit
    share one seed, and nothing in the key encodes an instance id.
    """
    raw = f"{row['repo']}@{row['base_commit']}".encode("utf-8")
    return hashlib.sha256(raw).hexdigest()[:16]


def image_for(instance_id: str) -> str:
    return IMAGE_TEMPLATE.format(slug=instance_id.replace("__", "_1776_"))


def _seed_in_image(ws: Workspace, timeout_s: int) -> str:
    """Run the instrumented full suite in the instance's image; return the map."""
    rtdd = _rtdd_path()
    script = (
        "set -e; cd /testbed; "
        "export COVERAGE_CORE=ctrace; "
        "rtdd init >/dev/null 2>&1 || true; "
        "rtdd seed; "
        "cat .rtdd/map.jsonl"
    )
    try:
        proc = subprocess.run(
            [
                _docker_bin(),
                "run",
                "--rm",
                "-v",
                f"{rtdd}:/usr/local/bin/rtdd:ro",
                image_for(ws.instance_id),
                "bash",
                "-lc",
                script,
            ],
            capture_output=True,
            text=True,
            timeout=timeout_s,
        )
    except subprocess.TimeoutExpired as exc:
        raise SeedFailed(
            SEED_TIMEOUT,
            f"the seed run exceeded {timeout_s}s on {ws.instance_id}",
        ) from exc
    if proc.returncode != 0 or not proc.stdout.strip():
        raise SeedFailed(
            SEED_FAILED,
            f"exit={proc.returncode} on {ws.instance_id}\n{proc.stderr}",
        )
    return proc.stdout


def ensure_seed(
    cache: Path, ws: Workspace, row: dict, *, timeout_s: int = SEED_TIMEOUT_S
) -> tuple[Path | None, str]:
    """Return ``(path to a cached map.jsonl or None, status)``.

    A cache hit — success or failure — is returned without re-running anything, so
    the second arm over an instance costs nothing and a suite that could not be
    seeded is not attempted 100 more times.
    """
    slot = cache / "seeds" / seed_cache_key(row)
    marker = slot / "status.json"
    if marker.exists():
        status = json.loads(marker.read_text(encoding="utf-8"))["status"]
        cached = slot / "map.jsonl"
        return ((cached, status) if status == SEEDED and cached.exists() else (None, status))

    # Outside the try: a harness fault must reach the caller unrecorded.
    _rtdd_path()

    slot.mkdir(parents=True, exist_ok=True)
    try:
        rows = _seed_in_image(ws, timeout_s)
    except SeedFailed as exc:
        (slot / "stderr.txt").write_text(exc.detail[-_MAX_STDERR_BYTES:], encoding="utf-8")
        _write_marker(marker, exc.status, exc.detail)
        return (None, exc.status)

    (slot / "map.jsonl").write_text(rows, encoding="utf-8")
    _write_marker(marker, SEEDED, "")
    return (slot / "map.jsonl", SEEDED)


def _write_marker(marker: Path, status: str, detail: str) -> None:
    payload = {"status": status}
    if detail:
        payload["detail"] = detail[-2_000:]
    marker.write_text(json.dumps(payload), encoding="utf-8")


def install_seed(ws: Workspace, seed: Path | None) -> bool:
    """Place the map where ``rtdd`` looks for it; report whether a real map landed.

    With ``None`` the instance proceeds with an empty map — the directory exists and
    the map does not, which is precisely the state ``rtdd`` calls unseeded.
    """
    rtdd_dir = ws.root / ".rtdd"
    rtdd_dir.mkdir(parents=True, exist_ok=True)
    if seed is None:
        return False
    shutil.copyfile(seed, rtdd_dir / "map.jsonl")
    (rtdd_dir / "meta.json").write_text(
        json.dumps(
            {"v": 1, "adapter": "python", "seeded_at": ws.base_commit, "cycles": 0},
        ),
        encoding="utf-8",
    )
    return True


# --- the context tool -------------------------------------------------------

#: rtdd's escalation reasons carry an operator-facing remediation clause. The tool
#: is context, not procedure, so the diagnosis is kept verbatim and the remedy —
#: which is addressed to whoever runs the benchmark, not to the agent — is dropped.
_REMEDIES = (": run rtdd seed", "; run rtdd seed", ": run `rtdd seed`")

_DESCRIPTION = (
    "Report the tests whose recorded execution covered this repository's current "
    "uncommitted changes, ranked by overlap, and the changed lines no recorded test "
    "executed. Takes no arguments."
)

_UNSEEDED_NOTE = (
    "no recorded coverage: this repository's map holds no recorded test execution, so "
    "the list below is empty for want of a recording rather than because no test "
    "covers these changes."
)


def _strip_remedy(text: str) -> str:
    for remedy in _REMEDIES:
        if text.endswith(remedy):
            return text[: -len(remedy)]
    return text


def _is_unseeded(ws: Workspace) -> bool:
    m = ws.root / ".rtdd" / "map.jsonl"
    return not m.is_file() or not m.read_text(encoding="utf-8").strip()


def render(doc: dict, *, unseeded: bool) -> str:
    """Turn one ``rtdd which --json`` document into the tool's text.

    Facts only: tiers, file names, line ranges, test ids, and rtdd's own diagnosis.
    Nothing here tells the agent what to do with any of it.
    """
    out: list[str] = []
    if unseeded:
        out.append(_UNSEEDED_NOTE)
        out.append("")
    out.append(f"tier: {doc.get('tier', '')} — {_strip_remedy(doc.get('reason', ''))}")

    changed = doc.get("changed") or []
    out.append(f"changed files ({len(changed)}):")
    for c in changed:
        lines = ", ".join(f"{r['start']}-{r['end']}" for r in c.get("lines") or [])
        suffix = f"  lines {lines}" if lines else ""
        out.append(f"  {c.get('status', '')} {c.get('path', '')}{suffix}")

    selection = doc.get("selection") or {}
    tests = selection.get("tests") or []
    out.append(f"tests whose recorded execution covered those changes ({len(tests)}):")
    out.extend(f"  {t}" for t in tests)

    unmapped = doc.get("unmapped_files") or []
    if unmapped:
        out.append(f"changed files no recorded test executed ({len(unmapped)}):")
        out.extend(f"  {p}" for p in unmapped)

    fallback = selection.get("import_fallback") or {}
    for path, ids in sorted(fallback.items()):
        out.append(f"test files importing {path}, from a static scan ({len(ids)}):")
        out.extend(f"  {i}" for i in ids)

    warnings = [_strip_remedy(w) for w in doc.get("warnings") or []]
    if warnings:
        out.append("what this report could not see:")
        out.extend(f"  {w}" for w in warnings)

    return "\n".join(out)


def which_tool(ws: Workspace) -> Tool:
    """The context tool arms D and E carry: ``rtdd which``, rendered as context."""

    def call(args: dict) -> str:
        try:
            proc = subprocess.run(
                [_rtdd_bin(), "which", "--base", "HEAD", "--json"],
                cwd=ws.root,
                capture_output=True,
                text=True,
                timeout=120,
            )
        except (subprocess.TimeoutExpired, OSError) as exc:
            return f"rtdd_which unavailable: {exc}"
        if proc.returncode != 0:
            return f"rtdd_which unavailable: {proc.stderr.strip()[:500]}"
        try:
            doc = json.loads(proc.stdout)
        except json.JSONDecodeError as exc:
            return f"rtdd_which unavailable: unparseable --json output: {exc}"
        return _clip(render(doc, unseeded=_is_unseeded(ws)))

    return Tool(
        name="rtdd_which",
        schema={
            "type": "function",
            "function": {
                "name": "rtdd_which",
                "description": _DESCRIPTION,
                "parameters": {
                    "type": "object",
                    "properties": {},
                    "required": [],
                    "additionalProperties": False,
                },
            },
        },
        call=call,
    )
