"""The acceptance gate for M4 Tasks 1-12: complete, tested, and correctly refusing.

Every other test file in this directory checks one module. This one checks the
*harness*, and it checks it the way a reviewer would: by running the shipped
entry points as subprocesses against the repository as committed, and by driving
one fixture run end to end from raw records to the rendered table.

Four claims are made here and nowhere else:

  1. **The refusal is real.** ``preflight.py`` and ``run_arm.py <arm>`` refuse on
     this repository, from the command line, naming the missing kill criterion —
     and the arm runners refuse before any model call, not after one.
  2. **The refusal is informative in every direction.** One test walks the gate
     through all five ways a pre-registration can be wrong — missing, blank,
     non-numeric, unsigned, untagged — and asserts five *distinct* messages.
  3. **The structural guarantee is load-bearing.** Two fixtures inject procedural
     prose into the RTDD arm, one inside the context block and one after it, and
     the dry run fails on each. The fixtures ship as evidence: they are the
     failing runs the guarantee exists to produce.
  4. **The regression numbers are mechanical.** A fixture pass goes raw records →
     predictions → collect → analyze → ``tables.md`` with no benchmark behind it,
     and the published figures are traced back to ``PASS_TO_PASS.failure`` entries
     over dataset-derived denominators.

Nothing here needs a model, a network, or Docker. That is the point: the harness
is finished when it can be checked without being run.
"""

from __future__ import annotations

import hashlib
import json
import os
import re
import subprocess
import sys
from pathlib import Path

import pytest
import yaml

import analyze
import evaluate
import prompts
import report
import run_arm as driver
from agent import AgentConfig
from preflight import instance_list_sha256, preflight
from prereg import PreregError
from providers.tdad import TDAD_PIN

HERE = Path(__file__).resolve().parent
SWEBENCH = HERE.parent
REPO_ROOT = SWEBENCH.parents[1]
PREREG_PATH = REPO_ROOT / "bench" / "PREREGISTRATION.md"
INSTANCES = SWEBENCH / "instances.txt"
FIXTURES = HERE / "fixtures"
HARNESS_DOC = REPO_ROOT / "docs" / "bench" / "swebench-harness.md"

#: The environment the "no network" claim is made under. A dead proxy for every
#: scheme and the offline switches for the dataset library, so a process that
#: reached for the network would fail loudly rather than quietly succeeding on a
#: machine that happens to be online.
OFFLINE = {
    "http_proxy": "http://127.0.0.1:1",
    "https_proxy": "http://127.0.0.1:1",
    "all_proxy": "http://127.0.0.1:1",
    "no_proxy": "",
    "HF_HUB_OFFLINE": "1",
    "HF_DATASETS_OFFLINE": "1",
}


def offline_env() -> dict[str, str]:
    return {**os.environ, **OFFLINE}


def run_script(name: str, *args: str, env: dict[str, str] | None = None):
    """Run one of the shipped entry points exactly as a reviewer would."""
    return subprocess.run(
        [sys.executable, str(SWEBENCH / name), *args],
        capture_output=True,
        text=True,
        cwd=str(SWEBENCH),
        env=env,
    )


def front_matter(path: Path) -> dict:
    text = path.read_text(encoding="utf-8")
    match = re.match(r"\A---\n(.*?)\n---\n", text, re.S)
    assert match is not None, f"{path} has no YAML front-matter"
    return yaml.safe_load(match.group(1))


# --------------------------------------------------------------------------
# 1. The refusal, end to end, on the repository as committed
# --------------------------------------------------------------------------


def test_the_shipped_preflight_cli_refuses_naming_the_missing_kill_criterion():
    proc = run_script("preflight.py", env=offline_env())
    assert proc.returncode != 0, proc.stdout
    assert proc.returncode == 3, proc.stderr
    assert "PREFLIGHT REFUSED" in proc.stderr
    assert "stratified_recall_floor" in proc.stderr
    # A refusal, not a crash: a traceback would mean the gate fell over rather
    # than declining, and the two are not the same verdict.
    assert "Traceback" not in proc.stderr


