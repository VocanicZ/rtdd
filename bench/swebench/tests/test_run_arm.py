"""The per-arm driver: gated, ordered, resumable, budgeted, and model-free on demand.

Every test here stubs the model, the checkout and the providers. That is the
point: the driver's job is not to be clever with any of them, it is to spend no
token before the gate says so, to walk the frozen list in file order, to leave
exactly one record per instance whatever happens to that instance, and to stop
dead at a ceiling with the cost written down.
"""

from __future__ import annotations

import inspect
import json
from pathlib import Path

import pytest

import agent
import prompts
import run_arm as driver
import sample
import workspace
from budget import Budget, BudgetExceeded
from prereg import PreregError

IDS = ["astropy__astropy-7166", "django__django-10554", "sympy__sympy-20049"]


def rows_for(ids: list[str]) -> dict[str, dict]:
    return {
        iid: {
            "instance_id": iid,
            "repo": iid.split("__")[0] + "/" + iid.split("__")[1].rsplit("-", 1)[0],
            "base_commit": "0" * 40,
            "problem_statement": f"{iid} is broken and should not be.",
        }
        for iid in ids
    }


class Recorder:
    """What the driver actually did, in the order it did it."""

    def __init__(self) -> None:
        self.prepared: list[str] = []
        self.ran: list[str] = []
        self.prompts: dict[str, str] = {}


def stub(monkeypatch, tmp_path, ids=IDS, *, result=None, prepare_error=None, on_run=None):
    rec = Recorder()
    rows = rows_for(ids)

    def fake_load_rows() -> dict[str, dict]:
        return rows

    def fake_prepare(cache: Path, work: Path, row: dict):
        rec.prepared.append(row["instance_id"])
        if prepare_error is not None and row["instance_id"] == prepare_error:
            raise RuntimeError("clone exploded")
        root = work / row["instance_id"]
        root.mkdir(parents=True, exist_ok=True)
        return workspace.Workspace(
            root=root,
            instance_id=row["instance_id"],
            repo=row["repo"],
            base_commit=row["base_commit"],
        )

    def fake_context_tools(arm: str, ws, cache: Path, row: dict):
        return [], {"seed_status": "seeded"} if arm.startswith("rtdd") else {}

    def fake_base_tools(ws, **kwargs):
        return []

    def fake_run_agent(ws, system_prompt, tools, cfg, client=None):
        rec.ran.append(ws.instance_id)
        rec.prompts[ws.instance_id] = system_prompt
        if on_run is not None:
            on_run(ws.instance_id)
        return result or agent.AgentResult(
            patch="diff --git a/x b/x\n",
            turns=3,
            prompt_tokens=1000,
            completion_tokens=100,
            stop_reason="finish",
        )

    monkeypatch.setattr(sample, "load_rows", fake_load_rows)
    monkeypatch.setattr(workspace, "prepare", fake_prepare)
    monkeypatch.setattr(driver, "context_tools", fake_context_tools)
    monkeypatch.setattr(driver.toolmod, "base_tools", fake_base_tools)
    monkeypatch.setattr(agent, "run_agent", fake_run_agent)
    monkeypatch.setattr(driver, "WORK_ROOT", tmp_path / "work")
    monkeypatch.setattr(driver, "CACHE_ROOT", tmp_path / "cache")
    return rec


def records(root: Path, arm: str) -> dict[str, dict]:
    raw = root / "bench" / "results" / "swebench" / "raw" / arm
    return {p.stem: json.loads(p.read_text(encoding="utf-8")) for p in sorted(raw.glob("*.json"))}


# --- the gate ------------------------------------------------------------


def test_an_unsigned_preregistration_is_refused_before_a_single_token(monkeypatch, tmp_path, make_repo):
    root = make_repo(IDS, status="UNSIGNED")
    rec = stub(monkeypatch, tmp_path)
    with pytest.raises(PreregError, match="not SIGNED"):
        driver.run_arm(root, "vanilla", agent.AgentConfig(), Budget())
    assert rec.ran == []
    assert rec.prepared == []


