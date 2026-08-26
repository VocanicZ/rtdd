"""Run one arm over the frozen instance list. Resumable, gated, budgeted.

This is the driver the whole benchmark passes through, so its guarantees are
structural rather than remembered:

  1. **The gate comes first.** :func:`run_arm` calls ``preflight`` before it
     prepares a workspace or builds a prompt, so no arm can spend a token on an
     unsigned, untagged, or mismatched pre-registration.
  2. **The frozen list is the spine.** Instances come from ``instances.txt`` in
     file order. Nothing here draws a sample; ``sample.draw`` is not called.
  3. **Every instance leaves a record.** A crash inside an instance is written
     down as a failed instance — empty patch, ``stop_reason: harness_error`` —
     rather than skipped, because an instance that vanishes silently shrinks the
     denominator every published rate divides by.
  4. **Resumable.** An instance whose record already exists is skipped, so a
     crash costs the current instance and nothing else, and the arm's previous
     spend is restored from its cost file so ``--max-usd`` stays a ceiling on
     the benchmark rather than on each restart.
  5. **The record is written before the budget is charged.** The tokens are
     already spent by then; charging first would let a ceiling stop discard the
     instance that paid for it.
  6. **The composition guarantee is checked on the bytes that ship.** Before an
     instance's prompt reaches the model, :func:`check_assembled` re-assembles
     all five arms from *that instance's real workspace root* and refuses the
     run if any arm is not its control plus exactly one ``<test-context>``
     block. The workspace carries no arm name for exactly this reason: it is
     interpolated into ``prompts.BASE``, and a per-arm path would make the arms
     differ by their root line as well as by the block.

``--dry-run`` is the sixth guarantee's mirror image: it exercises the arm
builders for all five arms across the whole instance list with **no model call
and no network**, and asserts on the prompts *as assembled here* — control plus
exactly one ``<test-context>`` block, byte for byte, with no imperative inside
that block. That is a stronger check than the module-level one in
``tests/test_prompts.py``, which can only see ``prompts.build``'s constants and
would not notice a driver that appended a sentence of its own at run time.

The dry run needs no signed pre-registration: it spends nothing, and it is the
mode CI runs on a repo whose pre-registration is deliberately unsigned. It
substitutes a stand-in problem statement, because reading the real ones means
the network; the guarantee it checks is a property of the composition, and the
statement is substituted into the one shared ``BASE`` every arm carries.
"""

from __future__ import annotations

import argparse
import json
import sys
import traceback
from pathlib import Path

import agent
import prompts
import sample
import tools as toolmod
import workspace
from budget import Budget, BudgetExceeded
from preflight import preflight
from prereg import PreregError
from providers import context_tools

#: Mirror and provider caches. A module attribute so tests can redirect it.
CACHE_ROOT = Path.home() / ".cache" / "rtdd-bench"
#: Per-arm checkout root. Also a module attribute for the same reason.
WORK_ROOT = Path("/tmp/rtdd-bench")

#: Stands in for the dataset's ``problem_statement`` when no model is being
#: called. Deliberately bland: the dry run is checking the composition around
#: it, and text that itself looked like a context block or an instruction would
#: be checking the stand-in instead.
DRY_RUN_PROBLEM_STATEMENT = (
    "A stand-in problem statement for instance {instance_id}. "
    "The dry run substitutes this in place of the dataset's text so that no "
    "network call is needed to assemble a prompt."
)


def instance_ids(repo_root: Path) -> list[str]:
    """The frozen sample, in file order — the only list this driver iterates."""
    path = repo_root / "bench" / "swebench" / "instances.txt"
    return [
        line.strip()
        for line in path.read_text(encoding="utf-8").splitlines()
        if line.strip()
    ]


def raw_path(repo_root: Path, arm: str, instance_id: str) -> Path:
    return repo_root / "bench" / "results" / "swebench" / "raw" / arm / f"{instance_id}.json"


def cost_path(repo_root: Path, arm: str) -> Path:
    """Where this arm's spend is written, prices included, however the run ends."""
    return repo_root / "bench" / "results" / "swebench" / "cost" / arm / "cost.json"


def read_cost(repo_root: Path, arm: str) -> dict:
    """The spend this arm's previous runs recorded, or ``{}`` if it has none.

    Read back so a resume tops the spend up instead of erasing it: the ``finally``
    block rewrites this same file, and a budget that started at zero would
    publish a cost of one restart for a benchmark that took several.
    """
    path = cost_path(repo_root, arm)
    if not path.exists():
        return {}
    return json.loads(path.read_text(encoding="utf-8"))


