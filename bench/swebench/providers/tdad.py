"""Arm C's context source: TDAD's own reference implementation, run locally.

Verified installable 2026-08-26 — github.com/pepealonso95/TDAD, MIT, Python,
Python-repo targeted, CLI ``tdad index`` / ``tdad impact``. Running it rather than
citing it is what puts all five arms on one harness, which is the only way the
comparison means anything: a delta measured against our vanilla arm and a delta
published against theirs are two numbers from two scaffolds, and the difference
between two such deltas is not a ranking.

Three things this module holds to, because each is a way the arm could quietly
stop being a fair comparison:

* **TDAD's own code is never edited.** The bridge lives entirely in this repo. The
  checkout in the cache is left byte-identical to the pinned commit, and the one
  concession to its packaging — installing the ``neo4j`` extra, whose driver
  ``tdad.analyzer.impact`` imports at module scope even under the default
  ``networkx`` backend — is an install flag, not a patch.
* **The pin is a resolved commit sha**, recorded here, in
  ``bench/results/swebench/config.json`` and in ``bench/PREREGISTRATION.md``
  beside the RTDD commit under test. A branch name would let the arm's context
  source change under a re-run.
* **Failure is loud.** If TDAD cannot be installed or indexed, :class:`TdadUnavailable`
  reaches the caller. The one thing that must never happen is arm C running
  against a context-free prompt and being published as though it had a map: that
  would report TDAD's arm as no better than vanilla when what actually happened is
  that TDAD never ran. The contingency — cite the published figure, in a labelled
  column, with the caveat — is written up front in ``docs/results/tdad-caveat.md``.

TDAD is installed into its own virtualenv inside the cache rather than into the
harness environment, so its dependency set cannot move ours and change what the
other four arms are running.
"""

from __future__ import annotations

import json
import os
import subprocess
from pathlib import Path

from tools import Tool, _clip
from workspace import Workspace

TDAD_REPO = "https://github.com/pepealonso95/TDAD.git"

#: The exact commit arm C runs. Resolved 2026-08-26 from ``refs/heads/main``.
TDAD_PIN = "73d1234d3fd02817bc58f1c03b434aca5027ea78"

#: Where the contingency is written down, quoted into every failure so the reader
#: of a traceback lands on the caveat rather than reconstructing it.
CAVEAT_DOC = "docs/results/tdad-caveat.md"

#: Overridable so tests (and any pinned toolchain) can point elsewhere.
TDAD_BIN_ENV = "RTDD_BENCH_TDAD_BIN"
UV_BIN_ENV = "RTDD_BENCH_UV_BIN"

#: TDAD's static map, relative to the repository it indexed.
STATIC_MAP = Path(".tdad") / "test_map.txt"

#: ``tdad.analyzer.impact`` imports the neo4j driver at module scope even though the
#: default backend is networkx, so the extra is required to run at all.
TDAD_PACKAGE_EXTRAS = "[neo4j]"

INSTALL_TIMEOUT_S = 1800
INDEX_TIMEOUT_S = 1800
IMPACT_TIMEOUT_S = 600
HELP_TIMEOUT_S = 120

_MAX_DETAIL_BYTES = 4_000

#: Set by :func:`ensure_installed` so ``index`` and ``impact_tool`` — which take a
#: workspace and no cache — can find the CLI that was just installed.
_INSTALLED_BIN: Path | None = None


class TdadUnavailable(RuntimeError):
    """TDAD could not be installed or indexed — take the contingency branch."""

    def __init__(self, detail: str) -> None:
        super().__init__(
            f"{detail[-_MAX_DETAIL_BYTES:]}\n"
            f"arm C cannot be run locally; the contingency this triggers is written up in "
            f"{CAVEAT_DOC}"
        )


def tdad_bin() -> str:
    """The ``tdad`` CLI: an explicit override, then the cached install, then PATH."""
    override = os.environ.get(TDAD_BIN_ENV)
    if override:
        return override
    if _INSTALLED_BIN is not None:
        return str(_INSTALLED_BIN)
    return "tdad"


def _uv_bin() -> str:
    return os.environ.get(UV_BIN_ENV) or "uv"


def _run(argv: list[str], *, timeout_s: int, what: str) -> subprocess.CompletedProcess:
    try:
        proc = subprocess.run(argv, capture_output=True, text=True, timeout=timeout_s)
    except subprocess.TimeoutExpired as exc:
        raise TdadUnavailable(f"{what} exceeded {timeout_s}s") from exc
    except OSError as exc:
        raise TdadUnavailable(f"{what} could not be run: {exc}") from exc
    if proc.returncode != 0:
        raise TdadUnavailable(f"{what} failed (exit {proc.returncode})\n{proc.stderr}")
    return proc


def _has_commit(checkout: Path, pin: str) -> bool:
    return (
        subprocess.run(
            ["git", "-C", str(checkout), "cat-file", "-e", f"{pin}^{{commit}}"],
            capture_output=True,
            text=True,
        ).returncode
        == 0
    )


