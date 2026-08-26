"""Evaluate an arm's predictions with the official SWE-bench Docker harness.

The harness is authoritative for pass/fail; this module only marshals inputs and
reads its report files back through ``metrics.from_report``, which owns the
definition of a regression. Nothing here re-decides what a passing test is, and
nothing here computes a rate.

Three properties are structural rather than remembered:

  1. **The frozen sample is the spine.** Both ``write_predictions`` and
     ``collect`` iterate ``instances.txt``, never the directory listing of
     whatever happened to be produced. An arm that crashed on an instance, or an
     evaluation that lost one, therefore still contributes a line and a row —
     a missing file cannot shrink a denominator.
  2. **Reports outlive the containers.** ``collect`` mirrors every per-instance
     report under ``bench/results/swebench/reports/<arm>/`` and reads that mirror
     when the harness log tree is gone, so analysis never depends on Docker.
  3. **A lost instance is an error, not a zero.** An instance carrying a patch
     that the harness never reported on comes back ``evaluated=False`` with its
     dataset-derived denominator intact, and ``harness_errors`` names it.
"""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
from collections.abc import Callable, Sequence
from pathlib import Path

import sample
from metrics import InstanceResult, from_report

RUN_ID_PREFIX = "rtdd-m4"


def results_dir(repo_root: Path) -> Path:
    return repo_root / "bench" / "results" / "swebench"


def run_id_for(arm: str) -> str:
    """Stable and arm-scoped, so a re-run reuses the same harness cache tree."""
    return f"{RUN_ID_PREFIX}-{arm}"


def harness_report_dir(repo_root: Path, arm: str) -> Path:
    """Where the official harness leaves this arm's per-instance reports."""
    return results_dir(repo_root) / "logs" / "run_evaluation" / run_id_for(arm)


def report_dir(repo_root: Path, arm: str) -> Path:
    """Our own copy of those reports, kept re-readable without Docker."""
    return results_dir(repo_root) / "reports" / arm


def instance_ids(repo_root: Path) -> list[str]:
    """The frozen sample, in file order — the only list either half iterates."""
    path = repo_root / "bench" / "swebench" / "instances.txt"
    return [
        line.strip()
        for line in path.read_text(encoding="utf-8").splitlines()
        if line.strip()
    ]


def _raw_record(repo_root: Path, arm: str, instance_id: str) -> dict:
    path = results_dir(repo_root) / "raw" / arm / f"{instance_id}.json"
    if not path.exists():
        return {}
    return json.loads(path.read_text(encoding="utf-8"))


def write_predictions(repo_root: Path, arm: str) -> Path:
    """Render the arm's raw records into the official prediction schema.

    One line per instance in the frozen sample, including the instances whose
    patch is empty or missing: the harness must be asked about all of them so
    the report tree covers the sample the analysis will divide by.
    """
    out = results_dir(repo_root) / "predictions" / f"{arm}.jsonl"
    out.parent.mkdir(parents=True, exist_ok=True)
    lines = [
        json.dumps(
            {
                "instance_id": instance_id,
                "model_name_or_path": f"rtdd-{arm}",
                "model_patch": _raw_record(repo_root, arm, instance_id).get("patch")
                or "",
            }
        )
        for instance_id in instance_ids(repo_root)
    ]
    out.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return out


def evaluation_command(predictions: Path, arm: str, workers: int) -> list[str]:
    return [
        sys.executable,
        "-m",
        "swebench.harness.run_evaluation",
        "--dataset_name",
        sample.DATASET,
        "--split",
        sample.SPLIT,
        "--predictions_path",
        str(predictions),
        "--max_workers",
        str(workers),
        "--run_id",
        run_id_for(arm),
        "--cache_level",
        "env",
    ]