def test_an_untagged_preregistration_is_refused_before_a_single_token(monkeypatch, tmp_path, make_repo):
    root = make_repo(IDS, tag=False)
    rec = stub(monkeypatch, tmp_path)
    with pytest.raises(PreregError, match="prereg-m4"):
        driver.run_arm(root, "vanilla", agent.AgentConfig(), Budget())
    assert rec.ran == []


def test_an_arm_outside_the_preregistered_five_is_refused(monkeypatch, tmp_path, make_repo):
    root = make_repo(IDS)
    rec = stub(monkeypatch, tmp_path)
    with pytest.raises(PreregError, match="not pre-registered"):
        driver.run_arm(root, "rtdd_plus", agent.AgentConfig(), Budget())
    assert rec.ran == []


def test_main_exits_3_with_the_refusal_message_on_an_unsigned_repo(monkeypatch, tmp_path, make_repo, capsys):
    root = make_repo(IDS, status="UNSIGNED")
    rec = stub(monkeypatch, tmp_path)
    code = driver.main(["vanilla", "--repo-root", str(root)])
    assert code == 3
    assert "PREFLIGHT REFUSED" in capsys.readouterr().err
    assert rec.ran == []


# --- the frozen list -----------------------------------------------------


def test_instances_come_from_the_frozen_list_in_file_order(monkeypatch, tmp_path, make_repo):
    root = make_repo(IDS)
    rec = stub(monkeypatch, tmp_path)
    driver.run_arm(root, "vanilla", agent.AgentConfig(), Budget())
    assert rec.ran == IDS
    assert sorted(records(root, "vanilla")) == sorted(IDS)


def test_the_arm_never_re_draws_its_own_sample(monkeypatch, tmp_path, make_repo):
    root = make_repo(IDS)
    stub(monkeypatch, tmp_path)

    def explode(*args, **kwargs):
        raise AssertionError("the arm re-drew the sample instead of reading instances.txt")

    monkeypatch.setattr(sample, "draw", explode)
    monkeypatch.setattr(sample, "eligible_ids", explode)
    driver.run_arm(root, "vanilla", agent.AgentConfig(), Budget())


def test_the_record_carries_the_arm_the_model_and_the_provider_provenance(monkeypatch, tmp_path, make_repo):
    root = make_repo(IDS)
    stub(monkeypatch, tmp_path)
    cfg = agent.AgentConfig(model="Qwen3-Coder-30B-A3B-Instruct")
    driver.run_arm(root, "rtdd", cfg, Budget())
    record = records(root, "rtdd")[IDS[0]]
    assert record["arm"] == "rtdd"
    assert record["model"] == cfg.model
    assert record["seed_status"] == "seeded"
    assert record["stop_reason"] == "finish"
    assert record["error"] is None


def test_the_prompt_is_the_arm_prompt_with_the_instance_substituted(monkeypatch, tmp_path, make_repo):
    root = make_repo(IDS)
    rec = stub(monkeypatch, tmp_path)
    driver.run_arm(root, "rtdd", agent.AgentConfig(), Budget())
    prompt = rec.prompts[IDS[0]]
    assert "is broken and should not be." in prompt
    assert "{problem_statement}" not in prompt
    assert prompt == driver.assemble(
        "rtdd", tmp_path / "work" / IDS[0], rows_for(IDS)[IDS[0]]["problem_statement"]
    )


# --- resumability --------------------------------------------------------


