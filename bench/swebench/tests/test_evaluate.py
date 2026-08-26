"""Evaluation is a marshalling layer, and these tests hold it to that.

Nothing here needs Docker: ``run_evaluation`` takes the process runner as a
parameter so the argv it builds can be asserted on directly, and every other
test works from fixture report files on disk — which is also the property the
harness needs in the field, where analysis happens long after the containers
that produced the reports are gone.
"""

import json
import shutil
from pathlib import Path

import pytest

import evaluate

ARM = "rtdd"
IDS = ["astropy__astropy-12907", "django__django-11039", "sympy__sympy-20049"]

ROWS = {
    "astropy__astropy-12907": {
        "PASS_TO_PASS": json.dumps(["p1", "p2", "p3", "p4"]),
        "FAIL_TO_PASS": json.dumps(["f1"]),
    },
    "django__django-11039": {
        "PASS_TO_PASS": json.dumps(["q1", "q2"]),
        "FAIL_TO_PASS": json.dumps(["g1", "g2"]),
    },
    "sympy__sympy-20049": {
        "PASS_TO_PASS": json.dumps(["r1", "r2", "r3"]),
        "FAIL_TO_PASS": json.dumps(["h1"]),
    },
}


def make_repo(tmp_path: Path, raw: dict[str, dict]) -> Path:
    """A repo root carrying a frozen sample and one arm's raw agent records."""
    (tmp_path / "bench" / "swebench").mkdir(parents=True)
    (tmp_path / "bench" / "swebench" / "instances.txt").write_text(
        "\n".join(IDS) + "\n", encoding="utf-8"
    )
    raw_dir = tmp_path / "bench" / "results" / "swebench" / "raw" / ARM
    raw_dir.mkdir(parents=True)
    for instance_id, record in raw.items():
        (raw_dir / f"{instance_id}.json").write_text(
            json.dumps({"instance_id": instance_id, "arm": ARM, **record}),
            encoding="utf-8",
        )
    return tmp_path


def report_entry(instance_id: str, p2p_fail: list[str], f2p_pass: list[str]) -> dict:
    p2p = json.loads(ROWS[instance_id]["PASS_TO_PASS"])
    f2p = json.loads(ROWS[instance_id]["FAIL_TO_PASS"])
    return {
        instance_id: {
            "patch_is_None": False,
            "patch_exists": True,
            "patch_successfully_applied": True,
            "resolved": not p2p_fail and len(f2p_pass) == len(f2p),
            "tests_status": {
                "PASS_TO_PASS": {
                    "success": [t for t in p2p if t not in p2p_fail],
                    "failure": list(p2p_fail),
                },
                "FAIL_TO_PASS": {
                    "success": list(f2p_pass),
                    "failure": [t for t in f2p if t not in f2p_pass],
                },
            },
        }
    }


def write_harness_report(repo_root: Path, instance_id: str, report: dict) -> None:
    """Drop a report where the official harness leaves one for this run id."""
    log_dir = evaluate.harness_report_dir(repo_root, ARM) / f"rtdd-{ARM}" / instance_id
    log_dir.mkdir(parents=True, exist_ok=True)
    (log_dir / "report.json").write_text(json.dumps(report), encoding="utf-8")


class FakeRunner:
    """Stands in for subprocess.run; records argv instead of launching Docker."""

    def __init__(self, returncode: int = 0):
        self.returncode = returncode
        self.calls: list[list[str]] = []
        self.kwargs: dict = {}

    def __call__(self, argv, **kwargs):
        self.calls.append(list(argv))
        self.kwargs = kwargs
        return type("Completed", (), {"returncode": self.returncode, "args": argv})()


def test_predictions_carry_every_frozen_instance_including_the_empty_patches(tmp_path):
    # An arm that produced nothing for an instance must still appear in the
    # predictions file: dropping the line would shrink the denominator the whole
    # benchmark divides by. The third id has no raw record at all.
    repo = make_repo(
        tmp_path,
        {IDS[0]: {"patch": "diff --git a/x b/x\n"}, IDS[1]: {"patch": ""}},
    )

    out = evaluate.write_predictions(repo, ARM)

    lines = [json.loads(l) for l in out.read_text(encoding="utf-8").splitlines() if l]
    assert [line["instance_id"] for line in lines] == IDS
    assert [line["model_patch"] for line in lines] == ["diff --git a/x b/x\n", "", ""]


def test_predictions_use_the_official_schema_and_an_arm_scoped_model_name(tmp_path):
    repo = make_repo(tmp_path, {IDS[0]: {"patch": "p"}})

    out = evaluate.write_predictions(repo, ARM)

    line = json.loads(out.read_text(encoding="utf-8").splitlines()[0])
    assert set(line) == {"instance_id", "model_name_or_path", "model_patch"}
    assert line["model_name_or_path"] == f"rtdd-{ARM}"
    assert out == repo / "bench" / "results" / "swebench" / "predictions" / f"{ARM}.jsonl"