@pytest.mark.parametrize("arm", prompts.ARMS)
def test_every_arm_refuses_from_the_command_line_before_any_model_call(arm):
    raw = evaluate.results_dir(REPO_ROOT) / "raw" / arm
    before = sorted(raw.glob("*.json")) if raw.exists() else []

    # A base URL nothing is listening on: had the runner reached the model, the
    # failure would be a connection error, not exit 3.
    proc = run_script(
        "run_arm.py", arm, "--base-url", "http://127.0.0.1:1/v1", env=offline_env()
    )

    assert proc.returncode == 3, f"{proc.returncode}: {proc.stderr}"
    assert "PREFLIGHT REFUSED" in proc.stderr
    assert "stratified_recall_floor" in proc.stderr
    assert "Connection" not in proc.stderr and "connect" not in proc.stderr.lower()

    after = sorted(raw.glob("*.json")) if raw.exists() else []
    assert after == before, f"{arm} wrote records despite refusing"
    cost = driver.cost_path(REPO_ROOT, arm)
    assert not cost.exists(), f"{arm} charged a budget despite refusing"


# --------------------------------------------------------------------------
# 2. All five refusal cases, in one walk
# --------------------------------------------------------------------------

PREREG_TEMPLATE = """---
status: {status}
sample_size: 2
sample_seed: 20260826
instance_list_sha256: {sha}
model: Qwen3-Coder-30B-A3B-Instruct
arms: [vanilla, tdd, tdad, rtdd, rtdd_tdd]
{floor_line}
vanilla_equivalence_k: 2
signed_by: {signed_by}
signed_at: {signed_at}
---

body
"""

CASE_IDS = ["astropy__astropy-12907", "django__django-11039"]


def git(repo: Path, *args: str) -> None:
    subprocess.run(["git", "-C", str(repo), *args], check=True, capture_output=True)


def build_repo(
    root: Path,
    *,
    floor_line: str = "stratified_recall_floor: 0.95",
    status: str = "SIGNED",
    signed_by: str = "VocanicZ",
    signed_at: str = "2026-08-27",
    tag: bool = True,
    ids: list[str] | None = None,
) -> Path:
    """A repository whose pre-registration says exactly what a case needs it to."""
    ids = ids if ids is not None else CASE_IDS
    (root / "bench" / "swebench").mkdir(parents=True, exist_ok=True)
    text = "\n".join(ids) + "\n"
    (root / "bench" / "swebench" / "instances.txt").write_text(text, encoding="utf-8")
    sha = hashlib.sha256(text.encode("utf-8")).hexdigest()
    (root / "bench" / "PREREGISTRATION.md").write_text(
        PREREG_TEMPLATE.format(
            status=status,
            sha=sha,
            floor_line=floor_line,
            signed_by=signed_by,
            signed_at=signed_at,
        ).replace("sample_size: 2", f"sample_size: {len(ids)}"),
        encoding="utf-8",
    )
    subprocess.run(["git", "init", "-q", str(root)], check=True)
    git(root, "config", "user.email", "t@t")
    git(root, "config", "user.name", "t")
    git(root, "add", "-A")
    git(root, "commit", "-qm", "prereg")
    if tag:
        git(root, "tag", "prereg-m4")
    return root


def refusal(root: Path) -> str:
    with pytest.raises(PreregError) as excinfo:
        preflight(root)
    return str(excinfo.value)


def test_the_five_refusal_cases_each_get_their_own_informative_message(tmp_path):
    """Missing, blank, non-numeric, unsigned, untagged — five wrongs, five messages.

    Held in one test on purpose. The claim is not that each case refuses, which
    the unit tests already make; it is that a human reading the refusal can tell
    the cases apart, and that is a claim about all five at once.
    """
    cases = {
        # The field is absent from the front-matter entirely.
        "missing": dict(floor_line="# stratified_recall_floor is not written yet"),
        # The key is there with nothing after the colon — the state this repo ships.
        "blank": dict(floor_line="stratified_recall_floor:"),
        # A human wrote a word where a number belongs.
        "non-numeric": dict(floor_line="stratified_recall_floor: TBD"),
        # Complete, but nobody put their name to it.
        "unsigned": dict(status="UNSIGNED"),
        # Signed, complete — and not reachable from the tag that freezes it.
        "untagged": dict(tag=False),
    }

    messages: dict[str, str] = {}
    for name, kwargs in cases.items():
        messages[name] = refusal(build_repo(tmp_path / name, **kwargs))

    assert len(set(messages.values())) == 5, messages

    assert "stratified_recall_floor" in messages["missing"]
    assert "missing or blank" in messages["missing"]

    assert "stratified_recall_floor" in messages["blank"]
    assert "missing or blank" in messages["blank"]

    assert "stratified_recall_floor" in messages["non-numeric"]
    assert "numeric" in messages["non-numeric"]
    assert "'TBD'" in messages["non-numeric"]

    assert "SIGNED" in messages["unsigned"]
    assert "a human must sign this" in messages["unsigned"]

    assert "prereg-m4" in messages["untagged"]
    assert "has not been frozen" in messages["untagged"]
    assert "fatal:" not in messages["untagged"], "a raw git error is not a refusal a human can act on"