def test_an_instance_with_a_record_is_skipped_so_a_crashed_run_resumes(monkeypatch, tmp_path, make_repo):
    root = make_repo(IDS)
    rec = stub(monkeypatch, tmp_path)
    driver.run_arm(root, "vanilla", agent.AgentConfig(), Budget())
    assert rec.ran == IDS

    kept = {iid: driver.raw_path(root, "vanilla", iid).read_text(encoding="utf-8") for iid in IDS}
    driver.raw_path(root, "vanilla", IDS[1]).unlink()

    rec2 = stub(monkeypatch, tmp_path)
    driver.run_arm(root, "vanilla", agent.AgentConfig(), Budget())
    assert rec2.ran == [IDS[1]]
    for iid in (IDS[0], IDS[2]):
        assert driver.raw_path(root, "vanilla", iid).read_text(encoding="utf-8") == kept[iid]
    assert sorted(records(root, "vanilla")) == sorted(IDS)


def test_a_second_run_with_every_record_present_spends_nothing_new(monkeypatch, tmp_path, make_repo):
    root = make_repo(IDS)
    stub(monkeypatch, tmp_path)
    driver.run_arm(root, "vanilla", agent.AgentConfig(), Budget())
    rec2 = stub(monkeypatch, tmp_path)
    b = Budget()
    driver.run_arm(root, "vanilla", agent.AgentConfig(), b)
    assert rec2.ran == []
    # Nothing new was charged, but the arm's recorded spend is still the first
    # run's: a resume that read zero would erase what the benchmark cost.
    assert (b.prompt_tokens, b.completion_tokens) == (3000, 300)


def test_arms_do_not_share_a_raw_directory(monkeypatch, tmp_path, make_repo):
    root = make_repo(IDS)
    stub(monkeypatch, tmp_path)
    driver.run_arm(root, "vanilla", agent.AgentConfig(), Budget())
    rec = stub(monkeypatch, tmp_path)
    driver.run_arm(root, "tdd", agent.AgentConfig(), Budget())
    assert rec.ran == IDS


# --- a failed instance is still an instance -------------------------------


def test_a_failed_instance_is_recorded_as_failed_not_skipped(monkeypatch, tmp_path, make_repo):
    root = make_repo(IDS)
    rec = stub(monkeypatch, tmp_path, prepare_error=IDS[1])
    driver.run_arm(root, "vanilla", agent.AgentConfig(), Budget())

    written = records(root, "vanilla")
    assert sorted(written) == sorted(IDS), "an errored instance vanished from the denominator"
    failed = written[IDS[1]]
    assert failed["patch"] == ""
    assert failed["stop_reason"] == "harness_error"
    assert "clone exploded" in failed["error"]
    assert "Traceback" in failed["traceback"]
    assert rec.ran == [IDS[0], IDS[2]], "a failure stopped the arm instead of being recorded"


def test_an_empty_patch_is_a_prediction_not_an_error(monkeypatch, tmp_path, make_repo):
    root = make_repo(IDS)
    stub(
        monkeypatch,
        tmp_path,
        result=agent.AgentResult(
            patch="",
            turns=40,
            prompt_tokens=10,
            completion_tokens=1,
            stop_reason="max_turns",
        ),
    )
    driver.run_arm(root, "vanilla", agent.AgentConfig(), Budget())
    record = records(root, "vanilla")[IDS[0]]
    assert record["patch"] == ""
    assert record["stop_reason"] == "max_turns"
    assert record["error"] is None


# --- the ceilings --------------------------------------------------------


def test_a_clean_run_writes_the_cost_file(monkeypatch, tmp_path, make_repo):
    root = make_repo(IDS)
    stub(monkeypatch, tmp_path)
    driver.run_arm(root, "vanilla", agent.AgentConfig(), Budget())
    cost = json.loads(driver.cost_path(root, "vanilla").read_text(encoding="utf-8"))
    assert cost["prompt_tokens"] == 3000
    assert cost["completion_tokens"] == 300
    assert cost["usd_per_m_prompt"] == Budget().usd_per_m_prompt
    assert cost["usd_per_m_completion"] == Budget().usd_per_m_completion