def work_root() -> Path:
    """The checkout root, and deliberately not a per-arm one.

    ``workspace.prepare`` puts the instance under this root, and that path is
    interpolated into ``prompts.BASE``'s ``Repository root:`` line. A per-arm
    root would therefore ship the arm's own name — and its string length —
    inside the system prompt of every instance, in exactly the arm whose claim
    is "context, no procedure". Arms run one at a time and each instance's
    worktree is rebuilt from the mirror, so one root serves all five.
    """
    return WORK_ROOT


class CompositionError(RuntimeError):
    """An assembled prompt is not its control plus exactly one context block.

    Raised out of the run loop rather than recorded as a failed instance: a
    prompt that violates the guarantee invalidates the comparison, so the arm
    stops instead of quietly producing records nobody may publish.
    """


def assemble(arm: str, repo_root: Path, problem_statement: str) -> str:
    """The one place a prompt is assembled, used by the run loop and the dry run alike.

    If these two ever assembled differently, the dry run would be validating a
    prompt no instance is given.
    """
    return prompts.build(arm).format(
        repo_root=repo_root,
        problem_statement=problem_statement,
    )


def check_assembled(root: Path, problem_statement: str) -> list[str]:
    """Findings against the assembled prompts of all five arms; ``[]`` when clean.

    A context arm must be its control plus exactly one block, byte for byte, and
    that block must survive :func:`prompts.lint_context`. A control arm must
    carry no block at all.
    """
    assembled = {arm: assemble(arm, root, problem_statement) for arm in prompts.ARMS}
    findings: list[str] = []
    for arm in prompts.ARMS:
        prompt = assembled[arm]
        try:
            context = prompts.extract_context(prompt)
        except prompts.ContextBlockError as exc:
            findings.append(f"{arm}: {exc}")
            continue
        control = prompts.control_for(arm)
        if control is None:
            if context is not None:
                findings.append(f"{arm}: a control arm assembled with a <test-context> block")
            continue
        if context is None:
            findings.append(f"{arm}: a context arm assembled without a <test-context> block")
            continue
        expected = assembled[control] + "\n" + prompts.context_block(context)
        if prompt != expected:
            findings.append(
                f"{arm}: assembled prompt is not {control} plus one context block, byte for byte"
            )
        findings.extend(f"{arm}: {f}" for f in prompts.lint_context(context))
    return findings


def dry_run_root(instance_id: str) -> Path:
    """The workspace path the dry run assembles against — the real one.

    Not a stand-in and not a substitution: :func:`work_root` carries no arm, so
    the path an instance is actually handed is already the same for all five
    arms, and the dry run can check the bytes that ship rather than bytes made
    comparable for it.
    """
    return work_root() / instance_id


def dry_run(repo_root: Path, out=print) -> list[str]:
    """Build and validate every arm's prompt for every instance, with no model call.

    Returns the findings, empty when the guarantee holds. The first instance's
    prompts are printed in full so the CI log carries the artefact that was
    checked rather than only the verdict.
    """
    ids = instance_ids(repo_root)
    findings: list[str] = []
    for index, instance_id in enumerate(ids, 1):
        statement = DRY_RUN_PROBLEM_STATEMENT.format(instance_id=instance_id)
        root = dry_run_root(instance_id)
        if index == 1:
            for arm in prompts.ARMS:
                prompt = assemble(arm, root, statement)
                out(f"--- {arm} / {instance_id} ({len(prompt)} bytes) ---")
                out(prompt)
        found = check_assembled(root, statement)
        findings.extend(f"{instance_id}: {f}" for f in found)
        out(f"[{index}/{len(ids)}] {instance_id} {len(prompts.ARMS)} arms, {len(found)} findings")
    out(
        f"dry run: {len(ids)} instances x {len(prompts.ARMS)} arms "
        f"({', '.join(prompts.ARMS)}), {len(findings)} findings"
    )
    return findings