def test_a_signed_tagged_and_matching_pre_registration_is_the_one_state_that_passes(tmp_path):
    """The gate is a gate, not a wall: the correct state opens it."""
    root = build_repo(tmp_path / "good")
    gate = preflight(root)
    assert gate.fields["stratified_recall_floor"] == 0.95
    assert gate.fields["status"] == "SIGNED"


# --------------------------------------------------------------------------
# 3. Five arms, dry-run, no model, no network
# --------------------------------------------------------------------------


def test_the_dry_run_cli_builds_five_arms_over_the_frozen_list_with_no_network():
    proc = run_script("run_arm.py", "--dry-run", env=offline_env())
    assert proc.returncode == 0, proc.stderr

    ids = driver.instance_ids(REPO_ROOT)
    assert len(ids) == 100
    for instance_id in ids:
        assert instance_id in proc.stdout
    for arm in prompts.ARMS:
        assert arm in proc.stdout
    assert f"dry run: {len(ids)} instances x {len(prompts.ARMS)} arms" in proc.stdout
    assert "0 findings" in proc.stdout


def test_the_dry_run_needs_no_signed_pre_registration_so_ci_can_run_it_here():
    """CI runs this on a deliberately unsigned repository, so it must not gate on one."""
    assert front_matter(PREREG_PATH)["status"] == "UNSIGNED"
    assert run_script("run_arm.py", "--dry-run", env=offline_env()).returncode == 0


# --------------------------------------------------------------------------
# 4. The structural guarantee, proven by deliberately failing fixtures
# --------------------------------------------------------------------------


def fixture(name: str) -> str:
    path = FIXTURES / name
    assert path.exists(), f"the failing fixture {path} must ship as evidence"
    return path.read_text(encoding="utf-8")


def fixture_repo(tmp_path: Path) -> Path:
    return build_repo(tmp_path / "fixture-repo")


def test_procedural_prose_inside_the_context_block_fails_the_imperative_lint(monkeypatch, tmp_path):
    """The fixture is the RTDD context with a TDD protocol smuggled into it."""
    poisoned = fixture("procedural_rtdd_context.md")
    assert prompts.lint_context(prompts.RTDD_CONTEXT) == []

    findings = prompts.lint_context(poisoned)
    assert findings, "the fixture is supposed to be procedural"
    assert any("test-driven" in f or "workflow" in f for f in findings)

    monkeypatch.setitem(prompts._CONTEXT, "rtdd", poisoned)
    driver_findings = driver.dry_run(fixture_repo(tmp_path), out=lambda *_: None)
    assert driver_findings, "the dry run failed to notice the procedure"
    assert all(f.startswith("rtdd") for f in _armless(driver_findings))


def test_procedural_prose_after_the_context_block_fails_the_byte_equality(monkeypatch, tmp_path):
    """Prose the lint cannot see, because it is outside the block, is caught by the bytes."""
    trailing = fixture("procedural_rtdd_trailing.md")
    real_build = prompts.build

    def poisoned_build(arm: str) -> str:
        prompt = real_build(arm)
        return prompt + trailing if arm == "rtdd" else prompt

    monkeypatch.setattr(prompts, "build", poisoned_build)
    findings = driver.dry_run(fixture_repo(tmp_path), out=lambda *_: None)
    assert findings, "the dry run failed to notice the appended procedure"
    assert any("byte for byte" in f for f in findings)
    assert all(f.startswith("rtdd") for f in _armless(findings))