def test_crossing_the_spend_cap_stops_the_arm_and_writes_the_cost_file(monkeypatch, tmp_path, make_repo):
    root = make_repo(IDS)
    rec = stub(monkeypatch, tmp_path)
    b = Budget(max_usd=0.0001, usd_per_m_prompt=1.0, usd_per_m_completion=1.0)
    with pytest.raises(BudgetExceeded):
        driver.run_arm(root, "vanilla", agent.AgentConfig(), b)

    assert rec.ran == [IDS[0]], "the arm kept spending after the cap"
    cost = json.loads(driver.cost_path(root, "vanilla").read_text(encoding="utf-8"))
    assert cost["prompt_tokens"] == 1000
    assert cost["completion_tokens"] == 100
    assert cost["usd_per_m_prompt"] == 1.0
    assert cost["usd_estimated"] > 0


def test_the_instance_that_crossed_the_cap_keeps_its_record(monkeypatch, tmp_path, make_repo):
    # Otherwise the stop would silently cost the benchmark an instance that ran.
    root = make_repo(IDS)
    stub(monkeypatch, tmp_path)
    b = Budget(max_usd=0.0001, usd_per_m_prompt=1.0, usd_per_m_completion=1.0)
    with pytest.raises(BudgetExceeded):
        driver.run_arm(root, "vanilla", agent.AgentConfig(), b)
    assert list(records(root, "vanilla")) == [IDS[0]]


def test_main_exits_4_on_a_budget_stop(monkeypatch, tmp_path, make_repo, capsys):
    root = make_repo(IDS)
    stub(monkeypatch, tmp_path)
    code = driver.main(["vanilla", "--repo-root", str(root), "--max-usd", "0.0001"])
    assert code == 4
    assert "BUDGET STOP" in capsys.readouterr().err


def test_main_exits_0_on_a_clean_run(monkeypatch, tmp_path, make_repo):
    root = make_repo(IDS)
    stub(monkeypatch, tmp_path)
    assert driver.main(["vanilla", "--repo-root", str(root)]) == 0


# --- the model-free dry run ----------------------------------------------


def no_model(monkeypatch):
    """Anything that would reach the network or the model is an error in dry-run."""

    def explode(*args, **kwargs):
        raise AssertionError("the dry run reached the model or the network")

    monkeypatch.setattr(agent, "run_agent", explode)
    monkeypatch.setattr(agent, "_client", explode)
    monkeypatch.setattr(sample, "load_rows", explode)
    monkeypatch.setattr(workspace, "prepare", explode)
    monkeypatch.setattr(driver, "context_tools", explode)


def test_dry_run_validates_every_arm_over_the_whole_list_with_no_model_and_no_network(
    monkeypatch, tmp_path, make_repo, capsys
):
    root = make_repo(IDS)
    no_model(monkeypatch)
    findings = driver.dry_run(root)
    assert findings == []
    out = capsys.readouterr().out
    for iid in IDS:
        assert iid in out
    for arm in prompts.ARMS:
        assert arm in out


def test_dry_run_needs_no_signed_preregistration(monkeypatch, make_repo):
    # It spends nothing, and it is the mode CI runs on an unsigned repo.
    root = make_repo(IDS, status="UNSIGNED", tag=False)
    no_model(monkeypatch)
    assert driver.dry_run(root) == []


def test_dry_run_checks_the_assembled_prompt_not_the_module_constant(monkeypatch, make_repo):
    root = make_repo(IDS)
    no_model(monkeypatch)
    real_build = prompts.build

    def leaky_build(arm: str) -> str:
        prompt = real_build(arm)
        # Only the assembled RTDD arm gains a sentence; prompts.build's own
        # byte-equality tests never see it.
        return prompt + "\nRun the ranked tests first.\n" if arm == "rtdd" else prompt

    monkeypatch.setattr(driver.prompts, "build", leaky_build)
    findings = driver.dry_run(root)
    assert findings, "an arm that is no longer its control plus one block passed the dry run"
    assert any("rtdd" in f for f in findings)