def ensure_installed(cache: Path, *, repo: str | None = None, pin: str | None = None) -> str:
    """Clone the pin into ``cache``, install it, and return the resolved sha.

    A second call over the same cache is a no-op returning the same sha: the arm
    runs once per instance and must not re-clone or re-install 100 times.
    """
    global _INSTALLED_BIN

    repo = repo or TDAD_REPO
    pin = pin or TDAD_PIN

    home = cache / "tdad"
    checkout = home / "src"
    venv = home / "venv"
    binary = venv / "bin" / "tdad"
    marker = home / "installed.json"

    if marker.exists():
        recorded = json.loads(marker.read_text(encoding="utf-8"))
        if recorded.get("pin") == pin and Path(recorded["bin"]).exists():
            _INSTALLED_BIN = Path(recorded["bin"])
            return str(recorded["pin"])

    if not (checkout / ".git").exists():
        home.mkdir(parents=True, exist_ok=True)
        _run(
            ["git", "clone", "--quiet", repo, str(checkout)],
            timeout_s=INSTALL_TIMEOUT_S,
            what=f"cloning {repo}",
        )

    if not _has_commit(checkout, pin):
        _run(
            ["git", "-C", str(checkout), "fetch", "--quiet", "--tags", "origin"],
            timeout_s=INSTALL_TIMEOUT_S,
            what=f"fetching {repo}",
        )
    if not _has_commit(checkout, pin):
        raise TdadUnavailable(f"the pinned commit {pin} is not in {repo} — the pin has rotted")

    _run(
        ["git", "-C", str(checkout), "checkout", "--quiet", "--detach", pin],
        timeout_s=INSTALL_TIMEOUT_S,
        what=f"checking out {pin}",
    )
    resolved = _run(
        ["git", "-C", str(checkout), "rev-parse", "HEAD"],
        timeout_s=HELP_TIMEOUT_S,
        what="resolving the pinned commit",
    ).stdout.strip()
    if resolved != pin:
        raise TdadUnavailable(f"the pin {pin} resolved to {resolved}")

    _run([_uv_bin(), "venv", str(venv)], timeout_s=INSTALL_TIMEOUT_S, what="creating TDAD's venv")
    _run(
        [
            _uv_bin(),
            "pip",
            "install",
            "--quiet",
            "--python",
            str(venv / "bin" / "python"),
            "-e",
            f"{checkout / 'tdad'}{TDAD_PACKAGE_EXTRAS}",
        ],
        timeout_s=INSTALL_TIMEOUT_S,
        what="the TDAD install",
    )
    if not binary.exists():
        raise TdadUnavailable(f"the TDAD install left no CLI at {binary}")
    _run([str(binary), "--help"], timeout_s=HELP_TIMEOUT_S, what="the tdad CLI")

    marker.write_text(json.dumps({"pin": resolved, "bin": str(binary)}), encoding="utf-8")
    _INSTALLED_BIN = binary
    return resolved


def index(ws: Workspace) -> Path:
    """Build TDAD's static map for ``ws`` and return its path.

    ``tdad index`` degrades rather than failing — a graph backend it cannot reach
    falls back to a filename heuristic, and both paths exit 0 — so the map file
    itself is the check. An index that wrote no map, or an empty one, is an arm
    with no context, which is the contingency and not a quiet zero.
    """
    _run(
        [tdad_bin(), "index", str(ws.root)],
        timeout_s=INDEX_TIMEOUT_S,
        what=f"tdad index on {ws.instance_id}",
    )
    static_map = ws.root / STATIC_MAP
    if not static_map.is_file() or not static_map.read_text(encoding="utf-8").strip():
        raise TdadUnavailable(
            f"tdad index on {ws.instance_id} exited 0 but wrote no static map at {STATIC_MAP}"
        )
    return static_map


# --- the context tool -------------------------------------------------------

_DESCRIPTION = (
    "Report the tests at risk of regressing if the given source files change, from "
    "a static index of this repository."
)


def impact_tool(ws: Workspace) -> Tool:
    """The context tool arm C carries: ``tdad impact``, rendered as context."""

    def call(args: dict) -> str:
        files = [str(f) for f in (args.get("files") or [])]
        if not files:
            return "no files given"
        flags = [f for f in files if f.startswith("-")]
        if flags:
            # Every argument after --files is a path; one that reads as a flag would
            # be TDAD's own option parser being steered from inside the tool call.
            return f"not a file path: {', '.join(flags)}"
        try:
            proc = subprocess.run(
                [tdad_bin(), "impact", str(ws.root), "--files", *files],
                capture_output=True,
                text=True,
                timeout=IMPACT_TIMEOUT_S,
            )
        except (subprocess.TimeoutExpired, OSError) as exc:
            return f"test_impact unavailable: {exc}"
        if proc.returncode != 0:
            return f"test_impact unavailable: {proc.stderr.strip()[:500]}"
        return _clip(proc.stdout.strip() or "(no impacted tests reported)")

    return Tool(
        name="test_impact",
        schema={
            "type": "function",
            "function": {
                "name": "test_impact",
                "description": _DESCRIPTION,
                "parameters": {
                    "type": "object",
                    "properties": {
                        "files": {"type": "array", "items": {"type": "string"}},
                    },
                    "required": ["files"],
                    "additionalProperties": False,
                },
            },
        },
        call=call,
    )