def test_the_failing_fixtures_make_the_shipped_dry_run_cli_exit_non_zero(monkeypatch, tmp_path):
    """A poisoned arm is a red CI run, not a warning in a log nobody reads."""
    monkeypatch.setitem(prompts._CONTEXT, "rtdd", fixture("procedural_rtdd_context.md"))
    root = fixture_repo(tmp_path)
    assert driver.main(["--dry-run", "--repo-root", str(root)]) == 5


def _armless(findings: list[str]) -> list[str]:
    """Strip the ``<instance_id>: `` prefix the dry run adds, leaving ``<arm>: ...``."""
    return [f.split(": ", 1)[1] for f in findings]


# --------------------------------------------------------------------------
# 5. Regression is mechanical: raw records -> predictions -> collect ->
#    analyze -> tables.md, over fixture reports and no benchmark run
# --------------------------------------------------------------------------

#: Dataset rows for the fixture pass. The `PASS_TO_PASS` cardinality here is the
#: only denominator any published rate may divide by.
FIXTURE_ROWS = {
    "astropy__astropy-12907": {
        "PASS_TO_PASS": json.dumps(["p1", "p2", "p3", "p4"]),
        "FAIL_TO_PASS": json.dumps(["f1"]),
    },
    "django__django-11039": {
        "PASS_TO_PASS": json.dumps(["q1", "q2", "q3"]),
        "FAIL_TO_PASS": json.dumps(["g1"]),
    },
}
FIXTURE_IDS = list(FIXTURE_ROWS)

#: What the harness reported failing, per arm. `rtdd` regresses nothing; `tdd`
#: regresses one test in one instance, which is the number the table must show.
FIXTURE_FAILURES: dict[str, dict[str, list[str]]] = {
    "vanilla": {"astropy__astropy-12907": ["p2"]},
    "tdd": {"astropy__astropy-12907": ["p1", "p2"], "django__django-11039": ["q1"]},
    "tdad": {"django__django-11039": ["q3"]},
    "rtdd": {},
    "rtdd_tdd": {"astropy__astropy-12907": ["p4"]},
}

#: Deliberately fewer P2P ids than the dataset carries, so a report-derived
#: denominator would produce a visibly different rate than a dataset-derived one.
TRUNCATED_REPORT_P2P = 2


def write_fixture_run(root: Path) -> None:
    """One raw record and one official-shaped harness report per (arm, instance)."""
    results = evaluate.results_dir(root)
    for arm, failures in FIXTURE_FAILURES.items():
        raw = results / "raw" / arm
        raw.mkdir(parents=True, exist_ok=True)
        logs = evaluate.harness_report_dir(root, arm) / f"rtdd-{arm}"
        for instance_id in FIXTURE_IDS:
            (raw / f"{instance_id}.json").write_text(
                json.dumps(
                    {
                        "instance_id": instance_id,
                        "arm": arm,
                        "patch": f"diff --git a/{instance_id} b/{instance_id}\n",
                        "seed_status": "seeded",
                        "stop_reason": "finish",
                        "prompt_tokens": 10,
                        "completion_tokens": 5,
                    }
                ),
                encoding="utf-8",
            )
            failed = failures.get(instance_id, [])
            p2p = json.loads(FIXTURE_ROWS[instance_id]["PASS_TO_PASS"])
            reported = [t for t in p2p if t not in failed][:TRUNCATED_REPORT_P2P]
            f2p = json.loads(FIXTURE_ROWS[instance_id]["FAIL_TO_PASS"])
            dest = logs / instance_id
            dest.mkdir(parents=True, exist_ok=True)
            (dest / "report.json").write_text(
                json.dumps(
                    {
                        instance_id: {
                            "patch_successfully_applied": True,
                            "resolved": not failed,
                            "tests_status": {
                                "PASS_TO_PASS": {"success": reported, "failure": failed},
                                "FAIL_TO_PASS": {"success": f2p, "failure": []},
                            },
                        }
                    }
                ),
                encoding="utf-8",
            )


def write_m3_recall(root: Path, value: float = 0.97) -> None:
    """Axis 2's published recall, which the kill criterion reads."""
    out = root / "bench" / "results" / "httpie"
    out.mkdir(parents=True, exist_ok=True)
    (out / "summary.json").write_text(
        json.dumps(
            {
                "repo_id": "httpie",
                "strategies": {
                    "rtdd": {
                        "strata": {
                            "1": {
                                "n": 20,
                                "change_level_recall": {
                                    "num": 20 * value,
                                    "den": 20,
                                    "value": value,
                                },
                            }
                        }
                    }
                },
            }
        ),
        encoding="utf-8",
    )