def test_run_evaluation_invokes_the_official_harness_with_a_stable_arm_run_id(tmp_path):
    repo = make_repo(tmp_path, {IDS[0]: {"patch": "p"}})
    runner = FakeRunner()

    reports = evaluate.run_evaluation(repo, ARM, workers=4, runner=runner)

    argv = runner.calls[0]
    assert argv[1:3] == ["-m", "swebench.harness.run_evaluation"]
    assert evaluate.run_id_for(ARM) == f"{evaluate.RUN_ID_PREFIX}-{ARM}"
    assert argv[argv.index("--run_id") + 1] == evaluate.run_id_for(ARM)
    assert argv[argv.index("--max_workers") + 1] == "4"
    assert argv[argv.index("--predictions_path") + 1].endswith(f"{ARM}.jsonl")
    assert reports == evaluate.harness_report_dir(repo, ARM)


def test_run_evaluation_refuses_to_report_success_when_the_harness_fails(tmp_path):
    repo = make_repo(tmp_path, {IDS[0]: {"patch": "p"}})

    with pytest.raises(RuntimeError, match=ARM):
        evaluate.run_evaluation(repo, ARM, workers=1, runner=FakeRunner(returncode=1))


def test_collect_reads_regressions_through_metrics(tmp_path):
    repo = make_repo(tmp_path, {i: {"patch": "p"} for i in IDS})
    for instance_id in IDS:
        write_harness_report(repo, instance_id, report_entry(instance_id, [], ["f1"]))
    write_harness_report(repo, IDS[0], report_entry(IDS[0], ["p2"], ["f1"]))

    results = {r.instance_id: r for r in evaluate.collect(repo, ARM, rows=ROWS)}

    assert results[IDS[0]].p2p_failed == ("p2",)
    assert results[IDS[0]].p2p_total == 4
    assert results[IDS[0]].evaluated is True
    assert results[IDS[0]].arm == ARM


def test_an_instance_the_harness_never_reported_keeps_its_dataset_denominator(tmp_path):
    # A crashed evaluation must not shrink the denominator by producing no
    # output for an instance.
    repo = make_repo(tmp_path, {i: {"patch": "p"} for i in IDS})
    write_harness_report(repo, IDS[0], report_entry(IDS[0], [], ["f1"]))

    results = {r.instance_id: r for r in evaluate.collect(repo, ARM, rows=ROWS)}

    missing = results[IDS[2]]
    assert missing.evaluated is False
    assert missing.p2p_failed == ()
    assert missing.p2p_total == 3
    assert missing.f2p_total == 1


def test_collect_covers_the_whole_frozen_sample_even_with_no_raw_records(tmp_path):
    repo = make_repo(tmp_path, {})

    results = evaluate.collect(repo, ARM, rows=ROWS)

    assert [r.instance_id for r in results] == IDS
    assert sum(r.p2p_total for r in results) == 9


def test_harness_errors_are_counted_and_an_empty_patch_is_not_one(tmp_path):
    repo = make_repo(
        tmp_path,
        {IDS[0]: {"patch": "p"}, IDS[1]: {"patch": ""}, IDS[2]: {"patch": "p"}},
    )
    write_harness_report(repo, IDS[0], report_entry(IDS[0], [], ["f1"]))

    results = evaluate.collect(repo, ARM, rows=ROWS)

    # IDS[2] had a patch and no report — the harness lost it, and that is an
    # error to surface. IDS[1] produced nothing, which is the arm's own doing.
    assert evaluate.harness_errors(results) == (IDS[2],)


def test_a_patch_the_harness_could_not_apply_is_not_counted_as_applied(tmp_path):
    repo = make_repo(tmp_path, {IDS[0]: {"patch": "p"}})
    report = report_entry(IDS[0], [], [])
    report[IDS[0]]["patch_successfully_applied"] = False
    write_harness_report(repo, IDS[0], report)

    results = evaluate.collect(repo, ARM, rows=ROWS)
    by_id = {r.instance_id: r for r in results}

    assert by_id[IDS[0]].patch_applied is False
    assert by_id[IDS[0]].evaluated is False
    assert evaluate.harness_errors(results) == ()


def test_reports_are_saved_under_the_arm_and_reread_without_the_harness_logs(tmp_path):
    repo = make_repo(tmp_path, {i: {"patch": "p"} for i in IDS})
    for instance_id in IDS:
        write_harness_report(repo, instance_id, report_entry(instance_id, ["p2"], []))

    first = evaluate.collect(repo, ARM, rows=ROWS)
    saved = repo / "bench" / "results" / "swebench" / "reports" / ARM
    assert sorted(p.stem for p in saved.glob("*.json")) == sorted(IDS)

    # Analysis must survive the logs being deleted: Docker is not re-run to
    # answer a question about numbers that were already measured.
    shutil.rmtree(evaluate.harness_report_dir(repo, ARM))
    again = evaluate.collect(repo, ARM, rows=ROWS)

    assert again == first