def test_dry_run_lints_the_context_block_as_assembled(monkeypatch, make_repo):
    root = make_repo(IDS)
    no_model(monkeypatch)
    real_build = prompts.build

    def imperative_build(arm: str) -> str:
        prompt = real_build(arm)
        if arm == "rtdd":
            return prompt.replace(
                "A tool named rtdd_which", "You must run the ranked tests. A tool named rtdd_which"
            )
        return prompt

    monkeypatch.setattr(driver.prompts, "build", imperative_build)
    findings = driver.dry_run(root)
    assert any("you must" in f for f in findings), findings


def test_dry_run_flags_a_control_arm_that_grew_a_context_block(monkeypatch, make_repo):
    root = make_repo(IDS)
    no_model(monkeypatch)
    real_build = prompts.build

    def leaky_control(arm: str) -> str:
        prompt = real_build(arm)
        if arm == "vanilla":
            return prompt + "\n" + prompts.context_block("A tool named rtdd_which exists.")
        return prompt

    monkeypatch.setattr(driver.prompts, "build", leaky_control)
    assert any("vanilla" in f for f in driver.dry_run(root))


def test_main_dry_run_exits_non_zero_when_an_arm_violates_the_guarantee(monkeypatch, make_repo, capsys):
    root = make_repo(IDS)
    no_model(monkeypatch)
    real_build = prompts.build
    monkeypatch.setattr(
        driver.prompts,
        "build",
        lambda arm: real_build(arm) + ("\nRun the tests.\n" if arm == "rtdd_tdd" else ""),
    )
    code = driver.main(["--dry-run", "--repo-root", str(root)])
    assert code != 0
    assert "rtdd_tdd" in capsys.readouterr().err


def test_main_dry_run_is_green_on_this_repo_for_all_five_arms(monkeypatch, capsys):
    # Exactly what CI runs, against the real frozen instance list.
    no_model(monkeypatch)
    repo_root = Path(__file__).resolve().parents[3]
    assert driver.main(["--dry-run", "--repo-root", str(repo_root)]) == 0
    out = capsys.readouterr().out
    assert "0 findings" in out
    for arm in prompts.ARMS:
        assert arm in out


def test_main_dry_run_accepts_no_arm_argument(monkeypatch, make_repo):
    root = make_repo(IDS)
    no_model(monkeypatch)
    assert driver.main(["--dry-run", "--repo-root", str(root)]) == 0


# --- the arm's name never reaches the prompt ------------------------------


def test_the_workspace_the_prompt_names_is_the_same_for_every_arm():
    # ``work_root`` feeds ``prompts.BASE``'s "Repository root:" line, so an
    # arm-shaped path would put the arm's own name — and its length — inside the
    # system prompt of every instance. It takes no arm; there is nothing to pass.
    assert list(inspect.signature(driver.work_root).parameters) == []
    root_lines = {
        next(
            line
            for line in driver.assemble(arm, driver.dry_run_root(IDS[0]), "why").splitlines()
            if line.startswith("Repository root:")
        )
        for arm in prompts.ARMS
    }
    assert len(root_lines) == 1, root_lines


def test_the_dry_run_assembles_against_the_root_a_real_run_uses(monkeypatch, tmp_path, make_repo):
    root = make_repo(IDS)
    rec = stub(monkeypatch, tmp_path)
    driver.run_arm(root, "rtdd", agent.AgentConfig(), Budget())
    statement = rows_for(IDS)[IDS[0]]["problem_statement"]
    assert driver.dry_run_root(IDS[0]) == driver.work_root() / IDS[0]
    assert rec.prompts[IDS[0]] == driver.assemble("rtdd", driver.dry_run_root(IDS[0]), statement)


