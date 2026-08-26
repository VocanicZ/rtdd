"""The pytest execution layer: three run modes behind one content-addressed cache.

The benchmark needs three different pytest invocations of the same commit and it
needs to keep them apart. A **full uninstrumented** run is the honest wall-clock
baseline a developer actually pays. A **full instrumented** run is the same suite
under ``--cov-context=test``; the difference between the two is the
instrumentation tax, and it only exists as its own column because both runs are
timed separately. A **subset** run is what a selection strategy would really have
cost, and is the only way to check the outcome-independence assumption the
analytic scoring rests on.

Two failure modes are treated as findings rather than crashes:

* A **collection error** — one unimportable test module — is recorded as an
  ``error`` outcome for that module and the rest of the suite still runs
  (``--continue-on-collection-errors``). Real corpus commits do occasionally fail
  to import; losing the whole commit's ground truth to one of them would silently
  bias the corpus toward tidy repos.
* A **lost dynamic context** is not survivable and raises
  :class:`SysmonContextError`. Under ``COVERAGE_CORE=sysmon`` coverage.py drops
  per-test contexts and warns ``no-sysmon-context``; the resulting map is ~90%
  empty (audit A7) while the run still exits 0. Measuring that would be worse
  than not measuring at all, so the runner forces ``ctrace`` and treats the
  warning as fatal.

Every run can be routed through :func:`cached_run`, whose keys come from
:func:`run_key` and are therefore salted with the :class:`replay.config.RunConfig`
digest: a config change is a miss, never a silent mix of old and new numbers.
"""

from __future__ import annotations

import dataclasses
import hashlib
import json
import os
import pathlib
import subprocess
import sys
import tempfile
import time
from collections.abc import Callable, Sequence

from replay.cache import Cache

MAX_ARGV_BYTES = 100_000

MODE_FULL = "full"
MODE_FULL_INSTRUMENTED = "full-instrumented"
MODE_SUBSET = "subset"

_PHASE_STATUS = {"passed": "pass", "failed": "fail", "skipped": "skip"}


class NoTestsCollectedError(RuntimeError):
    """pytest exited 4 (bad selector) or 5 (nothing collected).

    A historical commit whose tree will not collect is a fact about that commit,
    so the orchestrator records it in `skipped` and walks on; it is a distinct
    type precisely so that catching it cannot also swallow a real harness fault.
    """


class SysmonContextError(RuntimeError):
    pass


@dataclasses.dataclass(frozen=True)
class Outcome:
    test: str
    status: str  # "pass" | "fail" | "skip" | "error"
    duration_ms: int


@dataclasses.dataclass(frozen=True)
class RunResult:
    outcomes: tuple[Outcome, ...]
    exit_code: int
    wall_ms: int
    collected: tuple[str, ...]
    cached: bool = False
    """True when the numbers were replayed from the cache. Wall-clock read back
    from a cache was measured on some earlier run; the flag keeps a reader from
    mistaking it for a fresh timing."""

    def failing(self) -> frozenset[str]:
        return frozenset(o.test for o in self.outcomes if o.status in ("fail", "error"))

    def durations(self) -> dict[str, int]:
        return {o.test: o.duration_ms for o in self.outcomes}


def _env(instrumented: bool) -> dict[str, str]:
    env = dict(os.environ)
    env["PYTHONDONTWRITEBYTECODE"] = "1"
    if instrumented:
        # Spec §4: sysmon silently drops dynamic contexts, ctrace does not.
        env["COVERAGE_CORE"] = "ctrace"
    return env