def run_evaluation(
    repo_root: Path,
    arm: str,
    workers: int = 8,
    runner: Callable[..., subprocess.CompletedProcess] = subprocess.run,
) -> Path:
    """Run the official harness over this arm and return its report directory.

    ``runner`` is a seam, not a convenience: it lets the argv this function
    builds be asserted on without a Docker daemon anywhere near the test suite.
    """
    predictions = write_predictions(repo_root, arm)
    results_dir(repo_root).mkdir(parents=True, exist_ok=True)
    proc = runner(
        evaluation_command(predictions, arm, workers),
        cwd=str(results_dir(repo_root)),
    )
    if proc.returncode != 0:
        raise RuntimeError(
            f"run_evaluation failed for arm {arm} (exit {proc.returncode})"
        )
    return harness_report_dir(repo_root, arm)


def _harness_report(repo_root: Path, arm: str, instance_id: str) -> dict:
    """This run's report for one instance, from the harness tree or our mirror.

    The harness nests reports under the model name it read from the predictions
    file, so the instance directory is found rather than constructed.
    """
    log_root = harness_report_dir(repo_root, arm)
    if log_root.exists():
        for match in sorted(log_root.rglob(f"{instance_id}/report.json")):
            return json.loads(match.read_text(encoding="utf-8"))
    mirror = report_dir(repo_root, arm) / f"{instance_id}.json"
    if mirror.exists():
        return json.loads(mirror.read_text(encoding="utf-8"))
    return {}


def _patch_applied(record: dict, report: dict, instance_id: str) -> bool:
    """Whether this instance's patch reached the repository under test.

    An empty patch is never applied. Otherwise the harness's own verdict wins if
    it gave one; a patch with no report at all is treated as applied so the lost
    instance surfaces as a harness error rather than hiding among the arm's own
    empty outputs.
    """
    if not (record.get("patch") or "").strip():
        return False
    entry = report.get(instance_id) if isinstance(report, dict) else None
    if isinstance(entry, dict) and "patch_successfully_applied" in entry:
        return bool(entry["patch_successfully_applied"])
    return True


def collect(
    repo_root: Path, arm: str, rows: dict[str, dict] | None = None
) -> list[InstanceResult]:
    """Fold this arm's reports into one :class:`InstanceResult` per frozen instance.

    ``rows`` is the dataset, injectable so the suite need not hit the network;
    it supplies every denominator, exactly as ``metrics`` requires.
    """
    if rows is None:
        rows = sample.load_rows()
    out_dir = report_dir(repo_root, arm)
    out_dir.mkdir(parents=True, exist_ok=True)

    results: list[InstanceResult] = []
    for instance_id in instance_ids(repo_root):
        record = _raw_record(repo_root, arm, instance_id)
        report = _harness_report(repo_root, arm, instance_id)
        (out_dir / f"{instance_id}.json").write_text(
            json.dumps(report, indent=2), encoding="utf-8"
        )
        results.append(
            from_report(
                instance_id,
                arm,
                rows[instance_id],
                report,
                _patch_applied(record, report, instance_id),
            )
        )
    return results


def harness_errors(results: Sequence[InstanceResult]) -> tuple[str, ...]:
    """Instances whose patch was applied but which the harness never scored.

    These are the evaluation's own failures. They are named rather than counted
    into some silent zero, because an arm that looks clean because a quarter of
    its instances went missing is not a clean arm.
    """
    return tuple(r.instance_id for r in results if r.patch_applied and not r.evaluated)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("arm")
    parser.add_argument("--workers", type=int, default=8)
    parser.add_argument(
        "--collect-only",
        action="store_true",
        help="re-read the saved reports without running Docker again",
    )
    args = parser.parse_args()
    repo_root = Path(__file__).resolve().parents[2]

    if not args.collect_only:
        run_evaluation(repo_root, args.arm, args.workers)
    results = collect(repo_root, args.arm)
    errors = harness_errors(results)
    print(f"{args.arm}: collected {len(results)} instance results")
    print(f"{args.arm}: harness errors {len(errors)}")
    for instance_id in errors:
        print(f"  harness error: {instance_id}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