def test_two_arms_hand_the_model_prompts_that_differ_by_exactly_the_block(monkeypatch, tmp_path, make_repo):
    # The guarantee on the bytes an instance is actually given, not on
    # prompts.build's constants and not on a substituted stand-in root.
    root = make_repo(IDS)
    handed: dict[str, str] = {}
    for arm in prompts.ARMS:
        rec = stub(monkeypatch, tmp_path)
        driver.run_arm(root, arm, agent.AgentConfig(), Budget())
        handed[arm] = rec.prompts[IDS[0]]

    for arm in prompts.ARMS:
        control = prompts.control_for(arm)
        if control is None:
            assert prompts.extract_context(handed[arm]) is None
            continue
        block = prompts.extract_context(handed[arm])
        assert handed[arm] == handed[control] + "\n" + prompts.context_block(block)


def test_a_composition_violation_stops_the_arm_instead_of_being_recorded(monkeypatch, tmp_path, make_repo):
    root = make_repo(IDS)
    rec = stub(monkeypatch, tmp_path)
    real_build = prompts.build
    monkeypatch.setattr(
        driver.prompts,
        "build",
        lambda arm: real_build(arm) + ("\nRun the ranked tests first.\n" if arm == "rtdd" else ""),
    )
    with pytest.raises(driver.CompositionError):
        driver.run_arm(root, "rtdd", agent.AgentConfig(), Budget())
    assert rec.ran == [], "the arm spent a token on a prompt that violates the guarantee"


def test_main_exits_5_on_a_composition_violation(monkeypatch, tmp_path, make_repo, capsys):
    root = make_repo(IDS)
    stub(monkeypatch, tmp_path)
    real_build = prompts.build
    monkeypatch.setattr(
        driver.prompts,
        "build",
        lambda arm: real_build(arm) + ("\nRun the ranked tests first.\n" if arm == "rtdd" else ""),
    )
    code = driver.main(["rtdd", "--repo-root", str(root)])
    assert code == 5
    assert "COMPOSITION" in capsys.readouterr().err


# --- a resume tops the spend up, it does not erase it ---------------------


def test_a_resume_tops_up_the_recorded_spend_instead_of_erasing_it(monkeypatch, tmp_path, make_repo):
    root = make_repo(IDS)
    stub(monkeypatch, tmp_path)
    driver.run_arm(root, "vanilla", agent.AgentConfig(), Budget())
    before = json.loads(driver.cost_path(root, "vanilla").read_text(encoding="utf-8"))
    assert (before["prompt_tokens"], before["completion_tokens"]) == (3000, 300)

    driver.raw_path(root, "vanilla", IDS[1]).unlink()
    rec2 = stub(monkeypatch, tmp_path)
    driver.run_arm(root, "vanilla", agent.AgentConfig(), Budget())
    assert rec2.ran == [IDS[1]]

    after = json.loads(driver.cost_path(root, "vanilla").read_text(encoding="utf-8"))
    assert (after["prompt_tokens"], after["completion_tokens"]) == (4000, 400)
    assert after["usd_estimated"] > before["usd_estimated"]


def test_the_spend_cap_is_a_per_benchmark_ceiling_not_a_per_invocation_one(monkeypatch, tmp_path, make_repo):
    root = make_repo(IDS)
    rec = stub(monkeypatch, tmp_path)
    cap = dict(max_usd=0.0015, usd_per_m_prompt=1.0, usd_per_m_completion=1.0)
    with pytest.raises(BudgetExceeded):
        driver.run_arm(root, "vanilla", agent.AgentConfig(), Budget(**cap))
    assert rec.ran == [IDS[0], IDS[1]]

    rec2 = stub(monkeypatch, tmp_path)
    with pytest.raises(BudgetExceeded):
        driver.run_arm(root, "vanilla", agent.AgentConfig(), Budget(**cap))
    assert rec2.ran == [], "the resume was handed the whole cap again"
    assert sorted(records(root, "vanilla")) == sorted(IDS[:2])