def run_arm(repo_root: Path, arm: str, cfg: agent.AgentConfig, budget: Budget) -> None:
    """Drive one arm over the frozen list, one JSON record per instance."""
    pr = preflight(repo_root)
    if arm not in pr.fields["arms"]:
        raise PreregError(f"arm {arm!r} is not pre-registered; arms are {pr.fields['arms']}")

    cache = CACHE_ROOT
    work = work_root()
    ids = instance_ids(repo_root)
    rows = sample.load_rows()
    prior = read_cost(repo_root, arm)
    if prior:
        budget.restore(prior)
        print(
            f"{arm}: resuming with {budget.prompt_tokens} prompt and "
            f"{budget.completion_tokens} completion tokens already spent "
            f"(${budget.usd:.2f} of ${budget.max_usd:.2f})"
        )
    budget.start()

    try:
        # A resume that is already over a ceiling spends nothing more.
        budget.check()
        for index, instance_id in enumerate(ids, 1):
            out = raw_path(repo_root, arm, instance_id)
            if out.exists():
                print(f"[{index}/{len(ids)}] {arm} {instance_id} cached")
                continue
            out.parent.mkdir(parents=True, exist_ok=True)
            row = rows[instance_id]
            record: dict = {"instance_id": instance_id, "arm": arm, "model": cfg.model}
            try:
                ws = workspace.prepare(cache / "mirrors", work, row)
                # The guarantee, on the bytes this instance is about to be
                # handed: all five arms assembled from this very root, and each
                # context arm still its control plus exactly one block.
                violations = check_assembled(ws.root, row["problem_statement"])
                if violations:
                    raise CompositionError(
                        f"{instance_id}: " + "; ".join(violations)
                    )
                extra, provenance = context_tools(arm, ws, cache, row)
                record.update(provenance)
                system_prompt = assemble(arm, ws.root, row["problem_statement"])
                result = agent.run_agent(
                    ws, system_prompt, toolmod.base_tools(ws) + extra, cfg
                )
                record.update(
                    {
                        "patch": result.patch,
                        "turns": result.turns,
                        "prompt_tokens": result.prompt_tokens,
                        "completion_tokens": result.completion_tokens,
                        "stop_reason": result.stop_reason,
                        "error": None,
                    }
                )
            except CompositionError:
                # Not a failed instance: a broken comparison. Out, with the cost
                # file still written by the finally block below.
                raise
            except Exception as exc:  # an instance that errors is a failed instance
                record.update(
                    {
                        "patch": "",
                        "turns": 0,
                        "prompt_tokens": 0,
                        "completion_tokens": 0,
                        "stop_reason": "harness_error",
                        "error": f"{type(exc).__name__}: {exc}",
                        "traceback": traceback.format_exc()[-4000:],
                    }
                )
            # Written before the budget is charged: those tokens are already
            # spent, and a ceiling stop must not discard the instance that
            # bought them.
            out.write_text(json.dumps(record, indent=2), encoding="utf-8")
            budget.add(record["prompt_tokens"], record["completion_tokens"])
            print(
                f"[{index}/{len(ids)}] {arm} {instance_id} "
                f"{record['stop_reason']} patch={len(record['patch'])}B "
                f"${budget.usd:.2f}"
            )
    finally:
        cost = cost_path(repo_root, arm)
        cost.parent.mkdir(parents=True, exist_ok=True)
        cost.write_text(json.dumps(budget.snapshot(), indent=2), encoding="utf-8")


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("arm", nargs="?", choices=list(prompts.ARMS))
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="assemble and validate every arm's prompt for every instance; no model, no network",
    )
    parser.add_argument("--repo-root", default=None)
    parser.add_argument("--base-url", default=agent.AgentConfig.base_url)
    parser.add_argument("--model", default=agent.AgentConfig.model)
    parser.add_argument("--max-usd", type=float, default=Budget.max_usd)
    parser.add_argument("--max-wall-hours", type=float, default=Budget.max_wall_hours)
    args = parser.parse_args(argv)

    repo_root = (
        Path(args.repo_root).resolve()
        if args.repo_root
        else Path(__file__).resolve().parents[2]
    )

    if args.dry_run:
        findings = dry_run(repo_root)
        if findings:
            print("DRY RUN FAILED: the arm composition guarantee does not hold", file=sys.stderr)
            for finding in findings:
                print(f"  {finding}", file=sys.stderr)
            return 5
        return 0

    if args.arm is None:
        parser.error("an arm is required unless --dry-run is given")

    cfg = agent.AgentConfig(base_url=args.base_url, model=args.model)
    budget = Budget(max_usd=args.max_usd, max_wall_hours=args.max_wall_hours)
    try:
        run_arm(repo_root, args.arm, cfg, budget)
    except PreregError as exc:
        print(f"PREFLIGHT REFUSED: {exc}", file=sys.stderr)
        return 3
    except CompositionError as exc:
        print(f"COMPOSITION VIOLATION: {exc}", file=sys.stderr)
        return 5
    except BudgetExceeded as exc:
        print(f"BUDGET STOP: {exc}", file=sys.stderr)
        return 4
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