@pytest.fixture
def fixture_pass(tmp_path):
    """The whole pipeline, run once over the fixture reports; returns its artefacts."""
    root = build_repo(tmp_path / "run", ids=FIXTURE_IDS)
    write_fixture_run(root)
    write_m3_recall(root)

    predictions = {
        arm: evaluate.write_predictions(root, arm) for arm in FIXTURE_FAILURES
    }
    collected = {
        arm: evaluate.collect(root, arm, FIXTURE_ROWS) for arm in FIXTURE_FAILURES
    }
    summary = analyze.analyze(root, rows=FIXTURE_ROWS)
    tables = evaluate.results_dir(root) / "tables.md"
    tables.write_text(report.render(summary), encoding="utf-8")
    return {
        "root": root,
        "predictions": predictions,
        "collected": collected,
        "summary": summary,
        "tables": tables.read_text(encoding="utf-8"),
    }


def test_the_pipeline_runs_end_to_end_from_raw_records_to_a_rendered_table(fixture_pass):
    for arm, path in fixture_pass["predictions"].items():
        lines = [json.loads(ln) for ln in path.read_text(encoding="utf-8").splitlines()]
        assert [ln["instance_id"] for ln in lines] == FIXTURE_IDS
        assert all(ln["model_name_or_path"] == f"rtdd-{arm}" for ln in lines)
    assert (evaluate.results_dir(fixture_pass["root"]) / "summary.json").exists()
    assert fixture_pass["tables"].startswith("|")


def test_every_published_regression_traces_to_pass_to_pass_failures(fixture_pass):
    """The numerator is the failure list, and nothing else in the report."""
    for arm, failures in FIXTURE_FAILURES.items():
        expected = sum(len(v) for v in failures.values())
        assert fixture_pass["summary"]["arms"][arm]["p2p_failed"] == expected
        assert sum(
            len(r.p2p_failed) for r in fixture_pass["collected"][arm]
        ) == expected


def test_the_denominator_comes_from_the_dataset_not_from_the_report(fixture_pass):
    """A report that lost tests cannot shrink the number every rate divides by."""
    dataset_total = sum(
        len(json.loads(row["PASS_TO_PASS"])) for row in FIXTURE_ROWS.values()
    )
    reported_total = len(FIXTURE_IDS) * TRUNCATED_REPORT_P2P
    assert reported_total < dataset_total, "the fixture must truncate to prove the point"

    for arm, failures in FIXTURE_FAILURES.items():
        summary = fixture_pass["summary"]["arms"][arm]
        assert summary["p2p_total"] == dataset_total
        failed = sum(len(v) for v in failures.values())
        assert summary["test_level"] == pytest.approx(failed / dataset_total)


def test_the_rendered_table_carries_the_traced_numbers(fixture_pass):
    """What the reader sees is the number that was traced, not a second computation."""
    rows = [
        line
        for line in fixture_pass["tables"].splitlines()
        if line.startswith("|") and "---" not in line
    ]
    dataset_total = sum(
        len(json.loads(row["PASS_TO_PASS"])) for row in FIXTURE_ROWS.values()
    )
    for arm, failures in FIXTURE_FAILURES.items():
        failed = sum(len(v) for v in failures.values())
        label = report.ARM_LABEL[arm]
        detail = [r for r in rows if r.split("|")[1].strip() == label]
        assert any(
            f"{failed} / {dataset_total}" in r for r in detail
        ), f"{arm}: no row traces {failed}/{dataset_total}"
        rate = f"{failed / dataset_total * 100:.2f}%"
        assert any(rate in r for r in detail), f"{arm}: rendered no row at {rate}"


def test_the_generated_table_carries_a_resolution_rate_in_every_row(fixture_pass):
    text = fixture_pass["tables"]
    report.assert_resolution_beside_regression(text)

    header, _separator, *rows = _first_table(text)
    assert report.RESOLUTION_COLUMN in header
    column = header.index(report.RESOLUTION_COLUMN)

    labels = {report.ARM_LABEL[arm] for arm in FIXTURE_FAILURES}
    seen = set()
    for values in rows:
        assert values[0] in labels, values
        assert re.fullmatch(r"\d+\.\d%", values[column]), f"{values[0]}: {values[column]!r}"
        seen.add(values[0])
    assert seen == labels