def parse_report_log(path: pathlib.Path) -> tuple[Outcome, ...]:
    """Parse a ``pytest --report-log`` JSONL stream into per-test outcomes.

    Only the call phase produces a pass/fail; a setup or teardown failure maps to
    ``error`` so a broken fixture is never scored as a pass. A failed
    ``CollectReport`` is recorded under the module's node id, but never over a
    node that already has a real per-test verdict.
    """
    status: dict[str, str] = {}
    duration: dict[str, float] = {}
    collect_errors: dict[str, bool] = {}

    for line in path.read_text(encoding="utf-8", errors="replace").splitlines():
        if not line.strip():
            continue
        rec = json.loads(line)
        report_type = rec.get("$report_type")

        if report_type == "CollectReport":
            if rec.get("outcome") == "failed" and rec.get("nodeid"):
                collect_errors[rec["nodeid"]] = True
            continue

        if report_type != "TestReport":
            continue

        nodeid = rec["nodeid"]
        phase = rec.get("when")
        outcome = rec.get("outcome")
        duration[nodeid] = duration.get(nodeid, 0.0) + float(rec.get("duration", 0.0))
        if phase == "call":
            status[nodeid] = _PHASE_STATUS.get(outcome, "error")
        elif phase in ("setup", "teardown") and outcome == "failed":
            status[nodeid] = "error"
        elif phase == "setup" and outcome == "skipped":
            status.setdefault(nodeid, "skip")

    for nodeid in collect_errors:
        # A per-test verdict wins: the test really ran, so its own report is the
        # authoritative one even if an enclosing collector also reported failed.
        status.setdefault(nodeid, "error")

    return tuple(
        Outcome(test=t, status=status[t], duration_ms=int(round(duration.get(t, 0.0) * 1000)))
        for t in sorted(status)
    )


def chunk(tests: Sequence[str], max_bytes: int = MAX_ARGV_BYTES) -> list[list[str]]:
    """Split node ids into batches that fit an argv budget.

    A single id longer than the budget still gets its own batch rather than being
    dropped — an over-long argv is the OS's problem to report, a silently omitted
    test would be a wrong benchmark number.
    """
    parts: list[list[str]] = []
    cur: list[str] = []
    size = 0
    for t in tests:
        cost = len(t) + 1
        if cur and size + cost > max_bytes:
            parts.append(cur)
            cur, size = [], 0
        cur.append(t)
        size += cost
    if cur:
        parts.append(cur)
    return parts


def collect(work: pathlib.Path, python: str = sys.executable) -> tuple[str, ...]:
    proc = subprocess.run(
        [
            python,
            "-m",
            "pytest",
            "--collect-only",
            "-q",
            "--no-header",
            "-p",
            "no:cacheprovider",
        ],
        cwd=work,
        capture_output=True,
        text=True,
        env=_env(False),
    )
    ids = []
    for line in proc.stdout.splitlines():
        line = line.strip()
        if "::" in line and not line.startswith(("=", "-", "<")):
            ids.append(line)
    return tuple(sorted(set(ids)))


def _pytest_argv(
    instrumented: bool,
    source_globs: Sequence[str],
    log: pathlib.Path,
    exec_args: Sequence[str],
    cacheprovider: bool = False,
) -> list[str]:
    argv = ["-q", "--no-header", "--continue-on-collection-errors", f"--report-log={log}"]
    if not cacheprovider:
        # `--lf` is the one baseline that needs pytest's cache to survive a run;
        # every other mode disables it so one commit's run cannot leak state into
        # the next one's.
        argv += ["-p", "no:cacheprovider"]
    # A strategy whose whole intervention *is* the execution mode — `pytest -n auto`
    # — carries it here in `Selection.exec_args`. This is the only place those flags
    # become real; a baseline whose flags never reach argv is its own control.
    argv += list(exec_args)
    if instrumented:
        for src in source_globs or (".",):
            argv.append(f"--cov={src}")
        argv += ["--cov-context=test", "--cov-report=", "--cov-branch"]
    return argv


def _invoke(
    work: pathlib.Path,
    python: str,
    tests: Sequence[str],
    instrumented: bool,
    source_globs: Sequence[str],
    exec_args: Sequence[str],
    cacheprovider: bool = False,
) -> RunResult:
    outcomes: list[Outcome] = []
    exit_code = 0
    wall_ms = 0
    batches = chunk(list(tests)) if tests else [[]]
    for batch in batches:
        with tempfile.TemporaryDirectory() as td:
            log = pathlib.Path(td) / "report.jsonl"
            argv = [
                python,
                "-m",
                "pytest",
                *_pytest_argv(instrumented, source_globs, log, exec_args, cacheprovider),
                *batch,
            ]
            start = time.perf_counter()
            proc = subprocess.run(
                argv, cwd=work, capture_output=True, text=True, env=_env(instrumented)
            )
            wall_ms += int(round((time.perf_counter() - start) * 1000))
            if instrumented and "no-sysmon-context" in (proc.stderr + proc.stdout):
                raise SysmonContextError(
                    "coverage.py emitted no-sysmon-context: dynamic contexts were dropped. "
                    "Any map built from this run is ~90% empty (audit A7)."
                )
            if proc.returncode in (4, 5):
                raise NoTestsCollectedError(
                    f"pytest exit {proc.returncode} (bad selector / no tests collected) in {work}: "
                    f"{proc.stdout[-2000:]}"
                )
            if log.exists():
                outcomes.extend(parse_report_log(log))
            exit_code = exit_code or proc.returncode
    uniq = {o.test: o for o in outcomes}
    ordered = tuple(uniq[t] for t in sorted(uniq))
    return RunResult(
        outcomes=ordered,
        exit_code=exit_code,
        wall_ms=wall_ms,
        collected=tuple(o.test for o in ordered),
    )


def run_full(
    work: pathlib.Path,
    python: str = sys.executable,
    instrumented: bool = False,
    source_globs: Sequence[str] = (),
    exec_args: Sequence[str] = (),
    cacheprovider: bool = False,
) -> RunResult:
    return _invoke(work, python, (), instrumented, source_globs, exec_args, cacheprovider)


def run_subset(
    work: pathlib.Path,
    tests: Sequence[str],
    python: str = sys.executable,
    instrumented: bool = False,
    source_globs: Sequence[str] = (),
    exec_args: Sequence[str] = (),
    cacheprovider: bool = False,
) -> RunResult:
    if not tests:
        return RunResult(outcomes=(), exit_code=0, wall_ms=0, collected=())
    return _invoke(work, python, tests, instrumented, source_globs, exec_args, cacheprovider)


# --- cache integration ------------------------------------------------------


def result_to_dict(res: RunResult) -> dict:
    return {
        "outcomes": [
            {"test": o.test, "status": o.status, "duration_ms": o.duration_ms}
            for o in res.outcomes
        ],
        "exit_code": res.exit_code,
        "wall_ms": res.wall_ms,
        "collected": list(res.collected),
    }


def result_from_dict(obj: dict) -> RunResult:
    return RunResult(
        outcomes=tuple(
            Outcome(test=o["test"], status=o["status"], duration_ms=int(o["duration_ms"]))
            for o in obj["outcomes"]
        ),
        exit_code=int(obj["exit_code"]),
        wall_ms=int(obj["wall_ms"]),
        collected=tuple(obj["collected"]),
    )


def run_key(
    cache: Cache,
    mode: str,
    repo_id: str,
    commit: str,
    variant: str,
    strategy: str,
    tests: Sequence[str] = (),
    exec_args: Sequence[str] = (),
) -> str:
    """Cache key for one run, salted with the config digest by :meth:`Cache.key`.

    A subset run is additionally keyed by the *set* of selected tests: two
    strategies that happen to select the same tests may share the run, and the
    same strategy re-ordering its answer must not be a miss.

    It is keyed by `exec_args` too, and must be: `xdist` selects exactly what
    `full` selects, so without this a serial full run would be served straight
    back as the parallel baseline's timing and the two rows would agree to the
    millisecond — which is the bug this key separation exists to prevent.
    """
    parts = [mode, repo_id, commit, variant, strategy]
    if tests:
        digest = hashlib.sha256("\0".join(sorted(set(tests))).encode("utf-8")).hexdigest()
        parts.append(digest)
    if exec_args:
        parts.append("exec:" + " ".join(exec_args))
    return cache.key(*parts)


def cached_run(cache: Cache, key: str, build: Callable[[], RunResult]) -> RunResult:
    before = cache.stats()["hits"]
    obj = cache.json_or_build(key, lambda: result_to_dict(build()))
    res = result_from_dict(obj)
    return dataclasses.replace(res, cached=cache.stats()["hits"] > before)