def _first_table(text: str) -> list[list[str]]:
    """The arm table, as cell lists — the one a reader meets first."""
    rows: list[list[str]] = []
    for line in text.splitlines():
        if line.startswith("|"):
            rows.append([c.strip() for c in line.strip().strip("|").split("|")])
        elif rows:
            break
    return rows


# --------------------------------------------------------------------------
# 6. The pre-registration is complete except the human's number
# --------------------------------------------------------------------------


def test_the_shipped_pre_registration_is_complete_except_the_humans_number():
    fields = front_matter(PREREG_PATH)

    assert fields["status"] == "UNSIGNED"
    assert "stratified_recall_floor" in fields
    assert fields["stratified_recall_floor"] is None
    assert fields["signed_by"] is None
    assert fields["signed_at"] is None

    assert fields["sample_size"] == 100
    assert fields["sample_seed"] == 20260826
    assert fields["model"] == AgentConfig.model
    assert fields["arms"] == list(prompts.ARMS)
    assert fields["instance_list_sha256"] == instance_list_sha256(INSTANCES)


def test_the_pre_registration_pins_both_reference_implementations():
    """Neither half of the comparison may move between signing and publication."""
    fields = front_matter(PREREG_PATH)
    assert fields["tdad_commit"] == TDAD_PIN

    body = PREREG_PATH.read_text(encoding="utf-8")
    # RTDD's own pin is the tag itself: `assert_tagged` proves the signed file is
    # reachable from `prereg-m4`, and `report.write_config` records the commit.
    assert "prereg-m4" in body
    assert "rtdd_commit" in body


def test_the_frozen_instance_list_is_the_one_the_pre_registration_describes():
    ids = [ln.strip() for ln in INSTANCES.read_text(encoding="utf-8").splitlines() if ln.strip()]
    assert len(ids) == front_matter(PREREG_PATH)["sample_size"]
    assert ids == sorted(ids)
    assert len(set(ids)) == len(ids)


# --------------------------------------------------------------------------
# 7. CI is the enforcement, and docs say what a reviewer should see refuse
# --------------------------------------------------------------------------

CI_SCRIPT = REPO_ROOT / "scripts" / "ci-prereg.sh"
CI_WORKFLOW = REPO_ROOT / ".github" / "workflows" / "ci.yml"


def test_one_ci_run_covers_the_suite_the_refusal_the_dry_run_and_the_lint():
    text = CI_SCRIPT.read_text(encoding="utf-8")
    assert "uv run pytest -q" in text                       # the bench test suite
    assert "uv run python preflight.py" in text             # the prereg refusal
    assert "run_arm.py --dry-run" in text                   # the five-arm dry run
    assert "tests/test_prompts.py" in text                  # the non-procedural assertions
    assert "tests/test_acceptance.py" in text               # this file
    assert "exit 3" in text, "CI must assert the refusal's exit code, not merely run it"

    workflow = CI_WORKFLOW.read_text(encoding="utf-8")
    assert "scripts/ci-prereg.sh" in workflow


def test_the_docs_say_how_to_reproduce_the_harness_and_what_should_refuse():
    assert HARNESS_DOC.exists(), f"{HARNESS_DOC} must ship"
    text = HARNESS_DOC.read_text(encoding="utf-8")

    for command in ("uv sync", "uv run pytest", "preflight.py", "run_arm.py --dry-run"):
        assert command in text, f"the doc does not say how to run {command}"
    assert "exit 3" in text or "exits 3" in text
    assert "stratified_recall_floor" in text


def test_the_docs_scope_the_human_in_the_loop_tasks_out_of_this_prd():
    """Tasks 2, 13 and 14 are HITL. A reader must find that written down, with the unblock."""
    text = HARNESS_DOC.read_text(encoding="utf-8")
    for task in range(1, 13):
        assert re.search(rf"\bTask {task}\b", text), f"Task {task} is not accounted for"
    for task in (2, 13, 14):
        assert re.search(rf"\bTask {task}\b", text), f"Task {task} is not scoped out"
    assert "out of scope" in text.lower()
