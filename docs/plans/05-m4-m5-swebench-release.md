# RTDD M4/M5 — SWE-bench Verified and Release

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Answer spec §3's question — does a dynamic coverage map beat a static dependency graph at reducing AI-agent regressions? — by running five arms on one local SWE-bench Verified harness under a pre-registered kill criterion, then ship RTDD with generated agent front-ends and a README that cites its prior art before its results.

**Architecture:** `bench/swebench/` is a Python 3.12/uv harness that drives a frozen 100-instance sample of SWE-bench Verified through five prompt arms against one locally served open-weight model, evaluates every patch with the official SWE-bench Docker harness, and derives regression rates mechanically from `PASS_TO_PASS` status — never from prose. Arm prompts are composed so that each context-bearing arm is byte-provably its own control plus one delimited `<test-context>` block, which makes smuggling a procedural protocol into the RTDD arm structurally impossible rather than merely discouraged. `cmd/rtdd-gen` is a Go generator that renders `protocol/PROTOCOL.md` into three target-specific front-ends with per-target section sets, byte budgets, and per-target validity assertions, all checked in CI so a stale-check pass cannot mask wrong content.

**Tech Stack:** Python 3.12+/uv for the benchmark harness, Go 1.24+ for the generator, GoReleaser, GitHub Actions

## Global Constraints

- **Pre-registration is a git tag, not a promise.** `bench/PREREGISTRATION.md` must carry `status: SIGNED` and be reachable from the `prereg-m4` tag; `preflight.py` refuses to launch any arm otherwise.
- **The kill criterion number is chosen by a human.** Task 2 is a hard stop. No agent writes, guesses, defaults, or infers that value.
- **The RTDD arm carries context, never procedure.** Arm D's prompt must equal arm A's prompt plus one `<test-context>` block, byte for byte, and the block is lint-checked against an imperative-phrase blocklist. TDAD measured procedural prose at 9.94% vs 6.08% vanilla; that arm must not leak into ours.
- **Regression is mechanical.** A regression is a test id in the harness report's `tests_status.PASS_TO_PASS.failure` list. The denominator comes from the dataset, not the report, so a crashed evaluation cannot shrink it.
- **An arm cannot win by doing nothing.** Regression rate is never printed without resolution rate beside it; unapplied and empty patches stay in the denominator.
- **No cross-citation without a label.** If the local vanilla arm falls outside the pre-registered equivalence band around TDAD's published 6.08%, every table gets a `HARNESS DIVERGENCE` banner and TDAD's published figures are suppressed from the comparison columns.
- **Prior art before results.** The README's prior-art section ships in the same commit as the first results table, never after.
- **`dist/` is generated.** A hand edit under `dist/` is a CI failure, not a warning.
- **Publish either way.** Spec §15 commits to publishing the result and contributing to TDAD if the static graph matches or beats RTDD. Task 23 is that branch and it ships.

---

## File Structure

```
rtdd/
  bench/
    PREREGISTRATION.md              signed, tagged prereg-m4, gates every run
    corpus.yaml                     Axis 2, frozen in M3 — untouched here
    swebench/
      pyproject.toml                uv project, Python 3.12+
      __init__.py
      prereg.py                     pre-registration parser + tag gate
      preflight.py                  refuses to run unless everything is signed
      sample.py                     deterministic instance draw
      instances.txt                 FROZEN — 100 instance ids, sha256 in PREREGISTRATION.md
      metrics.py                    the mechanical definition of "regression"
      prompts.py                    arm composition; the non-procedural guarantee
      agent.py                      OpenAI-compatible tool loop
      tools.py                      read/list/grep/edit/shell + arm-gated context tools
      workspace.py                  per-instance worktree at base_commit
      providers/
        __init__.py                 registry: vanilla, tdd, tdad, rtdd, rtdd_tdd
        tdad.py                     `tdad index` / `tdad impact` bridge
        rtdd.py                     `rtdd seed` (cached) / `rtdd which --json` bridge
      run_arm.py                    per-arm driver, resumable
      evaluate.py                   SWE-bench Docker harness wrapper
      analyze.py                    rates, Wilson CIs, equivalence band, kill criterion
      report.py                     markdown tables written to bench/results
      budget.py                     spend/turn ceilings and the kill switch
      tests/
        test_prereg.py
        test_sample.py
        test_metrics.py
        test_prompts.py
        test_analyze.py
        test_install_merge.py       (mirror check; Go side owns the real one)
    results/
      swebench/
        raw/<arm>/<instance_id>.json
        predictions/<arm>.jsonl
        reports/<arm>/<instance_id>.json
        summary.json
        tables.md
        config.json
        cost.json
  protocol/
    PROTOCOL.md                     THE single source
  dist/                             GENERATED — never hand-edited
    SKILL.md
    AGENTS.md
    cursor/rules/rtdd.mdc
  cmd/
    rtdd/                           existing CLI (M1a–M3)
    rtdd-gen/main.go                generator CLI: render | check | verify
  internal/
    protocol/
      parse.go                      PROTOCOL.md section parser
      render.go                     per-target renderers + budgets
      targets.go                    target table: sections, budgets, validators
      protocol_test.go
      testdata/
        golden/SKILL.md
        golden/AGENTS.md
        golden/rtdd.mdc
        bad_budget.md
        bad_unterminated.md
    install/
      merge.go                      marker-delimited merge for AGENTS.md / CLAUDE.md
      install.go                    rtdd init file plan + apply
      merge_test.go
  .goreleaser.yaml
  install.sh
  README.md
  .github/workflows/ci.yml
  .github/workflows/release.yml
  docs/results/
    2026-XX-XX-swebench-verified.md
```

---

# M4 — SWE-bench Verified (Axis 1)

## Task 1 — Pre-registration schema and the preflight gate

Build the gate before anything it gates. `preflight.py` must be able to refuse a run for a
missing kill criterion before a single token is spent.

**Files:**
- `bench/swebench/pyproject.toml` (new)
- `bench/swebench/__init__.py` (new)
- `bench/swebench/prereg.py` (new)
- `bench/swebench/preflight.py` (new)
- `bench/swebench/tests/test_prereg.py` (new)
- `bench/PREREGISTRATION.md` (new, `status: UNSIGNED`)

**Interfaces:**
```python
class PreregError(RuntimeError): ...

@dataclass(frozen=True)
class Prereg:
    fields: dict
    path: Path

def load(path: Path) -> Prereg            # raises PreregError unless complete and SIGNED
def assert_tagged(repo_root: Path, path: Path, tag: str = "prereg-m4") -> str
def preflight(repo_root: Path) -> Prereg  # load + assert_tagged + instance-list sha check
```

**Steps:**

- [ ] Create the uv project.

```bash
mkdir -p /home/claude/rtdd/bench/swebench/tests /home/claude/rtdd/bench/swebench/providers
cat > /home/claude/rtdd/bench/swebench/pyproject.toml <<'EOF'
[project]
name = "rtdd-bench-swebench"
version = "0.1.0"
description = "RTDD Axis 1 harness: SWE-bench Verified regression-rate benchmark"
requires-python = ">=3.12"
dependencies = [
  "pyyaml>=6.0.2",
  "datasets>=3.0.0",
  "openai>=1.55.0",
  "swebench>=3.0.0",
]

[dependency-groups]
dev = ["pytest>=8.3.0"]

[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"

[tool.hatch.build.targets.wheel]
packages = ["."]

[tool.pytest.ini_options]
testpaths = ["tests"]
EOF
touch /home/claude/rtdd/bench/swebench/__init__.py /home/claude/rtdd/bench/swebench/providers/__init__.py
cd /home/claude/rtdd/bench/swebench && uv sync
```

- [ ] Write the failing test first.

```python
# bench/swebench/tests/test_prereg.py
import subprocess
from pathlib import Path

import pytest

from prereg import Prereg, PreregError, assert_tagged, load

COMPLETE = """---
status: SIGNED
sample_size: 100
sample_seed: 20260826
instance_list_sha256: 0000000000000000000000000000000000000000000000000000000000000000
model: Qwen3-Coder-30B-A3B-Instruct
arms: [vanilla, tdd, tdad, rtdd, rtdd_tdd]
stratified_recall_floor: 0.95
vanilla_equivalence_k: 2
signed_by: VocanicZ
signed_at: 2026-08-27
---

body
"""


def write(tmp_path: Path, text: str) -> Path:
    p = tmp_path / "PREREGISTRATION.md"
    p.write_text(text, encoding="utf-8")
    return p


def test_complete_and_signed_loads(tmp_path):
    pr = load(write(tmp_path, COMPLETE))
    assert isinstance(pr, Prereg)
    assert pr.fields["stratified_recall_floor"] == 0.95
    assert pr.fields["sample_seed"] == 20260826


def test_unsigned_is_refused(tmp_path):
    text = COMPLETE.replace("status: SIGNED", "status: UNSIGNED")
    with pytest.raises(PreregError, match="not SIGNED"):
        load(write(tmp_path, text))


def test_missing_kill_criterion_is_refused(tmp_path):
    text = COMPLETE.replace("stratified_recall_floor: 0.95\n", "")
    with pytest.raises(PreregError, match="stratified_recall_floor"):
        load(write(tmp_path, text))


def test_blank_kill_criterion_is_refused(tmp_path):
    text = COMPLETE.replace("stratified_recall_floor: 0.95", "stratified_recall_floor:")
    with pytest.raises(PreregError, match="stratified_recall_floor"):
        load(write(tmp_path, text))


def test_placeholder_kill_criterion_is_refused(tmp_path):
    text = COMPLETE.replace("stratified_recall_floor: 0.95", "stratified_recall_floor: TBD")
    with pytest.raises(PreregError, match="numeric"):
        load(write(tmp_path, text))


def test_no_front_matter_is_refused(tmp_path):
    with pytest.raises(PreregError, match="front-matter"):
        load(write(tmp_path, "just a document\n"))


def test_untagged_prereg_is_refused(tmp_path):
    subprocess.run(["git", "init", "-q", str(tmp_path)], check=True)
    subprocess.run(["git", "-C", str(tmp_path), "config", "user.email", "t@t"], check=True)
    subprocess.run(["git", "-C", str(tmp_path), "config", "user.name", "t"], check=True)
    p = write(tmp_path, COMPLETE)
    subprocess.run(["git", "-C", str(tmp_path), "add", "-A"], check=True)
    subprocess.run(["git", "-C", str(tmp_path), "commit", "-qm", "prereg"], check=True)
    with pytest.raises(PreregError, match="prereg-m4"):
        assert_tagged(tmp_path, p)
    subprocess.run(["git", "-C", str(tmp_path), "tag", "prereg-m4"], check=True)
    assert len(assert_tagged(tmp_path, p)) == 40
```

```bash
cd /home/claude/rtdd/bench/swebench && uv run pytest tests/test_prereg.py -q
# expected: ModuleNotFoundError: No module named 'prereg'
```

- [ ] Implement `prereg.py`.

```python
# bench/swebench/prereg.py
"""Parse and gate on bench/PREREGISTRATION.md.

Nothing in M4 may run unless this file is complete, SIGNED by a human, and
reachable from the `prereg-m4` git tag. The kill criterion in particular is
never defaulted: a missing, blank, or non-numeric value is a hard refusal.
"""

from __future__ import annotations

import re
import subprocess
from dataclasses import dataclass
from pathlib import Path

import yaml

REQUIRED: tuple[str, ...] = (
    "status",
    "sample_size",
    "sample_seed",
    "instance_list_sha256",
    "model",
    "arms",
    "stratified_recall_floor",
    "vanilla_equivalence_k",
    "signed_by",
    "signed_at",
)

NUMERIC: tuple[str, ...] = (
    "sample_size",
    "sample_seed",
    "stratified_recall_floor",
    "vanilla_equivalence_k",
)

_FRONT_MATTER = re.compile(r"\A---\n(.*?)\n---\n", re.S)


class PreregError(RuntimeError):
    """The pre-registration is absent, incomplete, unsigned, or untagged."""


@dataclass(frozen=True)
class Prereg:
    fields: dict
    path: Path


def load(path: Path) -> Prereg:
    if not path.exists():
        raise PreregError(f"{path}: pre-registration file does not exist")
    text = path.read_text(encoding="utf-8")
    match = _FRONT_MATTER.match(text)
    if match is None:
        raise PreregError(f"{path}: no YAML front-matter at the top of the file")
    fields = yaml.safe_load(match.group(1)) or {}
    if not isinstance(fields, dict):
        raise PreregError(f"{path}: front-matter is not a mapping")

    blank = [k for k in REQUIRED if fields.get(k) in (None, "", [], {})]
    if blank:
        raise PreregError(f"{path}: missing or blank pre-registered fields: {sorted(blank)}")

    for key in NUMERIC:
        value = fields[key]
        if isinstance(value, bool) or not isinstance(value, (int, float)):
            raise PreregError(
                f"{path}: {key} must be numeric and chosen before the run, got {value!r}"
            )

    if fields["status"] != "SIGNED":
        raise PreregError(
            f"{path}: status is {fields['status']!r}, not SIGNED — a human must sign this"
        )
    return Prereg(fields=fields, path=path)


def _git(repo_root: Path, *args: str) -> str:
    proc = subprocess.run(
        ["git", "-C", str(repo_root), *args],
        capture_output=True,
        text=True,
    )
    if proc.returncode != 0:
        raise PreregError(f"git {' '.join(args)} failed: {proc.stderr.strip()}")
    return proc.stdout.strip()


def assert_tagged(repo_root: Path, path: Path, tag: str = "prereg-m4") -> str:
    """Return the commit the tag points at, or refuse.

    The commit that last modified the pre-registration must be an ancestor of
    (or equal to) the tag, so the signed values cannot be edited after tagging
    without moving the tag — which is visible in the reflog.
    """
    tag_sha = _git(repo_root, "rev-list", "-n", "1", tag)
    rel = path.resolve().relative_to(repo_root.resolve()).as_posix()
    last = _git(repo_root, "log", "-n", "1", "--format=%H", "--", rel)
    if not last:
        raise PreregError(f"{rel} has never been committed")
    merge_base = _git(repo_root, "merge-base", tag_sha, last)
    if merge_base != last:
        raise PreregError(
            f"{rel} was modified in {last[:8]} which is not reachable from tag {tag} "
            f"({tag_sha[:8]}) — re-sign and re-tag before running"
        )
    return tag_sha
```

- [ ] Implement `preflight.py`.

```python
# bench/swebench/preflight.py
"""The single gate every arm runner calls first."""

from __future__ import annotations

import hashlib
import sys
from pathlib import Path

from prereg import Prereg, PreregError, assert_tagged, load


def instance_list_sha256(path: Path) -> str:
    ids = [line.strip() for line in path.read_text(encoding="utf-8").splitlines() if line.strip()]
    return hashlib.sha256(("\n".join(ids) + "\n").encode("utf-8")).hexdigest()


def preflight(repo_root: Path) -> Prereg:
    prereg_path = repo_root / "bench" / "PREREGISTRATION.md"
    pr = load(prereg_path)
    assert_tagged(repo_root, prereg_path)

    instances = repo_root / "bench" / "swebench" / "instances.txt"
    if not instances.exists():
        raise PreregError(f"{instances} does not exist — run sample.py and freeze it")
    actual = instance_list_sha256(instances)
    expected = str(pr.fields["instance_list_sha256"])
    if actual != expected:
        raise PreregError(
            f"instances.txt sha256 {actual} != pre-registered {expected} — "
            "the sample was changed after signing"
        )

    count = len([ln for ln in instances.read_text(encoding="utf-8").splitlines() if ln.strip()])
    if count != int(pr.fields["sample_size"]):
        raise PreregError(f"instances.txt has {count} ids, pre-registered {pr.fields['sample_size']}")
    return pr


if __name__ == "__main__":
    root = Path(__file__).resolve().parents[2]
    try:
        pr = preflight(root)
    except PreregError as exc:
        print(f"PREFLIGHT REFUSED: {exc}", file=sys.stderr)
        raise SystemExit(3)
    print(f"preflight ok: floor={pr.fields['stratified_recall_floor']} "
          f"n={pr.fields['sample_size']} seed={pr.fields['sample_seed']}")
```

- [ ] Write the unsigned template. Note the kill criterion is present as a key with **no
      value**, so `load()` refuses it. This is deliberate: the file is inert until a human
      fills it.

```bash
cat > /home/claude/rtdd/bench/PREREGISTRATION.md <<'EOF'
---
status: UNSIGNED
sample_size: 100
sample_seed: 20260826
instance_list_sha256:
model: Qwen3-Coder-30B-A3B-Instruct
arms: [vanilla, tdd, tdad, rtdd, rtdd_tdd]
stratified_recall_floor:
vanilla_equivalence_k: 2
signed_by:
signed_at:
---

# RTDD Axis 1 pre-registration

This file is the contract. `bench/swebench/preflight.py` refuses to launch any arm until
`status` is `SIGNED`, every field above has a value, and the commit that last touched this
file is reachable from the `prereg-m4` git tag.

## Sample

100 instances drawn from SWE-bench Verified (500) with
`random.Random(20260826).sample(eligible, 100)`, where `eligible` is the sorted list of
instance ids whose `PASS_TO_PASS` set is non-empty. Instances with an empty `PASS_TO_PASS`
set are ineligible because the regression rate is undefined for them; the count of such
exclusions is published. n=100 matches TDAD's published sample size.

## Arms

| id | prompt composition | context tool |
|---|---|---|
| `vanilla` | BASE | none |
| `tdd` | BASE + TDD_PROSE | none |
| `tdad` | BASE + TDD_PROSE + `<test-context>` | `test_impact` |
| `rtdd` | BASE + `<test-context>` | `rtdd_which` |
| `rtdd_tdd` | BASE + TDD_PROSE + `<test-context>` | `rtdd_which` |

`tdad` reproduces TDAD's published best arm, which is graph **plus** TDD prose.
`rtdd` is the project's actual claim: context with no procedure.
`rtdd_tdd` is the parity arm — the exact counterpart of `tdad`, so the comparison is not
confounded by the presence or absence of the prose.

## Regression

A regression is a test id appearing in the SWE-bench evaluation report's
`tests_status.PASS_TO_PASS.failure` list. The denominator is the dataset's `PASS_TO_PASS`
cardinality, not the report's, so an evaluation that crashes cannot shrink it. An instance
whose patch is empty or fails to apply contributes zero regressions and zero resolutions and
stays in both denominators.

Both rates are published:
- test-level: sum of failing P2P tests / sum of P2P tests
- instance-level: fraction of instances with at least one failing P2P test

Resolution rate is printed beside the regression rate in every table.

## Vanilla equivalence band

The local `vanilla` arm reproduces TDAD's published 6.08% if `0.0608` lies within
`local_rate ± vanilla_equivalence_k × wilson_half_width(local_rate, n)`. If it does not,
every published table carries a `HARNESS DIVERGENCE` banner, TDAD's published figures are
removed from the comparison columns, and all claims are stated in local-relative form only.

## Kill criterion

`stratified_recall_floor` is the M3 Axis-2 stratified change-level recall on the
`|F_full| == 1` stratum, below which RTDD's selector is not published regardless of the
Axis 1 outcome. **This value must be written by a human before the run.**

## Model and budget

One model for every arm: `Qwen3-Coder-30B-A3B-Instruct`, served locally, greedy decoding
(`temperature=0`), 32768-token context, 4096 max output tokens per turn, 40 tool-call turns
per instance. Matching TDAD's model class is deliberate — their published numbers were
measured on Qwen3-Coder 30B Q4_K_M, and swapping in a frontier model would make the
comparison to their table meaningless.
EOF
```

- [ ] Run the tests green and commit.

```bash
cd /home/claude/rtdd/bench/swebench && uv run pytest tests/test_prereg.py -q
# expected: 7 passed
git add bench && git commit -m "bench(swebench): pre-registration schema and preflight gate

The gate is built before anything it gates. preflight refuses to launch when the
kill criterion is missing, blank, non-numeric, unsigned, or untagged.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

## Task 2 — HUMAN-IN-THE-LOOP: choose and sign the kill criterion

> ## 🛑 STOP — HUMAN-IN-THE-LOOP
>
> **An autonomous agent MUST NOT complete this task.** It must present the options below,
> stop, and wait for a human to write the number. Spec §15 names this as the single open
> decision, and §10 says the criterion is unfalsifiable if chosen after the number is known.
> Do not pick a default. Do not infer one from M3's results. Do not "use 0.95 for now".

**Files:**
- `bench/PREREGISTRATION.md` (edited by a human)
- `bench/swebench/instances.txt` (frozen by Task 3, sha pasted in here)

**Interfaces:** none — this is a decision, not code.

**Steps:**

- [ ] Agent: read M3's published Axis 2 results and print the one number the decision hangs
      on, then stop.

```bash
cd /home/claude/rtdd && cat bench/results/replay/tables.md | sed -n '/stratum/,/^$/p'
# The relevant row is stratified change-level recall on |F_full| == 1, per repo.
```

- [ ] Agent: present exactly this to the human and **stop**.

```
DECISION REQUIRED — pre-registered kill criterion (spec §15)

Write a stratified recall floor into bench/PREREGISTRATION.md. It is the M3 Axis-2
stratified change-level recall on the |F_full| == 1 stratum below which RTDD's selector
is not published, regardless of what M4 finds.

  0.90  Permissive. Ships a tool that misses roughly one in ten single-killer changes.
        Defensible only if RTDD is framed strictly as a context provider that never
        replaces a full CI run — which spec §2 does say. Weakest claim, highest ship rate.

  0.95  The level at which a selection tool is normally trusted inside a pre-commit loop.
        Costs you the tool if hub coverage is carrying the pooled number and the
        single-killer stratum is weak. This is the level a reviewer will expect to see.

  0.99  Near-parity with the full suite. Honest but probably unreachable: spec §6 documents
        that import-time execution is attributed to no test at all, so a whole class of
        single-killer changes is structurally invisible to a coverage map. Choosing this
        is close to pre-committing to not shipping.

Whatever you choose, it binds. It is evaluated against M3's already-published numbers, so
it cannot be tuned to the M4 outcome after the fact.

I will not choose this for you. Reply with the number, or edit the file yourself.
```

- [ ] Human: write the value, set `status: SIGNED`, fill `signed_by` and `signed_at`, and
      paste the `instance_list_sha256` produced by Task 3.

- [ ] Human: commit and tag.

```bash
cd /home/claude/rtdd
git add bench/PREREGISTRATION.md bench/swebench/instances.txt
git commit -m "bench(swebench): sign the Axis 1 pre-registration

Kill criterion, sample, seed, arms, and equivalence band fixed before any arm runs.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
git tag -a prereg-m4 -m "Axis 1 pre-registration signed"
```

- [ ] Verify the gate opens.

```bash
cd /home/claude/rtdd/bench/swebench && uv run python preflight.py
# expected: preflight ok: floor=<the signed value> n=100 seed=20260826
```

- [ ] Add the gate to CI so a later edit re-breaks it.

```yaml
# append to .github/workflows/ci.yml under jobs:
  prereg:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with: { fetch-depth: 0 }
      - uses: astral-sh/setup-uv@v5
      - run: uv sync
        working-directory: bench/swebench
      - run: uv run python preflight.py
        working-directory: bench/swebench
```

---

## Task 3 — Freeze the instance sample

**Files:**
- `bench/swebench/sample.py` (new)
- `bench/swebench/instances.txt` (new, frozen)
- `bench/swebench/tests/test_sample.py` (new)

**Interfaces:**
```python
DATASET = "princeton-nlp/SWE-bench_Verified"

def eligible_ids(rows: dict[str, dict]) -> list[str]
def draw(rows: dict[str, dict], seed: int, n: int) -> list[str]
def sha256_ids(ids: list[str]) -> str
def load_rows() -> dict[str, dict]
```

**Steps:**

- [ ] Failing test first.

```python
# bench/swebench/tests/test_sample.py
import json

from sample import draw, eligible_ids, sha256_ids


def rows(n: int) -> dict[str, dict]:
    out = {}
    for i in range(n):
        p2p = [] if i % 17 == 0 else [f"t{i}_{j}" for j in range(3)]
        out[f"repo__proj-{i:04d}"] = {
            "instance_id": f"repo__proj-{i:04d}",
            "PASS_TO_PASS": json.dumps(p2p),
            "FAIL_TO_PASS": json.dumps([f"f{i}"]),
        }
    return out


def test_empty_p2p_is_ineligible():
    e = eligible_ids(rows(50))
    assert "repo__proj-0000" not in e
    assert "repo__proj-0017" not in e
    assert "repo__proj-0001" in e


def test_eligible_is_sorted():
    e = eligible_ids(rows(50))
    assert e == sorted(e)


def test_draw_is_deterministic():
    r = rows(300)
    a = draw(r, seed=20260826, n=100)
    b = draw(r, seed=20260826, n=100)
    assert a == b
    assert len(a) == 100
    assert a == sorted(a)


def test_draw_changes_with_seed():
    r = rows(300)
    assert draw(r, seed=20260826, n=100) != draw(r, seed=1, n=100)


def test_sha_is_stable_and_newline_terminated():
    ids = ["a", "b", "c"]
    import hashlib
    assert sha256_ids(ids) == hashlib.sha256(b"a\nb\nc\n").hexdigest()
```

```bash
cd /home/claude/rtdd/bench/swebench && uv run pytest tests/test_sample.py -q
# expected: ModuleNotFoundError: No module named 'sample'
```

- [ ] Implement.

```python
# bench/swebench/sample.py
"""Deterministic, pre-registered instance draw from SWE-bench Verified.

The draw is a pure function of (dataset rows, seed, n) so anyone can reproduce
instances.txt from the pre-registered seed. Instances with an empty PASS_TO_PASS
set are ineligible: the regression rate has no denominator for them.
"""

from __future__ import annotations

import hashlib
import json
import random
import sys
from pathlib import Path

DATASET = "princeton-nlp/SWE-bench_Verified"
SPLIT = "test"


def load_rows() -> dict[str, dict]:
    from datasets import load_dataset

    ds = load_dataset(DATASET, split=SPLIT)
    return {row["instance_id"]: dict(row) for row in ds}


def _p2p(row: dict) -> list[str]:
    raw = row["PASS_TO_PASS"]
    return json.loads(raw) if isinstance(raw, str) else list(raw)


def eligible_ids(rows: dict[str, dict]) -> list[str]:
    return sorted(iid for iid, row in rows.items() if _p2p(row))


def draw(rows: dict[str, dict], seed: int, n: int) -> list[str]:
    pool = eligible_ids(rows)
    if len(pool) < n:
        raise ValueError(f"only {len(pool)} eligible instances, need {n}")
    return sorted(random.Random(seed).sample(pool, n))


def sha256_ids(ids: list[str]) -> str:
    return hashlib.sha256(("\n".join(ids) + "\n").encode("utf-8")).hexdigest()


if __name__ == "__main__":
    seed = int(sys.argv[1]) if len(sys.argv) > 1 else 20260826
    n = int(sys.argv[2]) if len(sys.argv) > 2 else 100
    rows = load_rows()
    pool = eligible_ids(rows)
    ids = draw(rows, seed, n)
    out = Path(__file__).with_name("instances.txt")
    out.write_text("\n".join(ids) + "\n", encoding="utf-8")
    print(f"total={len(rows)} eligible={len(pool)} excluded_empty_p2p={len(rows) - len(pool)}")
    print(f"wrote {out} n={len(ids)}")
    print(f"instance_list_sha256: {sha256_ids(ids)}")
```

- [ ] Freeze the sample and hand the sha to Task 2.

```bash
cd /home/claude/rtdd/bench/swebench && uv run pytest tests/test_sample.py -q
# expected: 5 passed
uv run python sample.py 20260826 100
# expected shape:
#   total=500 eligible=<=500 excluded_empty_p2p=<count>
#   wrote /home/claude/rtdd/bench/swebench/instances.txt n=100
#   instance_list_sha256: <64 hex chars>
```

- [ ] Paste the sha into `bench/PREREGISTRATION.md` (`instance_list_sha256`) and record the
      exclusion count in the "Sample" section as the "attempted and excluded, with reason"
      table spec §10 requires. Commit with Task 2's signing commit.

---

## Task 4 — Encode the definition of "regression"

**Files:**
- `bench/swebench/metrics.py` (new)
- `bench/swebench/tests/test_metrics.py` (new)

**Interfaces:**
```python
@dataclass(frozen=True)
class InstanceResult:
    instance_id: str
    arm: str
    p2p_total: int            # from the DATASET, never from the report
    p2p_failed: tuple[str, ...]
    f2p_total: int
    f2p_passed: int
    patch_applied: bool
    evaluated: bool

def from_report(instance_id, arm, dataset_row, report, patch_applied) -> InstanceResult
def test_level_regression_rate(results) -> float
def instance_level_regression_rate(results) -> float
def resolution_rate(results) -> float
def wilson_ci(successes: int, trials: int, z: float = 1.96) -> tuple[float, float]
```

**Steps:**

- [ ] Failing test first. These tests are the definition; the prose is a comment.

```python
# bench/swebench/tests/test_metrics.py
import json

import pytest

from metrics import (
    InstanceResult,
    from_report,
    instance_level_regression_rate,
    resolution_rate,
    test_level_regression_rate,
    wilson_ci,
)

ROW = {
    "instance_id": "django__django-1",
    "PASS_TO_PASS": json.dumps(["p1", "p2", "p3", "p4"]),
    "FAIL_TO_PASS": json.dumps(["f1", "f2"]),
}


def report(p2p_fail, f2p_pass):
    return {
        "django__django-1": {
            "tests_status": {
                "PASS_TO_PASS": {
                    "success": [t for t in ["p1", "p2", "p3", "p4"] if t not in p2p_fail],
                    "failure": list(p2p_fail),
                },
                "FAIL_TO_PASS": {
                    "success": list(f2p_pass),
                    "failure": [t for t in ["f1", "f2"] if t not in f2p_pass],
                },
            }
        }
    }


def test_regression_is_a_p2p_failure():
    r = from_report("django__django-1", "rtdd", ROW, report(["p2"], ["f1", "f2"]), True)
    assert r.p2p_failed == ("p2",)
    assert r.p2p_total == 4


def test_denominator_comes_from_the_dataset_not_the_report():
    truncated = report(["p2"], ["f1"])
    truncated["django__django-1"]["tests_status"]["PASS_TO_PASS"]["success"] = []
    r = from_report("django__django-1", "rtdd", ROW, truncated, True)
    assert r.p2p_total == 4


def test_unapplied_patch_has_no_regressions_but_stays_in_the_denominator():
    r = from_report("django__django-1", "vanilla", ROW, {}, patch_applied=False)
    assert r.p2p_failed == ()
    assert r.p2p_total == 4
    assert r.evaluated is False
    assert resolution_rate([r]) == 0.0
    assert test_level_regression_rate([r]) == 0.0
    assert instance_level_regression_rate([r]) == 0.0


def test_a_do_nothing_arm_cannot_win():
    # Four instances, no patch at all: zero regressions AND zero resolutions.
    nothing = [from_report("django__django-1", "lazy", ROW, {}, False) for _ in range(4)]
    assert test_level_regression_rate(nothing) == 0.0
    assert resolution_rate(nothing) == 0.0


def test_test_level_rate_pools_tests_not_instances():
    a = from_report("django__django-1", "x", ROW, report(["p1", "p2"], []), True)
    b = from_report("django__django-1", "x", ROW, report([], ["f1", "f2"]), True)
    assert test_level_regression_rate([a, b]) == pytest.approx(2 / 8)
    assert instance_level_regression_rate([a, b]) == pytest.approx(0.5)


def test_resolution_requires_all_f2p_and_no_p2p_failure():
    solved = from_report("django__django-1", "x", ROW, report([], ["f1", "f2"]), True)
    broke = from_report("django__django-1", "x", ROW, report(["p1"], ["f1", "f2"]), True)
    partial = from_report("django__django-1", "x", ROW, report([], ["f1"]), True)
    assert resolution_rate([solved]) == 1.0
    assert resolution_rate([broke]) == 0.0
    assert resolution_rate([partial]) == 0.0


def test_wilson_ci_brackets_the_point_estimate():
    lo, hi = wilson_ci(6, 100)
    assert lo < 0.06 < hi
    assert 0.0 <= lo and hi <= 1.0


def test_wilson_ci_of_zero_successes_has_zero_lower_bound():
    lo, hi = wilson_ci(0, 100)
    assert lo == 0.0
    assert hi > 0.0
```

```bash
cd /home/claude/rtdd/bench/swebench && uv run pytest tests/test_metrics.py -q
# expected: ModuleNotFoundError: No module named 'metrics'
```

- [ ] Implement.

```python
# bench/swebench/metrics.py
"""The mechanical definition of a regression.

A regression is a test id that appears in the SWE-bench evaluation report's
tests_status.PASS_TO_PASS.failure list — a test that passed before the agent's
patch and fails after it. There is no prose definition anywhere in this harness;
this module is it.

Two anti-gaming rules are encoded here rather than trusted to discipline:

  1. p2p_total comes from the dataset row, never from the report. A crashed or
     truncated evaluation therefore cannot shrink an arm's denominator.
  2. An instance whose patch is empty or fails to apply contributes zero
     regressions AND zero resolutions and stays in both denominators, so an arm
     cannot lower its regression rate by producing nothing.
"""

from __future__ import annotations

import json
import math
from dataclasses import dataclass
from collections.abc import Sequence


@dataclass(frozen=True)
class InstanceResult:
    instance_id: str
    arm: str
    p2p_total: int
    p2p_failed: tuple[str, ...]
    f2p_total: int
    f2p_passed: int
    patch_applied: bool
    evaluated: bool


def _ids(row: dict, key: str) -> list[str]:
    raw = row[key]
    return json.loads(raw) if isinstance(raw, str) else list(raw)


def from_report(
    instance_id: str,
    arm: str,
    dataset_row: dict,
    report: dict,
    patch_applied: bool,
) -> InstanceResult:
    p2p_total = len(_ids(dataset_row, "PASS_TO_PASS"))
    f2p_total = len(_ids(dataset_row, "FAIL_TO_PASS"))

    entry = report.get(instance_id) if isinstance(report, dict) else None
    status = (entry or {}).get("tests_status") or {}
    evaluated = bool(status) and patch_applied

    if not evaluated:
        return InstanceResult(
            instance_id=instance_id,
            arm=arm,
            p2p_total=p2p_total,
            p2p_failed=(),
            f2p_total=f2p_total,
            f2p_passed=0,
            patch_applied=patch_applied,
            evaluated=False,
        )

    p2p_failed = tuple(status.get("PASS_TO_PASS", {}).get("failure", []))
    f2p_passed = len(status.get("FAIL_TO_PASS", {}).get("success", []))
    return InstanceResult(
        instance_id=instance_id,
        arm=arm,
        p2p_total=p2p_total,
        p2p_failed=p2p_failed,
        f2p_total=f2p_total,
        f2p_passed=f2p_passed,
        patch_applied=True,
        evaluated=True,
    )


def test_level_regression_rate(results: Sequence[InstanceResult]) -> float:
    denom = sum(r.p2p_total for r in results)
    if denom == 0:
        return 0.0
    return sum(len(r.p2p_failed) for r in results) / denom


def instance_level_regression_rate(results: Sequence[InstanceResult]) -> float:
    if not results:
        return 0.0
    return sum(1 for r in results if r.p2p_failed) / len(results)


def resolution_rate(results: Sequence[InstanceResult]) -> float:
    if not results:
        return 0.0
    solved = sum(
        1
        for r in results
        if r.evaluated and not r.p2p_failed and r.f2p_passed == r.f2p_total
    )
    return solved / len(results)


def regression_counts(results: Sequence[InstanceResult]) -> tuple[int, int]:
    """(failing P2P tests, total P2P tests) — the pair every CI is computed on."""
    return sum(len(r.p2p_failed) for r in results), sum(r.p2p_total for r in results)


def wilson_ci(successes: int, trials: int, z: float = 1.96) -> tuple[float, float]:
    if trials == 0:
        return (0.0, 0.0)
    p = successes / trials
    denom = 1 + z * z / trials
    centre = (p + z * z / (2 * trials)) / denom
    half = (z / denom) * math.sqrt(p * (1 - p) / trials + z * z / (4 * trials * trials))
    return (max(0.0, centre - half), min(1.0, centre + half))
```

- [ ] Green and commit.

```bash
cd /home/claude/rtdd/bench/swebench && uv run pytest tests/test_metrics.py -q
# expected: 9 passed
cd /home/claude/rtdd && git add bench/swebench/metrics.py bench/swebench/tests/test_metrics.py
git commit -m "bench(swebench): encode 'regression' as PASS_TO_PASS failure

Denominator from the dataset, not the report. Unapplied patches stay in both
denominators so an arm cannot win by producing nothing.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

## Task 5 — Arm composition and the non-procedural guarantee

This is the task the whole benchmark's credibility rests on. TDAD measured procedural TDD
prose at **9.94% vs 6.08% vanilla** — worse than nothing. If the RTDD arm smuggles in a
procedural protocol, a win is unattributable and a loss is uninterpretable.

The guarantee is structural, not stylistic: every context-bearing arm's prompt is built as
`<its own control> + "\n" + context_block(text)`, and a test asserts byte equality. There is
no code path that can add a sentence to the RTDD arm without also adding it to vanilla.

**Files:**
- `bench/swebench/prompts.py` (new)
- `bench/swebench/tests/test_prompts.py` (new)

**Interfaces:**
```python
ARMS: tuple[str, ...] = ("vanilla", "tdd", "tdad", "rtdd", "rtdd_tdd")
BASE: str
TDD_PROSE: str
RTDD_CONTEXT: str
TDAD_CONTEXT: str
BANNED_IMPERATIVES: tuple[str, ...]

def context_block(text: str) -> str
def control_for(arm: str) -> str        # the arm this arm must equal, minus its block
def build(arm: str) -> str
def lint_context(text: str) -> list[str]  # [] when the text carries no imperative
```

**Steps:**

- [ ] Failing test first. The first three assertions are the guarantee.

```python
# bench/swebench/tests/test_prompts.py
import pytest

from prompts import (
    ARMS,
    BANNED_IMPERATIVES,
    RTDD_CONTEXT,
    TDAD_CONTEXT,
    build,
    context_block,
    control_for,
    lint_context,
)


def test_rtdd_arm_is_vanilla_plus_one_context_block_byte_for_byte():
    assert build("rtdd") == build("vanilla") + "\n" + context_block(RTDD_CONTEXT)


def test_rtdd_tdd_arm_is_the_tdd_arm_plus_one_context_block():
    assert build("rtdd_tdd") == build("tdd") + "\n" + context_block(RTDD_CONTEXT)


def test_tdad_arm_is_the_tdd_arm_plus_one_context_block():
    # TDAD's published best arm is graph PLUS TDD prose; reproducing it means keeping both.
    assert build("tdad") == build("tdd") + "\n" + context_block(TDAD_CONTEXT)


def test_rtdd_arm_contains_no_tdd_prose():
    from prompts import TDD_PROSE

    assert TDD_PROSE not in build("rtdd")


def test_rtdd_context_carries_no_imperative():
    assert lint_context(RTDD_CONTEXT) == []


def test_lint_catches_a_smuggled_procedure():
    smuggled = RTDD_CONTEXT + "\nBefore you commit, you must run every listed test first."
    findings = lint_context(smuggled)
    assert findings
    assert any("you must" in f for f in findings)


def test_lint_catches_step_numbering():
    assert lint_context("Step 1. Write a failing test.\nStep 2. Make it pass.")


def test_every_banned_phrase_is_actually_caught():
    for phrase in BANNED_IMPERATIVES:
        assert lint_context(f"filler {phrase} filler"), phrase


def test_context_block_is_exactly_one_delimited_block():
    b = context_block("hello")
    assert b.count("<test-context>") == 1
    assert b.count("</test-context>") == 1
    assert b == "<test-context>\nhello\n</test-context>\n"


def test_all_arms_build():
    for arm in ARMS:
        assert build(arm).strip()


def test_unknown_arm_is_an_error():
    with pytest.raises(KeyError):
        build("nope")


def test_control_mapping_is_explicit():
    assert control_for("vanilla") is None
    assert control_for("tdd") is None
    assert control_for("rtdd") == "vanilla"
    assert control_for("rtdd_tdd") == "tdd"
    assert control_for("tdad") == "tdd"
```

```bash
cd /home/claude/rtdd/bench/swebench && uv run pytest tests/test_prompts.py -q
# expected: ModuleNotFoundError: No module named 'prompts'
```

- [ ] Implement.

```python
# bench/swebench/prompts.py
"""Arm composition for Axis 1.

Every context-bearing arm is its control plus exactly one <test-context> block.
The composition is done by `build`, which has no per-arm prose of its own, so
there is no code path that can add a sentence to the RTDD arm without adding it
to the vanilla arm too.

TDAD measured procedural TDD instructions at 9.94% regressions against a 6.08%
vanilla baseline — worse than no intervention. The `rtdd` arm therefore states
what information exists and what it means, and never what to do with it.
"""

from __future__ import annotations

ARMS: tuple[str, ...] = ("vanilla", "tdd", "tdad", "rtdd", "rtdd_tdd")

BASE = """You are working inside a Python repository at a specific commit. A bug report or \
feature request is described below. Edit the repository so the described problem is fixed.

Repository root: {repo_root}

Issue:
{problem_statement}

You have tools for listing directories, reading files, searching with grep, editing files, \
and running shell commands inside the repository. When you consider the work complete, call \
the finish tool. Your final answer is the diff of the working tree, so leave the repository \
in the state you want evaluated and do not commit."""

TDD_PROSE = """Follow a test-driven workflow. Before changing any source file, first write \
or identify a test that fails because of the described problem. Run it and confirm it fails. \
Then make the smallest change that makes it pass, and run the test again to confirm it \
passes. Before you finish, re-run the tests you have touched and make sure none of them \
regressed. Do not skip the failing-test step."""

RTDD_CONTEXT = """A tool named rtdd_which is available in this repository.

Given the repository's current uncommitted changes, it reports two things, derived from \
coverage recorded during a prior full run of this suite:

- the tests whose recorded execution touched the files you have changed, ranked by how much \
of each test's recorded file set the change overlaps;
- the lines you have changed that no recorded test executed, and separately the changed \
lines that executed only at import time and are attributed to no test.

The report reflects execution, not static analysis, so it includes edges reached through \
dynamic dispatch, dependency injection, and monkeypatching, and it omits any path no test \
has ever taken. Its output is information about this repository's test suite. It does not \
gate anything and it has no opinion about your patch."""

TDAD_CONTEXT = """A tool named test_impact is available in this repository.

Given a list of source files, it reports the tests at risk of regressing if those files \
change, drawn from a static index of the repository built by parsing its Python syntax \
trees: file-to-function edges, test-to-function edges, transitive call chains, and import \
dependencies. Each reported test carries a risk tier of high, medium, or low.

The index is static, so it covers call edges that no test has yet exercised, and it does \
not see edges that exist only at runtime."""

BANNED_IMPERATIVES: tuple[str, ...] = (
    "you must",
    "you should",
    "make sure",
    "be sure to",
    "first write",
    "before you",
    "before changing",
    "before committing",
    "step 1",
    "step 2",
    "then run",
    "always run",
    "do not skip",
    "follow a",
    "follow these",
    "workflow",
    "red-green",
    "test-driven",
    "procedure",
    "your job is to",
    "start by",
)

_OPEN = "<test-context>"
_CLOSE = "</test-context>"

_CONTROL: dict[str, str | None] = {
    "vanilla": None,
    "tdd": None,
    "tdad": "tdd",
    "rtdd": "vanilla",
    "rtdd_tdd": "tdd",
}

_CONTEXT: dict[str, str | None] = {
    "vanilla": None,
    "tdd": None,
    "tdad": TDAD_CONTEXT,
    "rtdd": RTDD_CONTEXT,
    "rtdd_tdd": RTDD_CONTEXT,
}

_PROSE: dict[str, bool] = {
    "vanilla": False,
    "tdd": True,
    "tdad": True,
    "rtdd": False,
    "rtdd_tdd": True,
}


def context_block(text: str) -> str:
    return f"{_OPEN}\n{text}\n{_CLOSE}\n"


def control_for(arm: str) -> str | None:
    return _CONTROL[arm]


def build(arm: str) -> str:
    if arm not in _CONTROL:
        raise KeyError(arm)
    prompt = BASE
    if _PROSE[arm]:
        prompt = prompt + "\n" + TDD_PROSE
    context = _CONTEXT[arm]
    if context is not None:
        prompt = prompt + "\n" + context_block(context)
    return prompt


def lint_context(text: str) -> list[str]:
    """Return every banned imperative phrase found in a context block."""
    lowered = text.lower()
    return [phrase for phrase in BANNED_IMPERATIVES if phrase in lowered]
```

- [ ] Green, then add a standing CI guard so a later edit to `RTDD_CONTEXT` cannot slip a
      procedure in unnoticed.

```bash
cd /home/claude/rtdd/bench/swebench && uv run pytest tests/test_prompts.py -q
# expected: 12 passed
```

```yaml
# append to the prereg job in .github/workflows/ci.yml
      - run: uv run pytest tests/test_prompts.py -q
        working-directory: bench/swebench
```

- [ ] Commit.

```bash
git add bench/swebench/prompts.py bench/swebench/tests/test_prompts.py .github/workflows/ci.yml
git commit -m "bench(swebench): compose arms so the RTDD arm cannot smuggle a procedure

build(arm) == build(control) + context_block(...), asserted byte for byte, plus an
imperative-phrase lint on the block itself. TDAD measured procedural prose at 9.94%
against 6.08% vanilla; that arm must stay out of ours.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

## Task 6 — Workspace, tools, and the agent loop

One agent scaffold serves every arm. The only per-arm variation is the system prompt from
Task 5 and which context tool is registered.

**Files:**
- `bench/swebench/workspace.py` (new)
- `bench/swebench/tools.py` (new)
- `bench/swebench/agent.py` (new)

**Interfaces:**
```python
@dataclass
class Workspace:
    root: Path
    instance_id: str
    repo: str
    base_commit: str
def prepare(cache: Path, work: Path, row: dict) -> Workspace
def diff(ws: Workspace) -> str

@dataclass
class Tool:
    name: str
    schema: dict
    call: Callable[[dict], str]
def base_tools(ws: Workspace) -> list[Tool]

@dataclass
class AgentResult:
    patch: str
    turns: int
    prompt_tokens: int
    completion_tokens: int
    stop_reason: str
def run_agent(ws, system_prompt, tools, cfg) -> AgentResult
```

**Steps:**

- [ ] Implement `workspace.py`.

```python
# bench/swebench/workspace.py
"""Per-instance checkout of the target repo at its base commit.

Repos are cloned once into a bare cache and then worktree'd per instance, so a
five-arm run over 100 instances does not clone django 500 times.
"""

from __future__ import annotations

import shutil
import subprocess
from dataclasses import dataclass
from pathlib import Path


@dataclass
class Workspace:
    root: Path
    instance_id: str
    repo: str
    base_commit: str


def _run(cwd: Path | None, *args: str, check: bool = True) -> subprocess.CompletedProcess:
    proc = subprocess.run(args, cwd=cwd, capture_output=True, text=True)
    if check and proc.returncode != 0:
        raise RuntimeError(f"{' '.join(args)} failed: {proc.stderr.strip()}")
    return proc


def _mirror(cache: Path, repo: str) -> Path:
    target = cache / repo.replace("/", "__")
    if not target.exists():
        target.parent.mkdir(parents=True, exist_ok=True)
        _run(None, "git", "clone", "--bare", f"https://github.com/{repo}.git", str(target))
    else:
        _run(target, "git", "fetch", "--all", "--tags", "--prune", check=False)
    return target


def prepare(cache: Path, work: Path, row: dict) -> Workspace:
    repo = row["repo"]
    base_commit = row["base_commit"]
    instance_id = row["instance_id"]
    mirror = _mirror(cache, repo)

    root = work / instance_id
    if root.exists():
        shutil.rmtree(root)
    root.parent.mkdir(parents=True, exist_ok=True)
    _run(None, "git", "clone", "--quiet", "--shared", str(mirror), str(root))
    _run(root, "git", "checkout", "--quiet", "--detach", base_commit)
    _run(root, "git", "config", "user.email", "bench@rtdd.local")
    _run(root, "git", "config", "user.name", "rtdd-bench")
    return Workspace(root=root, instance_id=instance_id, repo=repo, base_commit=base_commit)


def diff(ws: Workspace) -> str:
    """The prediction: every tracked and untracked change, as one patch."""
    _run(ws.root, "git", "add", "-A", "-N")
    proc = _run(ws.root, "git", "diff", "--no-color", "--binary")
    return proc.stdout
```

- [ ] Implement `tools.py`.

```python
# bench/swebench/tools.py
"""The tool surface every arm shares, plus the per-arm context tool slot."""

from __future__ import annotations

import subprocess
from collections.abc import Callable
from dataclasses import dataclass
from pathlib import Path

from workspace import Workspace

MAX_READ_BYTES = 60_000
MAX_OUTPUT_BYTES = 20_000
SHELL_TIMEOUT_S = 300


@dataclass
class Tool:
    name: str
    schema: dict
    call: Callable[[dict], str]


def _clip(text: str, limit: int = MAX_OUTPUT_BYTES) -> str:
    if len(text) <= limit:
        return text
    return text[:limit] + f"\n... [clipped, {len(text) - limit} more bytes]"


def _resolve(ws: Workspace, rel: str) -> Path:
    target = (ws.root / rel).resolve()
    if not str(target).startswith(str(ws.root.resolve())):
        raise ValueError(f"path escapes the repository: {rel}")
    return target


def base_tools(ws: Workspace) -> list[Tool]:
    def list_dir(args: dict) -> str:
        target = _resolve(ws, args.get("path", "."))
        if not target.is_dir():
            return f"not a directory: {args.get('path', '.')}"
        entries = sorted(
            (p.name + ("/" if p.is_dir() else "")) for p in target.iterdir()
            if p.name != ".git"
        )
        return _clip("\n".join(entries) or "(empty)")

    def read_file(args: dict) -> str:
        target = _resolve(ws, args["path"])
        if not target.is_file():
            return f"no such file: {args['path']}"
        text = target.read_text(encoding="utf-8", errors="replace")[:MAX_READ_BYTES]
        lines = text.splitlines()
        start = max(1, int(args.get("start", 1)))
        end = min(len(lines), int(args.get("end", len(lines))))
        numbered = "\n".join(f"{i}\t{lines[i - 1]}" for i in range(start, end + 1))
        return _clip(numbered)

    def grep(args: dict) -> str:
        proc = subprocess.run(
            ["grep", "-rnI", "--exclude-dir=.git", "-e", args["pattern"],
             str(_resolve(ws, args.get("path", ".")))],
            capture_output=True, text=True, timeout=SHELL_TIMEOUT_S,
        )
        rel = proc.stdout.replace(str(ws.root) + "/", "")
        return _clip(rel or "(no matches)")

    def edit_file(args: dict) -> str:
        target = _resolve(ws, args["path"])
        old, new = args["old"], args["new"]
        if not target.exists():
            if old:
                return f"no such file: {args['path']}"
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_text(new, encoding="utf-8")
            return f"created {args['path']}"
        text = target.read_text(encoding="utf-8")
        if text.count(old) == 0:
            return "old text not found; read the file again and retry"
        if text.count(old) > 1:
            return f"old text appears {text.count(old)} times; include more context"
        target.write_text(text.replace(old, new, 1), encoding="utf-8")
        return f"edited {args['path']}"

    def run_shell(args: dict) -> str:
        proc = subprocess.run(
            ["bash", "-lc", args["command"]],
            cwd=ws.root, capture_output=True, text=True, timeout=SHELL_TIMEOUT_S,
        )
        return _clip(f"exit={proc.returncode}\n{proc.stdout}\n{proc.stderr}")

    def finish(args: dict) -> str:
        return "FINISH"

    def t(name, desc, props, required, fn) -> Tool:
        return Tool(
            name=name,
            schema={
                "type": "function",
                "function": {
                    "name": name,
                    "description": desc,
                    "parameters": {
                        "type": "object",
                        "properties": props,
                        "required": required,
                        "additionalProperties": False,
                    },
                },
            },
            call=fn,
        )

    s = {"type": "string"}
    i = {"type": "integer"}
    return [
        t("list_dir", "List a directory in the repository.",
          {"path": s}, [], list_dir),
        t("read_file", "Read a file, optionally a line range, with line numbers.",
          {"path": s, "start": i, "end": i}, ["path"], read_file),
        t("grep", "Recursively search the repository for a pattern.",
          {"pattern": s, "path": s}, ["pattern"], grep),
        t("edit_file", "Replace a unique block of text in a file, or create a new file "
                       "by passing an empty old.",
          {"path": s, "old": s, "new": s}, ["path", "old", "new"], edit_file),
        t("run_shell", "Run a bash command from the repository root.",
          {"command": s}, ["command"], run_shell),
        t("finish", "Signal that the work is complete.", {}, [], finish),
    ]
```

- [ ] Implement `agent.py`.

```python
# bench/swebench/agent.py
"""A minimal OpenAI-compatible tool loop.

The endpoint is OpenAI-compatible on purpose: the pre-registered model is
Qwen3-Coder-30B-A3B-Instruct served locally, matching TDAD's model class so the
published 6.08% / 9.94% / 1.82% figures remain a meaningful reference point.
Swapping in a frontier model would make the comparison to their table
meaningless, and is listed as future work rather than run here.
"""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from pathlib import Path

from openai import OpenAI

from tools import Tool
from workspace import Workspace, diff


@dataclass
class AgentConfig:
    base_url: str = "http://127.0.0.1:8000/v1"
    api_key: str = "local"
    model: str = "Qwen3-Coder-30B-A3B-Instruct"
    temperature: float = 0.0
    max_tokens: int = 4096
    max_turns: int = 40


@dataclass
class AgentResult:
    patch: str
    turns: int
    prompt_tokens: int
    completion_tokens: int
    stop_reason: str
    transcript: list[dict] = field(default_factory=list)


def run_agent(
    ws: Workspace,
    system_prompt: str,
    tools: list[Tool],
    cfg: AgentConfig,
) -> AgentResult:
    client = OpenAI(base_url=cfg.base_url, api_key=cfg.api_key)
    by_name = {t.name: t for t in tools}
    messages: list[dict] = [{"role": "system", "content": system_prompt}]
    messages.append({"role": "user", "content": "Begin."})

    prompt_tokens = completion_tokens = 0
    stop_reason = "max_turns"

    for turn in range(cfg.max_turns):
        response = client.chat.completions.create(
            model=cfg.model,
            messages=messages,
            tools=[t.schema for t in tools],
            temperature=cfg.temperature,
            max_tokens=cfg.max_tokens,
        )
        usage = getattr(response, "usage", None)
        if usage is not None:
            prompt_tokens += usage.prompt_tokens or 0
            completion_tokens += usage.completion_tokens or 0

        choice = response.choices[0]
        message = choice.message
        messages.append(
            {
                "role": "assistant",
                "content": message.content or "",
                "tool_calls": [tc.model_dump() for tc in (message.tool_calls or [])],
            }
        )

        if not message.tool_calls:
            stop_reason = "no_tool_call"
            break

        finished = False
        for call in message.tool_calls:
            name = call.function.name
            try:
                args = json.loads(call.function.arguments or "{}")
            except json.JSONDecodeError as exc:
                result = f"tool arguments were not valid JSON: {exc}"
            else:
                tool = by_name.get(name)
                if tool is None:
                    result = f"no such tool: {name}"
                else:
                    try:
                        result = tool.call(args)
                    except Exception as exc:  # a tool crash is data, not a run failure
                        result = f"tool error: {type(exc).__name__}: {exc}"
            if result == "FINISH":
                finished = True
                result = "acknowledged"
            messages.append({"role": "tool", "tool_call_id": call.id, "content": result})

        if finished:
            stop_reason = "finish"
            break
    else:
        turn = cfg.max_turns - 1

    return AgentResult(
        patch=diff(ws),
        turns=turn + 1,
        prompt_tokens=prompt_tokens,
        completion_tokens=completion_tokens,
        stop_reason=stop_reason,
        transcript=messages,
    )
```

- [ ] Smoke-test against one instance with the vanilla arm, then commit.

```bash
cd /home/claude/rtdd/bench/swebench
uv run python - <<'EOF'
from pathlib import Path
import prompts, tools, agent, workspace, sample
rows = sample.load_rows()
iid = Path("instances.txt").read_text().split()[0]
ws = workspace.prepare(Path.home()/".cache/rtdd-bench/mirrors", Path("/tmp/rtdd-bench"), rows[iid])
sp = prompts.build("vanilla").format(repo_root=ws.root, problem_statement=rows[iid]["problem_statement"])
res = agent.run_agent(ws, sp, tools.base_tools(ws), agent.AgentConfig())
print(res.stop_reason, res.turns, res.prompt_tokens, res.completion_tokens, len(res.patch))
EOF
# expected shape: finish 14 183422 6110 2143
```

```bash
cd /home/claude/rtdd && git add bench/swebench/workspace.py bench/swebench/tools.py bench/swebench/agent.py
git commit -m "bench(swebench): shared workspace, tool surface, and agent loop

One scaffold for all five arms; the only per-arm variation is the system prompt and
which context tool is registered.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

## Task 7 — TDAD provider, with the not-installable contingency

**Verified 2026-08-26:** the reference implementation exists at
`https://github.com/pepealonso95/TDAD` (MIT, Python, `pip install -e ./tdad`), targets Python
repos, exposes `tdad index` / `tdad impact` / `tdad stats` / `tdad run-tests`, and ships its
own SWE-bench harness under `claudecode_n_codex_swebench/`. Arm C is therefore **run
locally**, not cited. The contingency below exists because a pinned commit can rot.

**Files:**
- `bench/swebench/providers/tdad.py` (new)
- `bench/swebench/providers/__init__.py` (edited)
- `docs/results/tdad-caveat.md` (new, used only by the contingency branch)

**Interfaces:**
```python
TDAD_PIN: str          # exact commit sha, recorded in bench/results/swebench/config.json
def ensure_installed(cache: Path) -> str    # returns the resolved sha
def index(ws: Workspace) -> Path            # builds the static map, returns its path
def impact_tool(ws: Workspace) -> Tool      # the test_impact tool given to arms tdad
```

**Steps:**

- [ ] Pin the reference implementation to a commit and record it.

```bash
cd /home/claude/rtdd
git clone https://github.com/pepealonso95/TDAD.git /tmp/TDAD
git -C /tmp/TDAD rev-parse HEAD
# copy the sha into TDAD_PIN below and into bench/PREREGISTRATION.md under a
# "## Reference implementations" heading, alongside the RTDD commit under test.
```

- [ ] Implement the provider.

```python
# bench/swebench/providers/tdad.py
"""Arm C's context source: TDAD's own reference implementation, run locally.

Verified installable 2026-08-26 — github.com/pepealonso95/TDAD, MIT, Python,
`pip install -e ./tdad`, Python-repo targeted, CLI `tdad index` / `tdad impact`.
Running it rather than citing it is what puts every arm on one harness.
"""

from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path

from tools import Tool, _clip
from workspace import Workspace

TDAD_REPO = "https://github.com/pepealonso95/TDAD.git"
TDAD_PIN = "REPLACE_WITH_SHA_FROM_THE_STEP_ABOVE"


class TdadUnavailable(RuntimeError):
    """TDAD could not be installed or indexed — trigger the contingency branch."""


def ensure_installed(cache: Path) -> str:
    checkout = cache / "TDAD"
    if not checkout.exists():
        checkout.parent.mkdir(parents=True, exist_ok=True)
        subprocess.run(["git", "clone", TDAD_REPO, str(checkout)], check=True)
    subprocess.run(["git", "-C", str(checkout), "checkout", "--quiet", TDAD_PIN], check=True)
    proc = subprocess.run(
        [sys.executable, "-m", "pip", "install", "-e", str(checkout / "tdad")],
        capture_output=True, text=True,
    )
    if proc.returncode != 0:
        raise TdadUnavailable(f"pip install -e TDAD/tdad failed:\n{proc.stderr}")
    check = subprocess.run(["tdad", "--help"], capture_output=True, text=True)
    if check.returncode != 0:
        raise TdadUnavailable(f"tdad CLI not runnable:\n{check.stderr}")
    return TDAD_PIN


def index(ws: Workspace) -> None:
    proc = subprocess.run(
        ["tdad", "index", str(ws.root)],
        capture_output=True, text=True, timeout=1800,
    )
    if proc.returncode != 0:
        raise TdadUnavailable(f"tdad index failed on {ws.instance_id}:\n{proc.stderr}")


def impact_tool(ws: Workspace) -> Tool:
    def call(args: dict) -> str:
        files = args.get("files") or []
        if not files:
            return "no files given"
        proc = subprocess.run(
            ["tdad", "impact", str(ws.root), "--files", *files],
            capture_output=True, text=True, timeout=600,
        )
        if proc.returncode != 0:
            return f"test_impact unavailable: {proc.stderr.strip()[:500]}"
        return _clip(proc.stdout.strip() or "(no impacted tests reported)")

    return Tool(
        name="test_impact",
        schema={
            "type": "function",
            "function": {
                "name": "test_impact",
                "description": (
                    "Report the tests at risk of regressing if the given source files "
                    "change, from a static index of this repository."
                ),
                "parameters": {
                    "type": "object",
                    "properties": {
                        "files": {"type": "array", "items": {"type": "string"}}
                    },
                    "required": ["files"],
                    "additionalProperties": False,
                },
            },
        },
        call=call,
    )
```

- [ ] Verify arm C is genuinely runnable before committing to it.

```bash
cd /home/claude/rtdd/bench/swebench
uv run python - <<'EOF'
from pathlib import Path
from providers import tdad
print("pinned:", tdad.ensure_installed(Path.home()/".cache/rtdd-bench"))
EOF
# expected: pinned: <sha>
# If this raises TdadUnavailable, take the contingency branch below and record why.
```

- [ ] **Contingency branch — only if `ensure_installed` or `index` fails on more than 10 of
      the 100 instances.** Do not silently degrade: record the failure, drop arm C from the
      locally-measured table, and cite TDAD's published 1.82% in a separate, labelled column.
      Write the caveat now so it is not drafted under pressure later.

```bash
mkdir -p /home/claude/rtdd/docs/results
cat > /home/claude/rtdd/docs/results/tdad-caveat.md <<'EOF'
# Methodology caveat: TDAD's arm is cited, not reproduced

TDAD's reference implementation could not be run on this harness. The 1.82% figure in the
comparison table is therefore **quoted from arXiv:2603.17973**, not measured here. What
follows is what that makes unfair, stated so a reader does not have to reconstruct it.

- **Different harness.** TDAD's numbers come from their agent scaffold
  (`claudecode_n_codex_swebench/`), not ours. Tool surface, turn limits, context clipping,
  edit granularity, and shell access all differ, and all of them move regression rates
  independently of the map under test.
- **Possibly different instances.** TDAD sampled 100 instances of SWE-bench Verified. Unless
  their instance list is published and matched exactly, the two 100-instance samples are
  different draws from a 500-instance pool, and the sampling variance is on the order of the
  effect being measured.
- **Different inference stack.** Quantization (Q4_K_M), serving engine, sampling parameters,
  and context length all differ unless matched, and quantization in particular is known to
  change agentic tool-calling behaviour.
- **Our vanilla arm is the only bridge.** Any claim of the form "RTDD beats TDAD" reduces to
  "RTDD's delta against our vanilla exceeds TDAD's published delta against their vanilla".
  That is a comparison of two deltas measured on two harnesses, which is weaker than a
  head-to-head and must be described as such in the README, not in a footnote.
- **Direction only.** With the arm cited rather than run, the honest reportable claim is a
  direction and a rough magnitude, never a ranking. If the two deltas are within a factor of
  two of each other, the correct conclusion published is "no distinguishable difference".

The contingency also changes the ship decision: with arm C cited rather than measured, the
"RTDD wins clearly" row of the Ship / Don't ship table is unreachable, and the outcome is
handled under the "ties" row.
EOF
```

- [ ] Commit.

```bash
cd /home/claude/rtdd && git add bench/swebench/providers/tdad.py docs/results/tdad-caveat.md
git commit -m "bench(swebench): run TDAD's reference implementation as arm C

Verified installable: github.com/pepealonso95/TDAD, MIT, Python, pinned by sha.
The cite-instead-of-run contingency and its fairness caveat are written up front.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

## Task 8 — RTDD provider: seeded maps and the `rtdd_which` tool

RTDD's map must be seeded from a real instrumented run of the target suite at
`base_commit`, which is the expensive part of M4. Seeds are cached by
`(repo, base_commit)` and shared across the `rtdd` and `rtdd_tdd` arms.

**Files:**
- `bench/swebench/providers/rtdd.py` (new)
- `bench/swebench/providers/__init__.py` (edited)

**Interfaces:**
```python
class SeedFailed(RuntimeError): ...
def seed_cache_key(row: dict) -> str
def ensure_seed(cache: Path, ws: Workspace, image: str) -> tuple[Path | None, str]
def install_seed(ws: Workspace, seed: Path | None) -> bool
def which_tool(ws: Workspace) -> Tool
```

**Steps:**

- [ ] Implement.

```python
# bench/swebench/providers/rtdd.py
"""Arm D/E context source: RTDD's own dynamic coverage map.

Seeding runs the instance's full suite once, instrumented, inside that instance's
SWE-bench Docker image at base_commit. Seeds are cached by (repo, base_commit),
so the five-arm run pays for ~100 seeds, not 500.

Pre-registered rule: an instance whose seed fails is NOT dropped. It runs with an
empty map, which makes `rtdd which` report an unseeded map and select nothing, and
the failure is published as a `seed_failed` count. Dropping the hard instances
would bias the arm upward, and the hard instances are exactly the large suites.
"""

from __future__ import annotations

import hashlib
import json
import shutil
import subprocess
from pathlib import Path

from tools import Tool, _clip
from workspace import Workspace

SEED_TIMEOUT_S = 5400
RTDD_BIN = "rtdd"


class SeedFailed(RuntimeError):
    """The instrumented seed run did not produce a usable map."""


def seed_cache_key(row: dict) -> str:
    raw = f"{row['repo']}@{row['base_commit']}".encode("utf-8")
    return hashlib.sha256(raw).hexdigest()[:16]


def _image_for(instance_id: str) -> str:
    # SWE-bench's published per-instance image naming.
    return f"swebench/sweb.eval.x86_64.{instance_id.replace('__', '_1776_')}:latest"


def ensure_seed(cache: Path, ws: Workspace, row: dict) -> tuple[Path | None, str]:
    """Return (path to a cached map.jsonl or None, status)."""
    key = seed_cache_key(row)
    slot = cache / "seeds" / key
    marker = slot / "status.json"
    if marker.exists():
        status = json.loads(marker.read_text(encoding="utf-8"))["status"]
        cached = slot / "map.jsonl"
        return (cached if cached.exists() else None, status)

    slot.mkdir(parents=True, exist_ok=True)
    image = _image_for(ws.instance_id)
    script = (
        "set -e; cd /testbed; "
        "export COVERAGE_CORE=ctrace; "
        f"{RTDD_BIN} init >/dev/null 2>&1 || true; "
        f"{RTDD_BIN} seed; "
        "cat .rtdd/map.jsonl"
    )
    proc = subprocess.run(
        [
            "docker", "run", "--rm",
            "-v", f"{Path(shutil.which(RTDD_BIN)).resolve()}:/usr/local/bin/rtdd:ro",
            image, "bash", "-lc", script,
        ],
        capture_output=True, text=True, timeout=SEED_TIMEOUT_S,
    )
    if proc.returncode != 0 or not proc.stdout.strip():
        (slot / "stderr.txt").write_text(proc.stderr[-40_000:], encoding="utf-8")
        marker.write_text(json.dumps({"status": "seed_failed"}), encoding="utf-8")
        return (None, "seed_failed")

    (slot / "map.jsonl").write_text(proc.stdout, encoding="utf-8")
    marker.write_text(json.dumps({"status": "seeded"}), encoding="utf-8")
    return (slot / "map.jsonl", "seeded")


def install_seed(ws: Workspace, seed: Path | None) -> bool:
    rtdd_dir = ws.root / ".rtdd"
    rtdd_dir.mkdir(exist_ok=True)
    if seed is None:
        return False
    shutil.copyfile(seed, rtdd_dir / "map.jsonl")
    (rtdd_dir / "meta.json").write_text(
        json.dumps({"v": 1, "adapter": "python", "seeded_at": ws.base_commit[:7], "cycles": 0}),
        encoding="utf-8",
    )
    return True


def which_tool(ws: Workspace) -> Tool:
    def call(args: dict) -> str:
        proc = subprocess.run(
            [RTDD_BIN, "which", "--base", "HEAD", "--json"],
            cwd=ws.root, capture_output=True, text=True, timeout=120,
        )
        if proc.returncode != 0:
            return f"rtdd_which unavailable: {proc.stderr.strip()[:500]}"
        return _clip(proc.stdout.strip() or "{}")

    return Tool(
        name="rtdd_which",
        schema={
            "type": "function",
            "function": {
                "name": "rtdd_which",
                "description": (
                    "Report the tests whose recorded execution covered this repository's "
                    "current uncommitted changes, ranked by overlap, and the changed lines "
                    "no recorded test executed. Takes no arguments."
                ),
                "parameters": {
                    "type": "object", "properties": {},
                    "required": [], "additionalProperties": False,
                },
            },
        },
        call=call,
    )
```

- [ ] Wire the registry.

```python
# bench/swebench/providers/__init__.py
"""Per-arm context providers."""

from __future__ import annotations

from pathlib import Path

from tools import Tool
from workspace import Workspace

from . import rtdd as rtdd_provider
from . import tdad as tdad_provider

CONTEXT_TOOL_ARMS = {"tdad", "rtdd", "rtdd_tdd"}


def context_tools(arm: str, ws: Workspace, cache: Path, row: dict) -> tuple[list[Tool], dict]:
    """Return (extra tools, per-instance provenance to record in the raw result)."""
    if arm == "tdad":
        tdad_provider.index(ws)
        return [tdad_provider.impact_tool(ws)], {"tdad_pin": tdad_provider.TDAD_PIN}
    if arm in ("rtdd", "rtdd_tdd"):
        seed, status = rtdd_provider.ensure_seed(cache, ws, row)
        installed = rtdd_provider.install_seed(ws, seed)
        return [rtdd_provider.which_tool(ws)], {"seed_status": status, "seed_installed": installed}
    return [], {}
```

- [ ] Pre-seed everything before any arm runs, so seed cost is paid once and visibly.

```bash
cd /home/claude/rtdd/bench/swebench
uv run python - <<'EOF'
from pathlib import Path
import sample, workspace
from providers import rtdd as rp
cache = Path.home()/".cache/rtdd-bench"
rows = sample.load_rows()
ids = [ln.strip() for ln in Path("instances.txt").read_text().splitlines() if ln.strip()]
ok = fail = 0
for iid in ids:
    ws = workspace.prepare(cache/"mirrors", Path("/tmp/rtdd-bench"), rows[iid])
    _, status = rp.ensure_seed(cache, ws, rows[iid])
    ok, fail = (ok + 1, fail) if status == "seeded" else (ok, fail + 1)
    print(f"{iid}\t{status}")
print(f"seeded={ok} seed_failed={fail}")
EOF
# expected shape: seeded=93 seed_failed=7
# Record both numbers in bench/results/swebench/config.json. Failures are published,
# not dropped.
```

- [ ] Commit.

```bash
cd /home/claude/rtdd && git add bench/swebench/providers
git commit -m "bench(swebench): RTDD seed cache and the rtdd_which context tool

Seeds are instrumented full-suite runs inside each instance's SWE-bench image,
cached by (repo, base_commit). Seed failures run with an empty map and are
published as a count rather than dropped.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

## Task 9 — Arm driver, budget ceilings, and resumability

**Files:**
- `bench/swebench/budget.py` (new)
- `bench/swebench/run_arm.py` (new)

**Interfaces:**
```python
@dataclass
class Budget:
    max_usd: float
    usd_per_m_prompt: float
    usd_per_m_completion: float
    max_wall_hours: float
def spend(prompt_tokens, completion_tokens, b: Budget) -> float
class BudgetExceeded(RuntimeError): ...

def run_arm(repo_root: Path, arm: str, cfg: AgentConfig, budget: Budget) -> None
```

**Steps:**

- [ ] Implement `budget.py`.

```python
# bench/swebench/budget.py
"""Hard ceilings. A benchmark that can run away is a benchmark that will."""

from __future__ import annotations

import time
from dataclasses import dataclass


class BudgetExceeded(RuntimeError):
    pass


@dataclass
class Budget:
    max_usd: float = 75.0
    usd_per_m_prompt: float = 0.10
    usd_per_m_completion: float = 0.40
    max_wall_hours: float = 60.0
    started_at: float = 0.0
    prompt_tokens: int = 0
    completion_tokens: int = 0

    def start(self) -> None:
        self.started_at = time.time()

    def add(self, prompt_tokens: int, completion_tokens: int) -> None:
        self.prompt_tokens += prompt_tokens
        self.completion_tokens += completion_tokens
        if self.usd > self.max_usd:
            raise BudgetExceeded(f"spend ${self.usd:.2f} exceeded cap ${self.max_usd:.2f}")
        hours = (time.time() - self.started_at) / 3600.0
        if hours > self.max_wall_hours:
            raise BudgetExceeded(f"wall clock {hours:.1f}h exceeded cap {self.max_wall_hours}h")

    @property
    def usd(self) -> float:
        return (
            self.prompt_tokens / 1_000_000 * self.usd_per_m_prompt
            + self.completion_tokens / 1_000_000 * self.usd_per_m_completion
        )

    def snapshot(self) -> dict:
        return {
            "prompt_tokens": self.prompt_tokens,
            "completion_tokens": self.completion_tokens,
            "usd_estimated": round(self.usd, 4),
            "usd_per_m_prompt": self.usd_per_m_prompt,
            "usd_per_m_completion": self.usd_per_m_completion,
            "wall_hours": round((time.time() - self.started_at) / 3600.0, 3),
        }
```

- [ ] Implement `run_arm.py`.

```python
# bench/swebench/run_arm.py
"""Run one arm over the frozen instance list. Resumable, gated, budgeted."""

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


def raw_path(repo_root: Path, arm: str, instance_id: str) -> Path:
    return repo_root / "bench" / "results" / "swebench" / "raw" / arm / f"{instance_id}.json"


def run_arm(repo_root: Path, arm: str, cfg: agent.AgentConfig, budget: Budget) -> None:
    pr = preflight(repo_root)
    if arm not in pr.fields["arms"]:
        raise PreregError(f"arm {arm!r} is not pre-registered; arms are {pr.fields['arms']}")

    cache = Path.home() / ".cache" / "rtdd-bench"
    work = Path("/tmp/rtdd-bench") / arm
    ids = [
        line.strip()
        for line in (repo_root / "bench" / "swebench" / "instances.txt")
        .read_text(encoding="utf-8")
        .splitlines()
        if line.strip()
    ]
    rows = sample.load_rows()
    budget.start()

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
            extra, provenance = context_tools(arm, ws, cache, row)
            record.update(provenance)
            system_prompt = prompts.build(arm).format(
                repo_root=ws.root,
                problem_statement=row["problem_statement"],
            )
            result = agent.run_agent(ws, system_prompt, toolmod.base_tools(ws) + extra, cfg)
            budget.add(result.prompt_tokens, result.completion_tokens)
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
        except BudgetExceeded:
            raise
        except Exception as exc:
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
        out.write_text(json.dumps(record, indent=2), encoding="utf-8")
        print(
            f"[{index}/{len(ids)}] {arm} {instance_id} "
            f"{record['stop_reason']} patch={len(record['patch'])}B "
            f"${budget.usd:.2f}"
        )

    summary = repo_root / "bench" / "results" / "swebench" / f"budget-{arm}.json"
    summary.parent.mkdir(parents=True, exist_ok=True)
    summary.write_text(json.dumps(budget.snapshot(), indent=2), encoding="utf-8")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("arm", choices=list(prompts.ARMS))
    parser.add_argument("--base-url", default="http://127.0.0.1:8000/v1")
    parser.add_argument("--model", default="Qwen3-Coder-30B-A3B-Instruct")
    parser.add_argument("--max-usd", type=float, default=75.0)
    args = parser.parse_args()

    repo_root = Path(__file__).resolve().parents[2]
    cfg = agent.AgentConfig(base_url=args.base_url, model=args.model)
    try:
        run_arm(repo_root, args.arm, cfg, Budget(max_usd=args.max_usd))
    except PreregError as exc:
        print(f"PREFLIGHT REFUSED: {exc}", file=sys.stderr)
        return 3
    except BudgetExceeded as exc:
        print(f"BUDGET STOP: {exc}", file=sys.stderr)
        return 4
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
```

- [ ] Verify the gate actually blocks by temporarily breaking the tag, then commit.

```bash
cd /home/claude/rtdd/bench/swebench
git -C /home/claude/rtdd tag -d prereg-m4
uv run python run_arm.py vanilla; echo "exit=$?"
# expected: PREFLIGHT REFUSED: git rev-list ... failed: ...  / exit=3
git -C /home/claude/rtdd tag -a prereg-m4 -m "Axis 1 pre-registration signed" <signed-commit-sha>
uv run python run_arm.py vanilla --max-usd 0.01; echo "exit=$?"
# expected: BUDGET STOP: spend $... exceeded cap $0.01 / exit=4
```

```bash
git add bench/swebench/budget.py bench/swebench/run_arm.py
git commit -m "bench(swebench): resumable arm driver behind the preflight and budget gates

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

## Task 10 — Evaluation via the official SWE-bench Docker harness

**Files:**
- `bench/swebench/evaluate.py` (new)

**Interfaces:**
```python
def write_predictions(repo_root: Path, arm: str) -> Path
def run_evaluation(repo_root: Path, arm: str, workers: int) -> Path
def collect(repo_root: Path, arm: str) -> list[InstanceResult]
```

**Steps:**

- [ ] Implement.

```python
# bench/swebench/evaluate.py
"""Evaluate an arm's predictions with the official SWE-bench harness.

The harness is authoritative for pass/fail; this module only marshals inputs and
reads its report files back through metrics.from_report, which owns the
definition of a regression.
"""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
from pathlib import Path

import sample
from metrics import InstanceResult, from_report

RUN_ID_PREFIX = "rtdd-m4"


def results_dir(repo_root: Path) -> Path:
    return repo_root / "bench" / "results" / "swebench"


def write_predictions(repo_root: Path, arm: str) -> Path:
    raw_dir = results_dir(repo_root) / "raw" / arm
    out = results_dir(repo_root) / "predictions" / f"{arm}.jsonl"
    out.parent.mkdir(parents=True, exist_ok=True)
    lines = []
    for path in sorted(raw_dir.glob("*.json")):
        record = json.loads(path.read_text(encoding="utf-8"))
        lines.append(
            json.dumps(
                {
                    "instance_id": record["instance_id"],
                    "model_name_or_path": f"rtdd-{arm}",
                    "model_patch": record.get("patch", ""),
                }
            )
        )
    out.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return out


def run_evaluation(repo_root: Path, arm: str, workers: int = 8) -> Path:
    predictions = write_predictions(repo_root, arm)
    run_id = f"{RUN_ID_PREFIX}-{arm}"
    proc = subprocess.run(
        [
            sys.executable, "-m", "swebench.harness.run_evaluation",
            "--dataset_name", sample.DATASET,
            "--split", sample.SPLIT,
            "--predictions_path", str(predictions),
            "--max_workers", str(workers),
            "--run_id", run_id,
            "--cache_level", "env",
        ],
        cwd=results_dir(repo_root),
    )
    if proc.returncode != 0:
        raise RuntimeError(f"run_evaluation failed for arm {arm} (exit {proc.returncode})")
    return results_dir(repo_root) / "logs" / "run_evaluation" / run_id


def _report_for(log_root: Path, instance_id: str) -> dict:
    matches = list(log_root.rglob(f"{instance_id}/report.json"))
    if not matches:
        return {}
    return json.loads(matches[0].read_text(encoding="utf-8"))


def collect(repo_root: Path, arm: str) -> list[InstanceResult]:
    rows = sample.load_rows()
    raw_dir = results_dir(repo_root) / "raw" / arm
    log_root = results_dir(repo_root) / "logs" / "run_evaluation" / f"{RUN_ID_PREFIX}-{arm}"
    out_dir = results_dir(repo_root) / "reports" / arm
    out_dir.mkdir(parents=True, exist_ok=True)

    results: list[InstanceResult] = []
    for path in sorted(raw_dir.glob("*.json")):
        record = json.loads(path.read_text(encoding="utf-8"))
        instance_id = record["instance_id"]
        report = _report_for(log_root, instance_id)
        (out_dir / f"{instance_id}.json").write_text(
            json.dumps(report, indent=2), encoding="utf-8"
        )
        patch_applied = bool(record.get("patch", "").strip()) and bool(report)
        results.append(
            from_report(instance_id, arm, rows[instance_id], report, patch_applied)
        )
    return results


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("arm")
    parser.add_argument("--workers", type=int, default=8)
    args = parser.parse_args()
    repo_root = Path(__file__).resolve().parents[2]
    run_evaluation(repo_root, args.arm, args.workers)
    results = collect(repo_root, args.arm)
    print(f"{args.arm}: collected {len(results)} instance results")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
```

- [ ] Verify on a two-instance slice before committing 500 evaluations to it.

```bash
cd /home/claude/rtdd/bench/swebench && uv run python evaluate.py vanilla --workers 2
# expected: vanilla: collected 100 instance results
```

```bash
cd /home/claude/rtdd && git add bench/swebench/evaluate.py
git commit -m "bench(swebench): evaluate arms with the official SWE-bench Docker harness

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

## Task 11 — Analysis: rates, CIs, the equivalence band, the kill criterion

**Files:**
- `bench/swebench/analyze.py` (new)
- `bench/swebench/tests/test_analyze.py` (new)

**Interfaces:**
```python
TDAD_PUBLISHED = {"vanilla": 0.0608, "tdd": 0.0994, "tdad": 0.0182}

@dataclass(frozen=True)
class ArmSummary:
    arm: str; n: int
    test_level: float; test_ci: tuple[float, float]
    instance_level: float; instance_ci: tuple[float, float]
    resolution: float
    p2p_failed: int; p2p_total: int
    unapplied: int; harness_errors: int

def summarize(results) -> ArmSummary
def vanilla_reproduces(local: ArmSummary, k: float) -> tuple[bool, str]
def compare(a: ArmSummary, b: ArmSummary) -> dict     # delta + disjointness
def kill_criterion_met(repo_root: Path, floor: float) -> tuple[bool, float]
def analyze(repo_root: Path) -> dict
```

**Steps:**

- [ ] Failing test first.

```python
# bench/swebench/tests/test_analyze.py
import pytest

from analyze import ArmSummary, compare, vanilla_reproduces
from metrics import wilson_ci


def summary(arm, failed, total, n=100):
    return ArmSummary(
        arm=arm, n=n,
        test_level=failed / total, test_ci=wilson_ci(failed, total),
        instance_level=0.0, instance_ci=(0.0, 0.0),
        resolution=0.30, p2p_failed=failed, p2p_total=total,
        unapplied=0, harness_errors=0,
    )


def test_vanilla_at_the_published_rate_reproduces():
    ok, note = vanilla_reproduces(summary("vanilla", 608, 10000), k=2)
    assert ok
    assert "reproduces" in note


def test_vanilla_far_from_the_published_rate_diverges():
    ok, note = vanilla_reproduces(summary("vanilla", 2500, 10000), k=2)
    assert not ok
    assert "HARNESS DIVERGENCE" in note


def test_divergence_note_names_both_numbers():
    _, note = vanilla_reproduces(summary("vanilla", 2500, 10000), k=2)
    assert "0.0608" in note
    assert "0.2500" in note


def test_compare_reports_disjoint_intervals():
    out = compare(summary("tdad", 182, 10000), summary("rtdd", 90, 10000))
    assert out["delta"] == pytest.approx(0.0182 - 0.0090)
    assert out["ci_disjoint"] is True


def test_compare_reports_overlapping_intervals_as_a_tie():
    out = compare(summary("tdad", 182, 10000), summary("rtdd", 175, 10000))
    assert out["ci_disjoint"] is False
    assert out["verdict"] == "no distinguishable difference"
```

```bash
cd /home/claude/rtdd/bench/swebench && uv run pytest tests/test_analyze.py -q
# expected: ModuleNotFoundError: No module named 'analyze'
```

- [ ] Implement.

```python
# bench/swebench/analyze.py
"""Turn raw per-instance results into the published numbers.

Three rules are encoded rather than remembered:

  1. Both regression rates and the resolution rate are always produced together.
  2. TDAD's published figures are only usable if the local vanilla arm falls
     inside the pre-registered equivalence band around 6.08%. Otherwise the
     analysis emits HARNESS DIVERGENCE and the report drops the cross-citation.
  3. The kill criterion is read from the signed pre-registration and evaluated
     against M3's already-published Axis 2 stratified recall — it is never
     recomputed here, because recomputing it after M4 is exactly what
     pre-registration exists to prevent.
"""

from __future__ import annotations

import json
from dataclasses import asdict, dataclass
from pathlib import Path
from collections.abc import Sequence

import evaluate
import prompts
from metrics import (
    InstanceResult,
    instance_level_regression_rate,
    regression_counts,
    resolution_rate,
    test_level_regression_rate,
    wilson_ci,
)
from preflight import preflight

TDAD_PUBLISHED: dict[str, float] = {"vanilla": 0.0608, "tdd": 0.0994, "tdad": 0.0182}
TDAD_CITATION = "arXiv:2603.17973, n=100 SWE-bench Verified, Qwen3-Coder 30B Q4_K_M"


@dataclass(frozen=True)
class ArmSummary:
    arm: str
    n: int
    test_level: float
    test_ci: tuple[float, float]
    instance_level: float
    instance_ci: tuple[float, float]
    resolution: float
    p2p_failed: int
    p2p_total: int
    unapplied: int
    harness_errors: int


def summarize(results: Sequence[InstanceResult]) -> ArmSummary:
    failed, total = regression_counts(results)
    instance_hits = sum(1 for r in results if r.p2p_failed)
    return ArmSummary(
        arm=results[0].arm if results else "?",
        n=len(results),
        test_level=test_level_regression_rate(results),
        test_ci=wilson_ci(failed, total),
        instance_level=instance_level_regression_rate(results),
        instance_ci=wilson_ci(instance_hits, len(results) or 1),
        resolution=resolution_rate(results),
        p2p_failed=failed,
        p2p_total=total,
        unapplied=sum(1 for r in results if not r.patch_applied),
        harness_errors=sum(1 for r in results if r.patch_applied and not r.evaluated),
    )


def vanilla_reproduces(local: ArmSummary, k: float) -> tuple[bool, str]:
    published = TDAD_PUBLISHED["vanilla"]
    lo, hi = local.test_ci
    half = (hi - lo) / 2.0
    band = (local.test_level - k * half, local.test_level + k * half)
    if band[0] <= published <= band[1]:
        return (
            True,
            f"local vanilla {local.test_level:.4f} reproduces the published {published:.4f} "
            f"(band ±{k}×{half:.4f} = [{band[0]:.4f}, {band[1]:.4f}])",
        )
    return (
        False,
        f"HARNESS DIVERGENCE: local vanilla {local.test_level:.4f} excludes the published "
        f"{published:.4f} (band ±{k}×{half:.4f} = [{band[0]:.4f}, {band[1]:.4f}]). "
        "TDAD's published figures are removed from the comparison columns and every claim "
        "is stated in local-relative form only.",
    )


def compare(a: ArmSummary, b: ArmSummary) -> dict:
    disjoint = a.test_ci[0] > b.test_ci[1] or b.test_ci[0] > a.test_ci[1]
    if not disjoint:
        verdict = "no distinguishable difference"
    elif b.test_level < a.test_level:
        verdict = f"{b.arm} lower than {a.arm}"
    else:
        verdict = f"{a.arm} lower than {b.arm}"
    return {
        "a": a.arm,
        "b": b.arm,
        "delta": a.test_level - b.test_level,
        "a_ci": list(a.test_ci),
        "b_ci": list(b.test_ci),
        "ci_disjoint": disjoint,
        "verdict": verdict,
    }


def kill_criterion_met(repo_root: Path, floor: float) -> tuple[bool, float]:
    """Read M3's published single-killer stratified recall and compare to the floor."""
    path = repo_root / "bench" / "results" / "replay" / "summary.json"
    if not path.exists():
        raise FileNotFoundError(f"{path} — M3 must be published before M4 is analysed")
    m3 = json.loads(path.read_text(encoding="utf-8"))
    observed = float(m3["stratified"]["single_killer"]["change_level_recall"])
    return (observed >= floor, observed)


def analyze(repo_root: Path) -> dict:
    pr = preflight(repo_root)
    floor = float(pr.fields["stratified_recall_floor"])
    k = float(pr.fields["vanilla_equivalence_k"])

    summaries: dict[str, ArmSummary] = {}
    for arm in pr.fields["arms"]:
        results = evaluate.collect(repo_root, arm)
        if results:
            summaries[arm] = summarize(results)

    reproduces, note = vanilla_reproduces(summaries["vanilla"], k)
    met, observed = kill_criterion_met(repo_root, floor)

    out = {
        "prereg": dict(pr.fields),
        "arms": {name: asdict(s) for name, s in summaries.items()},
        "vanilla_equivalence": {"reproduces": reproduces, "note": note},
        "tdad_published": TDAD_PUBLISHED if reproduces else None,
        "tdad_citation": TDAD_CITATION,
        "kill_criterion": {
            "floor": floor,
            "observed_single_killer_recall": observed,
            "met": met,
        },
        "comparisons": [
            compare(summaries["vanilla"], summaries["rtdd"]),
            compare(summaries["tdd"], summaries["rtdd_tdd"]),
            compare(summaries["tdad"], summaries["rtdd_tdd"]),
            compare(summaries["tdad"], summaries["rtdd"]),
            compare(summaries["vanilla"], summaries["tdd"]),
        ],
    }
    dest = evaluate.results_dir(repo_root) / "summary.json"
    dest.write_text(json.dumps(out, indent=2), encoding="utf-8")
    return out


if __name__ == "__main__":
    root = Path(__file__).resolve().parents[2]
    result = analyze(root)
    print(result["vanilla_equivalence"]["note"])
    print(
        "kill criterion:",
        "MET" if result["kill_criterion"]["met"] else "NOT MET",
        f"({result['kill_criterion']['observed_single_killer_recall']:.4f} vs floor "
        f"{result['kill_criterion']['floor']:.4f})",
    )
    for c in result["comparisons"]:
        print(f"  {c['a']} vs {c['b']}: delta {c['delta']:+.4f}  {c['verdict']}")
```

- [ ] Green and commit.

```bash
cd /home/claude/rtdd/bench/swebench && uv run pytest tests/test_analyze.py -q
# expected: 5 passed
cd /home/claude/rtdd && git add bench/swebench/analyze.py bench/swebench/tests/test_analyze.py
git commit -m "bench(swebench): analysis with equivalence band and pre-registered kill criterion

TDAD's published figures are suppressed automatically when the local vanilla arm
falls outside the pre-registered band. The kill criterion is read from the signed
file and evaluated against M3's already-published recall, never recomputed.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

## Task 12 — Results tables

**Files:**
- `bench/swebench/report.py` (new)
- `bench/results/swebench/tables.md` (generated)
- `bench/results/swebench/config.json` (generated)

**Interfaces:**
```python
def render(summary: dict) -> str
def write_config(repo_root: Path) -> Path
```

**Steps:**

- [ ] Implement.

```python
# bench/swebench/report.py
"""Render summary.json into the tables that go in the README and docs/results.

Regression rate is never emitted without resolution rate in the same row, and a
divergent vanilla arm strips the published column rather than footnoting it.
"""

from __future__ import annotations

import json
import platform
import subprocess
from pathlib import Path

import evaluate

ARM_LABEL = {
    "vanilla": "vanilla",
    "tdd": "TDD procedural prose",
    "tdad": "TDAD static graph (+ prose)",
    "rtdd": "RTDD dynamic coverage",
    "rtdd_tdd": "RTDD dynamic coverage (+ prose)",
}


def render(summary: dict) -> str:
    reproduces = summary["vanilla_equivalence"]["reproduces"]
    published = summary["tdad_published"]
    lines: list[str] = []

    if not reproduces:
        lines += [
            "> **HARNESS DIVERGENCE**",
            ">",
            f"> {summary['vanilla_equivalence']['note']}",
            "",
        ]

    header = "| arm | regressions (test-level) | 95% CI | regressions (instance-level) | resolved |"
    sep = "|---|---|---|---|---|"
    if reproduces:
        header += " TDAD published |"
        sep += "---|"
    lines += [header, sep]

    for arm in ("vanilla", "tdd", "tdad", "rtdd", "rtdd_tdd"):
        s = summary["arms"].get(arm)
        if s is None:
            continue
        lo, hi = s["test_ci"]
        row = (
            f"| {ARM_LABEL[arm]} | {s['test_level'] * 100:.2f}% "
            f"| [{lo * 100:.2f}%, {hi * 100:.2f}%] "
            f"| {s['instance_level'] * 100:.2f}% "
            f"| {s['resolution'] * 100:.1f}% |"
        )
        if reproduces:
            cited = published.get(arm)
            row += f" {cited * 100:.2f}% |" if cited is not None else " — |"
        lines.append(row)

    lines += ["", "| arm | n | P2P failed / total | unapplied patches | harness errors |", "|---|---|---|---|---|"]
    for arm, s in summary["arms"].items():
        lines.append(
            f"| {ARM_LABEL.get(arm, arm)} | {s['n']} | {s['p2p_failed']} / {s['p2p_total']} "
            f"| {s['unapplied']} | {s['harness_errors']} |"
        )

    lines += ["", "## Pairwise", "", "| comparison | delta (test-level) | CIs disjoint | verdict |", "|---|---|---|---|"]
    for c in summary["comparisons"]:
        lines.append(
            f"| {ARM_LABEL.get(c['a'], c['a'])} vs {ARM_LABEL.get(c['b'], c['b'])} "
            f"| {c['delta'] * 100:+.2f} pp | {'yes' if c['ci_disjoint'] else 'no'} | {c['verdict']} |"
        )

    kc = summary["kill_criterion"]
    lines += [
        "",
        "## Pre-registered kill criterion",
        "",
        f"Floor (signed before the run): **{kc['floor']:.4f}** stratified change-level recall "
        "on the `|F_full| == 1` stratum.",
        f"Observed in M3: **{kc['observed_single_killer_recall']:.4f}**.",
        f"Result: **{'MET' if kc['met'] else 'NOT MET'}**.",
        "",
        f"TDAD figures, where shown, are cited from {summary['tdad_citation']}.",
    ]
    return "\n".join(lines) + "\n"


def write_config(repo_root: Path) -> Path:
    rtdd_sha = subprocess.run(
        ["git", "-C", str(repo_root), "rev-parse", "HEAD"],
        capture_output=True, text=True, check=True,
    ).stdout.strip()
    config = {
        "rtdd_commit": rtdd_sha,
        "hardware": {
            "platform": platform.platform(),
            "processor": platform.processor(),
            "python": platform.python_version(),
        },
        "note": "Wall-clock figures come from this machine, never from a CI runner (spec §10).",
    }
    dest = evaluate.results_dir(repo_root) / "config.json"
    dest.parent.mkdir(parents=True, exist_ok=True)
    dest.write_text(json.dumps(config, indent=2), encoding="utf-8")
    return dest


if __name__ == "__main__":
    root = Path(__file__).resolve().parents[2]
    summary = json.loads((evaluate.results_dir(root) / "summary.json").read_text(encoding="utf-8"))
    out = evaluate.results_dir(root) / "tables.md"
    out.write_text(render(summary), encoding="utf-8")
    write_config(root)
    print(f"wrote {out}")
```

- [ ] Commit.

```bash
cd /home/claude/rtdd && git add bench/swebench/report.py
git commit -m "bench(swebench): render results tables with divergence handling

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

## Task 13 — Run the benchmark

**Files:** none new — this task produces `bench/results/swebench/**`.

**Interfaces:** none.

### Cost and capacity, stated before the run

Pre-registered model: `Qwen3-Coder-30B-A3B-Instruct`, matching TDAD's Qwen3-Coder 30B
Q4_K_M so their published table stays a meaningful reference. Served locally with vLLM on
the disclosed benchmark machine; greedy decoding, 32768-token context, 4096 max output
tokens per turn, 40 turns per instance.

| item | quantity | estimate |
|---|---|---|
| agent runs | 5 arms × 100 instances | 500 |
| tokens per run | ~480k prompt processed, ~20k generated | — |
| local inference | prefill ~2000 tok/s, decode ~60 tok/s, 4-way concurrency | ~21 GPU-hours |
| RTDD seeding | ~100 instrumented full-suite runs, cached by (repo, base_commit), ~25 min each, 8-way | ~5.5 h |
| SWE-bench evaluation | 500 evaluations, ~4 min each, 8-way | ~4 h |
| total wall clock | one machine | **~1.5 days** |
| disk | SWE-bench env + instance images, mirrors, seeds | **~150 GB** |
| marginal cash cost, local | electricity only | **$0** |

Hosted fallback if the local GPU is unavailable: serve the same model from an
OpenAI-compatible provider. At the `budget.py` defaults of $0.10/M prompt and $0.40/M
completion, 500 runs cost `500 × (0.48 × 0.10 + 0.02 × 0.40) ≈ $28`. The cap is set to $75
to absorb retries. Confirm the provider's posted per-token price at launch and write both
the assumed and actual figures into `bench/results/swebench/cost.json` — the estimate above
is an assumption, and the run records what it actually was.

**Steps:**

- [ ] Confirm the gate is open and the model is serving.

```bash
cd /home/claude/rtdd/bench/swebench && uv run python preflight.py
curl -s http://127.0.0.1:8000/v1/models | head -c 200
# expected: preflight ok: floor=<signed> n=100 seed=20260826
# expected: {"object":"list","data":[{"id":"Qwen3-Coder-30B-A3B-Instruct",...
```

- [ ] Pre-seed (Task 8's seeding step) if not already done, and record `seeded` /
      `seed_failed` counts into `bench/results/swebench/config.json`.

- [ ] Run all five arms. They are resumable; a crash costs the current instance only.

```bash
cd /home/claude/rtdd/bench/swebench
for arm in vanilla tdd tdad rtdd rtdd_tdd; do
  uv run python run_arm.py "$arm" --max-usd 75 2>&1 | tee "/tmp/rtdd-m4-$arm.log"
done
# expected tail per arm: [100/100] <arm> <instance> finish patch=1842B $5.60
```

- [ ] Evaluate all five arms.

```bash
cd /home/claude/rtdd/bench/swebench
for arm in vanilla tdd tdad rtdd rtdd_tdd; do
  uv run python evaluate.py "$arm" --workers 8
done
# expected per arm: <arm>: collected 100 instance results
```

- [ ] Analyse and render.

```bash
cd /home/claude/rtdd/bench/swebench
uv run python analyze.py
uv run python report.py
cat /home/claude/rtdd/bench/results/swebench/tables.md
# expected shape:
#   local vanilla 0.0591 reproduces the published 0.0608 (band ±2×0.0021 = [0.0549, 0.0633])
#   kill criterion: MET (0.9640 vs floor 0.9500)
#     vanilla vs rtdd: delta +0.0374  rtdd lower than vanilla
#     ...
```

- [ ] Sanity-check the guarantee held in the actual run: the rendered arm prompts must still
      satisfy the byte-equality property recorded in the raw results.

```bash
cd /home/claude/rtdd/bench/swebench && uv run pytest tests/test_prompts.py -q
uv run python - <<'EOF'
import json, pathlib, prompts
raw = pathlib.Path("../results/swebench/raw")
for arm in ("vanilla", "rtdd"):
    n = len(list((raw/arm).glob("*.json")))
    print(arm, n)
assert prompts.build("rtdd") == prompts.build("vanilla") + "\n" + prompts.context_block(prompts.RTDD_CONTEXT)
assert prompts.lint_context(prompts.RTDD_CONTEXT) == []
print("non-procedural guarantee holds")
EOF
# expected: ... / non-procedural guarantee holds
```

- [ ] Commit the results. Raw transcripts are large; commit the reports and summaries and
      publish the transcripts as a release asset.

```bash
cd /home/claude/rtdd
cat > bench/results/swebench/.gitignore <<'EOF'
logs/
raw/*/transcript-*.json
EOF
git add bench/results/swebench
git commit -m "bench(swebench): Axis 1 results, five arms, one harness

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

## Task 14 — HUMAN-IN-THE-LOOP: go/no-go on publishing

> ## 🛑 STOP — HUMAN-IN-THE-LOOP
>
> **An autonomous agent MUST NOT decide this.** The agent's job here is to print the outcome
> and the matching row of the Ship / Don't ship table, then stop. Spec §15 commits to
> publishing the result either way; what changes with the outcome is *what the README says*
> and *what ships*, not *whether it is published*.

**Files:**
- `docs/results/2026-XX-XX-swebench-verified.md` (new — the standalone results write-up)

**Interfaces:** none.

**Steps:**

- [ ] Agent: write the standalone results document, outcome-neutral.

```bash
cd /home/claude/rtdd
{
  echo "# Axis 1 — SWE-bench Verified, five arms on one harness"
  echo
  echo "Pre-registered at \`bench/PREREGISTRATION.md\`, tag \`prereg-m4\`."
  echo "Sample: 100 instances of SWE-bench Verified, seed 20260826, sha256 in the pre-registration."
  echo "Model: Qwen3-Coder-30B-A3B-Instruct, greedy, 40 turns, one model for every arm."
  echo
  cat bench/results/swebench/tables.md
} > "docs/results/$(date +%Y-%m-%d)-swebench-verified.md"
```

- [ ] Agent: print exactly this to the human and **stop**.

```
DECISION REQUIRED — go/no-go on publishing (spec §15)

Kill criterion: <MET | NOT MET>   (observed <x> vs signed floor <y>)
Vanilla equivalence: <reproduces | HARNESS DIVERGENCE>
Primary contrast   (tdad vs rtdd_tdd): delta <±n.nn> pp, CIs <disjoint | overlapping>
Isolation contrast (vanilla vs rtdd):  delta <±n.nn> pp, CIs <disjoint | overlapping>

Matching row of the Ship / Don't ship table: <row name>
That row says: <action>

Spec §15 commits to publishing this result whichever way it fell. What needs your decision
is the ship action, not the publication. Confirm the row, or tell me you read it differently.
```

- [ ] Human: confirm the row. That confirmation selects between Task 22 (positive README) and
      Task 23 (negative README), and feeds Task 24.

- [ ] Commit the results document.

```bash
cd /home/claude/rtdd && git add docs/results
git commit -m "docs(results): publish the Axis 1 SWE-bench Verified result

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

# M5 — Protocol, generated front-ends, release

## Task 15 — `protocol/PROTOCOL.md` and its parser

One source, but the source carries **per-target variants**, because one body cannot serve
all three targets well: a Claude Code `SKILL.md` is progressively disclosed and can be long,
`AGENTS.md` sits in context on every turn and must be short, and a Cursor `.mdc` needs
frontmatter with globs. Naive templating of one body into three wrappers would give two of
the three the wrong content while the drift check passed.

**Files:**
- `protocol/PROTOCOL.md` (new)
- `internal/protocol/parse.go` (new)
- `internal/protocol/parse_test.go` (new)

**Interfaces:**
```go
package protocol

type Section struct {
    ID       string
    Title    string
    Targets  []string          // "skill", "agents", "mdc"
    Order    int
    Body     string            // the default body, trimmed
    Variants map[string]string // target -> replacement body
}

type Doc struct {
    Version  int
    Sections []Section
}

func Parse(src string) (*Doc, error)
func (d *Doc) For(target string) []Section      // ordered by Order, then ID
func (s Section) BodyFor(target string) string  // Variants[target] if present, else Body
```

**Steps:**

- [ ] Failing test first.

```go
// internal/protocol/parse_test.go
package protocol

import "testing"

const sample = `# title

<!-- rtdd:meta version=1 -->

<!-- rtdd:section id=b title="Second" targets=skill,agents order=20 -->
long body for b
<!-- rtdd:variant target=agents -->
short b
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=a title="First" targets=skill order=10 -->
body for a
<!-- rtdd:endsection -->
`

func TestParseReadsMetaAndSections(t *testing.T) {
	d, err := Parse(sample)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if d.Version != 1 {
		t.Fatalf("Version = %d, want 1", d.Version)
	}
	if len(d.Sections) != 2 {
		t.Fatalf("len(Sections) = %d, want 2", len(d.Sections))
	}
}

func TestForOrdersByOrderNotFilePosition(t *testing.T) {
	d, _ := Parse(sample)
	got := d.For("skill")
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("For(skill) = %v, want [a b]", ids(got))
	}
}

func TestForFiltersByTarget(t *testing.T) {
	d, _ := Parse(sample)
	if got := ids(d.For("agents")); len(got) != 1 || got[0] != "b" {
		t.Fatalf("For(agents) = %v, want [b]", got)
	}
	if got := ids(d.For("mdc")); len(got) != 0 {
		t.Fatalf("For(mdc) = %v, want []", got)
	}
}

func TestBodyForPrefersTheVariant(t *testing.T) {
	d, _ := Parse(sample)
	b := d.For("skill")[1]
	if b.BodyFor("skill") != "long body for b" {
		t.Fatalf("skill body = %q", b.BodyFor("skill"))
	}
	if b.BodyFor("agents") != "short b" {
		t.Fatalf("agents body = %q", b.BodyFor("agents"))
	}
}

func TestTitleIsRequired(t *testing.T) {
	_, err := Parse("<!-- rtdd:meta version=1 -->\n<!-- rtdd:section id=x targets=skill order=1 -->\nb\n<!-- rtdd:endsection -->\n")
	if err == nil {
		t.Fatal("want error for a section with no title")
	}
}

func TestUnterminatedSectionIsAnError(t *testing.T) {
	_, err := Parse("<!-- rtdd:meta version=1 -->\n<!-- rtdd:section id=x title=\"X\" targets=skill order=1 -->\nbody\n")
	if err == nil {
		t.Fatal("want error for an unterminated section")
	}
}

func TestDuplicateIDIsAnError(t *testing.T) {
	src := "<!-- rtdd:meta version=1 -->\n" +
		"<!-- rtdd:section id=x title=\"X\" targets=skill order=1 -->\na\n<!-- rtdd:endsection -->\n" +
		"<!-- rtdd:section id=x title=\"X\" targets=skill order=2 -->\nb\n<!-- rtdd:endsection -->\n"
	if _, err := Parse(src); err == nil {
		t.Fatal("want error for a duplicate section id")
	}
}

func TestUnknownTargetIsAnError(t *testing.T) {
	src := "<!-- rtdd:meta version=1 -->\n<!-- rtdd:section id=x title=\"X\" targets=vscode order=1 -->\nb\n<!-- rtdd:endsection -->\n"
	if _, err := Parse(src); err == nil {
		t.Fatal("want error for an unknown target")
	}
}

func ids(secs []Section) []string {
	out := make([]string, 0, len(secs))
	for _, s := range secs {
		out = append(out, s.ID)
	}
	return out
}
```

```bash
go test ./internal/protocol/
# expected: no Go files in .../internal/protocol
```

- [ ] Implement the parser.

```go
// internal/protocol/parse.go

// Package protocol parses protocol/PROTOCOL.md, the single source for every
// generated agent front-end, and renders it per target.
//
// The source carries per-target body variants because one body cannot serve all
// three targets well: a Claude Code SKILL.md is progressively disclosed and can
// be long, an AGENTS.md is always in context and must be short, and a Cursor
// .mdc needs glob frontmatter. Templating one body into three wrappers would
// leave two of the three wrong while a byte-level drift check still passed.
package protocol

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// KnownTargets is closed on purpose: a typo in a targets= list must fail the
// build rather than silently drop a section from one front-end.
var KnownTargets = map[string]bool{"skill": true, "agents": true, "mdc": true}

type Section struct {
	ID       string
	Title    string
	Targets  []string
	Order    int
	Body     string
	Variants map[string]string
}

type Doc struct {
	Version  int
	Sections []Section
}

func (s Section) BodyFor(target string) string {
	if v, ok := s.Variants[target]; ok {
		return v
	}
	return s.Body
}

func (s Section) HasTarget(target string) bool {
	for _, t := range s.Targets {
		if t == target {
			return true
		}
	}
	return false
}

func (d *Doc) For(target string) []Section {
	out := make([]Section, 0, len(d.Sections))
	for _, s := range d.Sections {
		if s.HasTarget(target) {
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Order != out[j].Order {
			return out[i].Order < out[j].Order
		}
		return out[i].ID < out[j].ID
	})
	return out
}

const (
	metaPrefix       = "<!-- rtdd:meta "
	sectionPrefix    = "<!-- rtdd:section "
	sectionEnd       = "<!-- rtdd:endsection -->"
	variantPrefix    = "<!-- rtdd:variant "
	variantEnd       = "<!-- rtdd:endvariant -->"
	directiveSuffix  = " -->"
)

// attrs parses `key=value key="quoted value"` into a map.
func attrs(s string) (map[string]string, error) {
	out := map[string]string{}
	rest := strings.TrimSpace(s)
	for rest != "" {
		eq := strings.Index(rest, "=")
		if eq < 0 {
			return nil, fmt.Errorf("attribute without '=': %q", rest)
		}
		key := strings.TrimSpace(rest[:eq])
		rest = rest[eq+1:]
		var value string
		if strings.HasPrefix(rest, `"`) {
			end := strings.Index(rest[1:], `"`)
			if end < 0 {
				return nil, fmt.Errorf("unterminated quote in attribute %q", key)
			}
			value = rest[1 : 1+end]
			rest = rest[2+end:]
		} else {
			sp := strings.IndexAny(rest, " \t")
			if sp < 0 {
				value, rest = rest, ""
			} else {
				value, rest = rest[:sp], rest[sp:]
			}
		}
		out[key] = value
		rest = strings.TrimSpace(rest)
	}
	return out, nil
}

func directive(line, prefix string) (string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, prefix) || !strings.HasSuffix(line, directiveSuffix) {
		return "", false
	}
	return line[len(prefix) : len(line)-len(directiveSuffix)], true
}

func Parse(src string) (*Doc, error) {
	lines := strings.Split(src, "\n")
	doc := &Doc{}
	seen := map[string]bool{}

	for i := 0; i < len(lines); i++ {
		if raw, ok := directive(lines[i], metaPrefix); ok {
			a, err := attrs(raw)
			if err != nil {
				return nil, fmt.Errorf("line %d: %w", i+1, err)
			}
			v, err := strconv.Atoi(a["version"])
			if err != nil {
				return nil, fmt.Errorf("line %d: meta version must be an integer", i+1)
			}
			doc.Version = v
			continue
		}

		raw, ok := directive(lines[i], sectionPrefix)
		if !ok {
			continue
		}
		a, err := attrs(raw)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", i+1, err)
		}
		sec := Section{ID: a["id"], Title: a["title"], Variants: map[string]string{}}
		if sec.ID == "" {
			return nil, fmt.Errorf("line %d: section has no id", i+1)
		}
		if sec.Title == "" {
			return nil, fmt.Errorf("line %d: section %q has no title", i+1, sec.ID)
		}
		if seen[sec.ID] {
			return nil, fmt.Errorf("line %d: duplicate section id %q", i+1, sec.ID)
		}
		seen[sec.ID] = true
		if a["targets"] == "" {
			return nil, fmt.Errorf("line %d: section %q has no targets", i+1, sec.ID)
		}
		for _, t := range strings.Split(a["targets"], ",") {
			t = strings.TrimSpace(t)
			if !KnownTargets[t] {
				return nil, fmt.Errorf("line %d: section %q: unknown target %q", i+1, sec.ID, t)
			}
			sec.Targets = append(sec.Targets, t)
		}
		sec.Order, err = strconv.Atoi(a["order"])
		if err != nil {
			return nil, fmt.Errorf("line %d: section %q: order must be an integer", i+1, sec.ID)
		}

		var body []string
		closed := false
		for i++; i < len(lines); i++ {
			line := strings.TrimSpace(lines[i])
			if line == sectionEnd {
				closed = true
				break
			}
			if vraw, ok := directive(lines[i], variantPrefix); ok {
				va, err := attrs(vraw)
				if err != nil {
					return nil, fmt.Errorf("line %d: %w", i+1, err)
				}
				target := va["target"]
				if !KnownTargets[target] {
					return nil, fmt.Errorf("line %d: section %q: unknown variant target %q", i+1, sec.ID, target)
				}
				var vbody []string
				vclosed := false
				for i++; i < len(lines); i++ {
					if strings.TrimSpace(lines[i]) == variantEnd {
						vclosed = true
						break
					}
					vbody = append(vbody, lines[i])
				}
				if !vclosed {
					return nil, fmt.Errorf("section %q: unterminated variant for %q", sec.ID, target)
				}
				sec.Variants[target] = strings.TrimSpace(strings.Join(vbody, "\n"))
				continue
			}
			body = append(body, lines[i])
		}
		if !closed {
			return nil, fmt.Errorf("section %q: unterminated (missing %s)", sec.ID, sectionEnd)
		}
		sec.Body = strings.TrimSpace(strings.Join(body, "\n"))
		if sec.Body == "" {
			return nil, fmt.Errorf("section %q: empty body", sec.ID)
		}
		doc.Sections = append(doc.Sections, sec)
	}

	if doc.Version == 0 {
		return nil, fmt.Errorf("missing %sversion=N%s", metaPrefix, directiveSuffix)
	}
	if len(doc.Sections) == 0 {
		return nil, fmt.Errorf("no sections found")
	}
	return doc, nil
}
```

- [ ] Write the actual protocol source.

```bash
mkdir -p /home/claude/rtdd/protocol
cat > /home/claude/rtdd/protocol/PROTOCOL.md <<'EOF'
# RTDD agent protocol

This file is the single source for every generated agent front-end. Edit it, then run
`rtdd-gen render`. Do not edit anything under `dist/`.

<!-- rtdd:meta version=1 -->

<!-- rtdd:section id=what title="What rtdd reports" targets=skill,agents,mdc order=10 -->
`rtdd` reports which tests cover the code you just changed, and which of the lines you just
changed nothing covers. Both come from coverage recorded during real execution of this
repository's suite, not from a static call graph, so the relation includes edges reached
through dynamic dispatch, dependency injection, plugin registries, and monkeypatching, and
omits any path no test has ever taken.

It reports. It does not gate, block, or fail anything on policy.
<!-- rtdd:variant target=agents -->
`rtdd` reports which tests cover code you changed, and which changed lines nothing covers,
from recorded coverage rather than a static graph. It reports; it never gates.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=which title="rtdd which" targets=skill,agents,mdc order=20 -->
```
rtdd which [--base <ref>] [--json]
```

Prints the ranked selection for the current changed set and the uncovered report, and runs
nothing. It costs one map lookup and one `git diff`, so it is cheap to consult often.

The changed set is the union of `git diff --name-only <base>` and
`git status --porcelain -uall`, so untracked files count — a file you just wrote is in the
changed set before you commit it.

Ranking is by descending share of each test's recorded file set that your change touches,
then last-failed first, then smallest file set, then fastest.
<!-- rtdd:variant target=agents -->
`rtdd which` prints the ranked tests covering your current changes plus the uncovered
report, and runs nothing. Untracked files count, so a file you just wrote is included.
`rtdd which --json` is the machine-readable form.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=run title="rtdd run" targets=skill,agents,mdc order=30 -->
```
rtdd run [--base <ref>] [--fail-fast] [--json]
```

Runs the selection, refreshes the map rows for the tests it executed, and prints the
uncovered report computed from fresh post-run coverage. It exits non-zero only when a test
fails. An empty selection and a non-empty uncovered report are both exit 0.

`--fail-fast` is opt-in and never implied.
<!-- rtdd:variant target=agents -->
`rtdd run` runs the selection and prints the uncovered report. Exit non-zero means a test
failed, and nothing else — an empty selection and an uncovered report are both exit 0.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=uncovered title="The uncovered report" targets=skill,agents,mdc order=40 -->
The report classifies your changed lines into three classes, and the distinction matters:

- **covered** — an executing test touched these changed lines.
- **uncovered** — no test executed them.
- **import-time** — executed during collection and attributed to no test. Reported
  separately and never counted as uncovered. Dataclasses, enums, config modules, ORM model
  definitions, route decorators, and `__init__.py` re-exports land here routinely while
  being correctly tested.

An uncovered range is information about the suite, not a verdict on the patch.
<!-- rtdd:variant target=agents -->
The uncovered report splits changed lines into covered, uncovered, and import-time.
Import-time lines execute during collection and are attributed to no test — they are
reported separately and are not a coverage gap.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=empty title="An empty selection is not green" targets=skill,agents,mdc order=50 -->
When nothing is selected, `rtdd` says so explicitly. An empty selection is a distinct
outcome from "all selected tests passed", because every under-selection path terminates
there. Treat it as "the map has nothing to say about this change", not as a pass.
<!-- rtdd:variant target=agents -->
An empty selection is reported as its own outcome, never as a pass.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=json title="JSON output" targets=skill order=60 -->
`--json` emits one object for programmatic consumption:

```json
{
  "tier": "T0",
  "reason": "changed files intersect 12 recorded test rows",
  "base": "HEAD",
  "changed": ["src/auth.py", "src/db.py"],
  "direct": ["tests/test_auth.py"],
  "tests": ["tests/test_auth.py::test_login", "tests/test_db.py::test_pool"],
  "uncovered": [{"path": "src/auth.py", "ranges": [{"start": 52, "end": 58}]}],
  "import_time": [{"path": "src/constants.py", "ranges": [{"start": 1, "end": 12}]}],
  "selected_duration_ms": 1412,
  "map_tests": 8471
}
```

`tier` is one of `empty`, `direct`, `T0`, `T1`, `T2`. `T2` means the full suite was selected
because the map is unseeded, a dependency manifest changed, the test-harness config changed,
or the drift guard was reached; `reason` says which.
<!-- rtdd:endsection -->

<!-- rtdd:section id=commands title="The rest of the commands" targets=skill order=70 -->
```
rtdd status                  adapter, map freshness, seed state
rtdd seed                    one full instrumented run; the only op that may shrink a row
rtdd verify                  full suite
rtdd doctor                  fan-out / coupling report
rtdd explain <file>          which tests cover this file
rtdd map compact             collapse duplicate rows after a union merge
rtdd init                    install .gitattributes, config, and agent front-ends
```

`rtdd seed` is the only operation that may narrow a test's recorded file set. Every other
path unions, because a subset run legitimately records less coverage than a full run — an
import-time line migrates to whichever test ran first, and a failing test records only a
truncated prefix of its real path.
<!-- rtdd:endsection -->

<!-- rtdd:section id=limits title="What it cannot see" targets=skill,mdc order=80 -->
Stated plainly, because a selector that hides its blind spots is worse than no selector:

- Coverage only knows paths some test actually took. A branch nothing has ever exercised has
  no edge, and `rtdd which` will not find it.
- Anything executed once per process — `@lru_cache`, module singletons, DI containers,
  session-scoped fixtures — is attributed to whichever test happened to run first, so the
  most coupled file in a repo can appear as its cleanest in `rtdd doctor`.
- Selection is file-level. Changing one function in a file selects every test that touched
  any part of that file.
- A merge commit escalates, because a union-merged map cannot narrow relative to its parents
  but can still be stale relative to the merged code.
- `rtdd verify` is a convenience, not a substitute for CI.
<!-- rtdd:variant target=mdc -->
Coverage only knows paths some test actually took; a never-exercised branch has no edge.
Once-per-process execution (`@lru_cache`, singletons, session fixtures) is attributed to
whichever test ran first. Selection is file-level. `rtdd verify` does not replace CI.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=map title="The map file" targets=skill order=90 -->
`.rtdd/map.jsonl` is committed, sorted by test id, one line per test, file-level only:

```
{"t":"tests/test_auth.py::test_login","f":["src/auth.py","src/db.py"],"c":"a3f21e0","d":412,"s":"pass"}
```

`rtdd init` installs `.rtdd/map.jsonl merge=union` into `.gitattributes`. Two agents editing
different tests merge cleanly; two editing the same test leave two lines, which `rtdd map
compact` collapses by set-union of `f`. Committing the map is what lets a fresh worktree
inherit it and pay no seed cost.

Line-level coverage is never persisted. It is recomputed after each run and used
immediately, which is also why the uncovered report never suffers line drift.
<!-- rtdd:endsection -->
EOF
```

- [ ] Green and commit.

```bash
cd /home/claude/rtdd && go test ./internal/protocol/
# expected: ok  github.com/VocanicZ/rtdd/internal/protocol
git add protocol internal/protocol
git commit -m "protocol: single source with per-target body variants, and its parser

One body cannot serve SKILL.md, AGENTS.md, and .mdc well, so the source carries
per-target variants and the parser resolves them. Unknown targets and duplicate
ids fail the parse rather than silently dropping a section.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

## Task 16 — Per-target renderers with byte budgets

**Files:**
- `internal/protocol/targets.go` (new)
- `internal/protocol/render.go` (new)

**Interfaces:**
```go
type Target struct {
    Name     string
    OutPath  string
    MaxBytes int
    Required []string
    Render   func(d *Doc) (string, error)
    Validate func(out string) error
}

var Targets []Target
func TargetByName(name string) (Target, bool)
func RenderAll(d *Doc) (map[string]string, error) // OutPath -> content
var ErrOverBudget = errors.New("rendered output exceeds the target's byte budget")
```

**Steps:**

- [ ] Implement `targets.go`.

```go
// internal/protocol/targets.go
package protocol

const (
	// A Claude Code skill is progressively disclosed: the agent loads it on
	// demand, so length costs little and completeness is worth more.
	skillMaxBytes = 20000
	// AGENTS.md is in context on every turn of every task, most of which have
	// nothing to do with tests. It must stay small enough that its presence is
	// not itself a cost.
	agentsMaxBytes = 1800
	// A Cursor rule is attached when its globs match, so it is between the two.
	mdcMaxBytes = 4000
)

const (
	SkillDescription = "Surface which tests cover the code you changed, and which changed " +
		"lines nothing covers, from recorded coverage rather than a static graph. Use when " +
		"editing a Python repository that has a .rtdd/map.jsonl, before or after changing " +
		"source files, to find the relevant tests and the untested part of a diff."
	MdcDescription = "Which tests cover the code you changed, from recorded coverage."
	MdcGlobs       = "**/*.py"
)

var Targets = []Target{
	{
		Name:     "skill",
		OutPath:  "dist/SKILL.md",
		MaxBytes: skillMaxBytes,
		Required: []string{"what", "which", "run", "uncovered", "empty", "json", "commands", "limits", "map"},
		Render:   renderSkill,
		Validate: validateSkill,
	},
	{
		Name:     "agents",
		OutPath:  "dist/AGENTS.md",
		MaxBytes: agentsMaxBytes,
		Required: []string{"what", "which", "run", "uncovered", "empty"},
		Render:   renderAgents,
		Validate: validateAgents,
	},
	{
		Name:     "mdc",
		OutPath:  "dist/cursor/rules/rtdd.mdc",
		MaxBytes: mdcMaxBytes,
		Required: []string{"what", "which", "run", "uncovered", "empty", "limits"},
		Render:   renderMDC,
		Validate: validateMDC,
	},
}

func TargetByName(name string) (Target, bool) {
	for _, t := range Targets {
		if t.Name == name {
			return t, true
		}
	}
	return Target{}, false
}
```

- [ ] Implement `render.go`.

```go
// internal/protocol/render.go
package protocol

import (
	"errors"
	"fmt"
	"strings"
)

var ErrOverBudget = errors.New("rendered output exceeds the target's byte budget")

type Target struct {
	Name     string
	OutPath  string
	MaxBytes int
	Required []string
	Render   func(d *Doc) (string, error)
	Validate func(out string) error
}

// Generated marks every output so a reader knows where to edit.
const Generated = "<!-- GENERATED by `rtdd-gen render` from protocol/PROTOCOL.md. Do not edit. -->"

// AGENTS.md and CLAUDE.md are host-owned files, so the block that goes into them
// is delimited and `rtdd init` only ever touches what is between these markers.
const (
	BeginMarker = "<!-- BEGIN rtdd (generated from protocol/PROTOCOL.md; do not edit here) -->"
	EndMarker   = "<!-- END rtdd -->"
)

func requireSections(d *Doc, t Target) error {
	present := map[string]bool{}
	for _, s := range d.For(t.Name) {
		present[s.ID] = true
	}
	var missing []string
	for _, id := range t.Required {
		if !present[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("target %q is missing required sections %v", t.Name, missing)
	}
	return nil
}

func renderSkill(d *Doc) (string, error) {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("name: rtdd\n")
	b.WriteString("description: " + SkillDescription + "\n")
	b.WriteString("---\n\n")
	b.WriteString(Generated + "\n\n")
	b.WriteString("# rtdd\n\n")
	for _, s := range d.For("skill") {
		b.WriteString("## " + s.Title + "\n\n")
		b.WriteString(s.BodyFor("skill") + "\n\n")
	}
	return strings.TrimRight(b.String(), "\n") + "\n", nil
}

func renderAgents(d *Doc) (string, error) {
	var b strings.Builder
	b.WriteString(BeginMarker + "\n")
	b.WriteString("## rtdd\n\n")
	for _, s := range d.For("agents") {
		b.WriteString(s.BodyFor("agents") + "\n\n")
	}
	b.WriteString(EndMarker + "\n")
	return b.String(), nil
}

func renderMDC(d *Doc) (string, error) {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("description: " + MdcDescription + "\n")
	b.WriteString("globs: " + MdcGlobs + "\n")
	b.WriteString("alwaysApply: false\n")
	b.WriteString("---\n\n")
	b.WriteString(Generated + "\n\n")
	b.WriteString("# rtdd\n\n")
	for _, s := range d.For("mdc") {
		b.WriteString("## " + s.Title + "\n\n")
		b.WriteString(s.BodyFor("mdc") + "\n\n")
	}
	return strings.TrimRight(b.String(), "\n") + "\n", nil
}

// RenderAll renders every target, enforcing required sections, byte budgets, and
// per-target validity. A budget overflow is an error, never a truncation: a
// silently truncated AGENTS.md would pass a byte-comparison drift check while
// being wrong.
func RenderAll(d *Doc) (map[string]string, error) {
	out := make(map[string]string, len(Targets))
	for _, t := range Targets {
		if err := requireSections(d, t); err != nil {
			return nil, err
		}
		body, err := t.Render(d)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", t.Name, err)
		}
		if len(body) > t.MaxBytes {
			return nil, fmt.Errorf(
				"%w: %s is %d bytes, budget %d — shorten the target's variants, do not raise the budget",
				ErrOverBudget, t.OutPath, len(body), t.MaxBytes)
		}
		if err := t.Validate(body); err != nil {
			return nil, fmt.Errorf("validate %s: %w", t.OutPath, err)
		}
		out[t.OutPath] = body
	}
	return out, nil
}
```

- [ ] Commit after Task 17 makes it compile (the `Validate` funcs live there).

---

## Task 17 — Per-target validity assertions and golden files

> The failure mode this task exists to prevent: a drift check that compares bytes and passes,
> while `AGENTS.md` has swallowed the whole skill body and the `.mdc` has lost its
> frontmatter. Byte-comparison alone cannot see that. These assertions can.

**Files:**
- `internal/protocol/validate.go` (new)
- `internal/protocol/protocol_test.go` (new)
- `internal/protocol/testdata/golden/SKILL.md` (generated)
- `internal/protocol/testdata/golden/AGENTS.md` (generated)
- `internal/protocol/testdata/golden/rtdd.mdc` (generated)

**Interfaces:**
```go
func validateSkill(out string) error
func validateAgents(out string) error
func validateMDC(out string) error
```

**Steps:**

- [ ] Implement `validate.go`.

```go
// internal/protocol/validate.go
package protocol

import (
	"fmt"
	"strings"
)

// mustContain is the shared shape of every per-target assertion: the content the
// front-end is useless without.
func mustContain(out string, needles ...string) error {
	for _, n := range needles {
		if !strings.Contains(out, n) {
			return fmt.Errorf("missing required content %q", n)
		}
	}
	return nil
}

func mustNotContain(out string, needles ...string) error {
	for _, n := range needles {
		if strings.Contains(out, n) {
			return fmt.Errorf("contains content that belongs to another target: %q", n)
		}
	}
	return nil
}

func validateSkill(out string) error {
	if !strings.HasPrefix(out, "---\nname: rtdd\n") {
		return fmt.Errorf("SKILL.md must open with YAML frontmatter starting `name: rtdd`")
	}
	if !strings.Contains(out, "\ndescription: ") {
		return fmt.Errorf("SKILL.md frontmatter has no description")
	}
	end := strings.Index(out[4:], "\n---\n")
	if end < 0 {
		return fmt.Errorf("SKILL.md frontmatter is unterminated")
	}
	if err := mustContain(out, Generated, "rtdd which", "rtdd run", "--json", "import-time"); err != nil {
		return err
	}
	// Progressive disclosure earns its length; a skill this short lost sections.
	if len(out) < 3000 {
		return fmt.Errorf("SKILL.md is %d bytes — too short to carry the full protocol", len(out))
	}
	return nil
}

func validateAgents(out string) error {
	if !strings.HasPrefix(out, BeginMarker) {
		return fmt.Errorf("AGENTS.md block must open with the BEGIN marker so `rtdd init` can merge it")
	}
	if !strings.HasSuffix(out, EndMarker+"\n") {
		return fmt.Errorf("AGENTS.md block must close with the END marker")
	}
	if err := mustContain(out, "rtdd which", "rtdd run"); err != nil {
		return err
	}
	// AGENTS.md is always in context. These belong only to the long-form targets;
	// their presence means the skill body leaked in.
	if err := mustNotContain(out, "name: rtdd", "alwaysApply", "map.jsonl", "rtdd doctor"); err != nil {
		return err
	}
	if strings.Count(out, "\n## ") > 1 {
		return fmt.Errorf("AGENTS.md block has more than one heading — it must read as one short block")
	}
	return nil
}

func validateMDC(out string) error {
	if !strings.HasPrefix(out, "---\n") {
		return fmt.Errorf(".mdc must open with frontmatter")
	}
	if err := mustContain(out,
		"\ndescription: ", "\nglobs: ", "\nalwaysApply: false\n", Generated, "rtdd which",
	); err != nil {
		return err
	}
	if strings.Contains(out, "\nalwaysApply: true\n") {
		return fmt.Errorf(".mdc must not set alwaysApply: true — it is a glob-scoped rule")
	}
	// The .mdc is not the skill: JSON schema and map internals belong in SKILL.md.
	return mustNotContain(out, "name: rtdd", "map_tests", "rtdd map compact")
}
```

- [ ] Write the tests that make a wrong-but-stale-free output fail.

```go
// internal/protocol/protocol_test.go
package protocol

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func load(t *testing.T) *Doc {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("..", "..", "protocol", "PROTOCOL.md"))
	if err != nil {
		t.Fatalf("read PROTOCOL.md: %v", err)
	}
	d, err := Parse(string(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return d
}

func TestRenderAllSucceedsOnTheRealProtocol(t *testing.T) {
	if _, err := RenderAll(load(t)); err != nil {
		t.Fatalf("RenderAll: %v", err)
	}
}

func TestEachTargetMatchesItsGolden(t *testing.T) {
	out, err := RenderAll(load(t))
	if err != nil {
		t.Fatalf("RenderAll: %v", err)
	}
	golden := map[string]string{
		"dist/SKILL.md":               "SKILL.md",
		"dist/AGENTS.md":              "AGENTS.md",
		"dist/cursor/rules/rtdd.mdc":  "rtdd.mdc",
	}
	for path, name := range golden {
		want, err := os.ReadFile(filepath.Join("testdata", "golden", name))
		if err != nil {
			t.Fatalf("read golden %s: %v", name, err)
		}
		if out[path] != string(want) {
			t.Errorf("%s differs from testdata/golden/%s; run `rtdd-gen render` and review", path, name)
		}
	}
}

func TestAgentsStaysUnderItsBudget(t *testing.T) {
	out, _ := RenderAll(load(t))
	tg, _ := TargetByName("agents")
	if got := len(out[tg.OutPath]); got > tg.MaxBytes {
		t.Fatalf("AGENTS.md is %d bytes, budget %d", got, tg.MaxBytes)
	}
}

func TestOverBudgetIsAnErrorNotATruncation(t *testing.T) {
	d := load(t)
	// Inflate a section that AGENTS.md renders, with no agents variant to shrink it.
	for i := range d.Sections {
		if d.Sections[i].ID == "empty" {
			d.Sections[i].Variants = map[string]string{}
			d.Sections[i].Body = strings.Repeat("padding padding padding\n", 400)
		}
	}
	_, err := RenderAll(d)
	if err == nil {
		t.Fatal("want an over-budget error")
	}
	if !strings.Contains(err.Error(), "budget") {
		t.Fatalf("error should name the budget, got %v", err)
	}
}

func TestAgentsRejectsTheSkillBodyLeakingIn(t *testing.T) {
	// The exact failure a byte-comparison drift check cannot see.
	leaked := BeginMarker + "\n## rtdd\n\nname: rtdd\n\n" + EndMarker + "\n"
	if err := validateAgents(leaked); err == nil {
		t.Fatal("want an error when skill frontmatter leaks into AGENTS.md")
	}
}

func TestMDCRejectsMissingGlobs(t *testing.T) {
	broken := "---\ndescription: x\nalwaysApply: false\n---\n\n" + Generated + "\n\nrtdd which\n"
	if err := validateMDC(broken); err == nil {
		t.Fatal("want an error when .mdc frontmatter has no globs")
	}
}

func TestMDCRejectsAlwaysApplyTrue(t *testing.T) {
	broken := "---\ndescription: x\nglobs: **/*.py\nalwaysApply: true\n---\n\n" + Generated + "\n\nrtdd which\n"
	if err := validateMDC(broken); err == nil {
		t.Fatal("want an error when .mdc sets alwaysApply: true")
	}
}

func TestSkillRejectsMissingFrontmatter(t *testing.T) {
	if err := validateSkill("# rtdd\n\nrtdd which\n"); err == nil {
		t.Fatal("want an error when SKILL.md has no frontmatter")
	}
}

func TestEveryTargetGetsDifferentContent(t *testing.T) {
	out, _ := RenderAll(load(t))
	skill := out["dist/SKILL.md"]
	agents := out["dist/AGENTS.md"]
	mdc := out["dist/cursor/rules/rtdd.mdc"]
	if skill == agents || skill == mdc || agents == mdc {
		t.Fatal("two targets rendered identical content — the variants are not being applied")
	}
	if len(agents) >= len(mdc) || len(mdc) >= len(skill) {
		t.Fatalf("expected agents < mdc < skill by length, got %d %d %d",
			len(agents), len(mdc), len(skill))
	}
}
```

- [ ] Generate the goldens from the first successful render, review them by eye, and commit.

```bash
cd /home/claude/rtdd
mkdir -p internal/protocol/testdata/golden
go run ./cmd/rtdd-gen render --out internal/protocol/testdata/golden --flat
# (Task 18 adds the CLI; until then, generate with a throwaway `go run` of RenderAll.)
go test ./internal/protocol/
# expected: ok  github.com/VocanicZ/rtdd/internal/protocol
git add internal/protocol protocol
git commit -m "protocol: per-target renderers, byte budgets, and validity assertions

Budget overflow errors rather than truncating, and each target asserts the content
it is useless without plus the content that belongs to another target. A drift
check that passes while two of three front-ends are wrong now fails here first.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

## Task 18 — `rtdd-gen` CLI and the CI drift check

**Files:**
- `cmd/rtdd-gen/main.go` (new)
- `.github/workflows/ci.yml` (edited)

**Interfaces:**
```
rtdd-gen render [--out <dir>] [--flat]   write every target
rtdd-gen check                           fail if any target on disk differs from a fresh render
rtdd-gen verify                          re-validate the files on disk against the target rules
```

**Steps:**

- [ ] Implement the CLI.

```go
// cmd/rtdd-gen/main.go

// Command rtdd-gen renders protocol/PROTOCOL.md into the generated agent
// front-ends under dist/, and checks them for staleness and validity.
//
// `check` catches drift. `verify` catches content that is wrong for a target
// even when it is not stale — the two are separate because a byte comparison
// alone cannot tell that AGENTS.md swallowed the skill body.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/VocanicZ/rtdd/internal/protocol"
)

const protocolPath = "protocol/PROTOCOL.md"

func loadDoc(root string) (*protocol.Doc, error) {
	src, err := os.ReadFile(filepath.Join(root, protocolPath))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", protocolPath, err)
	}
	return protocol.Parse(string(src))
}

func render(root, out string, flat bool) error {
	doc, err := loadDoc(root)
	if err != nil {
		return err
	}
	files, err := protocol.RenderAll(doc)
	if err != nil {
		return err
	}
	for rel, body := range files {
		dest := filepath.Join(root, rel)
		if out != "" {
			if flat {
				dest = filepath.Join(out, filepath.Base(rel))
			} else {
				dest = filepath.Join(out, rel)
			}
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dest, []byte(body), 0o644); err != nil {
			return err
		}
		fmt.Printf("wrote %s (%d bytes)\n", dest, len(body))
	}
	return nil
}

func check(root string) error {
	doc, err := loadDoc(root)
	if err != nil {
		return err
	}
	files, err := protocol.RenderAll(doc)
	if err != nil {
		return err
	}
	var stale []string
	for rel, want := range files {
		got, err := os.ReadFile(filepath.Join(root, rel))
		if err != nil {
			stale = append(stale, rel+" (missing)")
			continue
		}
		if string(got) != want {
			stale = append(stale, rel)
		}
	}
	if len(stale) > 0 {
		return fmt.Errorf("generated front-ends are stale: %v\nrun `rtdd-gen render` and commit the result", stale)
	}
	fmt.Println("dist/ is up to date with protocol/PROTOCOL.md")
	return nil
}

func verify(root string) error {
	for _, t := range protocol.Targets {
		body, err := os.ReadFile(filepath.Join(root, t.OutPath))
		if err != nil {
			return fmt.Errorf("%s: %w", t.OutPath, err)
		}
		if len(body) > t.MaxBytes {
			return fmt.Errorf("%s is %d bytes, budget %d", t.OutPath, len(body), t.MaxBytes)
		}
		if err := t.Validate(string(body)); err != nil {
			return fmt.Errorf("%s: %w", t.OutPath, err)
		}
		fmt.Printf("%-30s ok  %5d / %5d bytes\n", t.OutPath, len(body), t.MaxBytes)
	}
	return nil
}

func main() {
	root := flag.String("root", ".", "repository root")
	out := flag.String("out", "", "write to this directory instead of the repository")
	flat := flag.Bool("flat", false, "with --out, write basenames into one directory")
	flag.Parse()

	cmd := flag.Arg(0)
	var err error
	switch cmd {
	case "render":
		err = render(*root, *out, *flat)
	case "check":
		err = check(*root)
	case "verify":
		err = verify(*root)
	default:
		err = errors.New("usage: rtdd-gen [render|check|verify]")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "rtdd-gen: "+err.Error())
		os.Exit(1)
	}
}
```

- [ ] Render, then prove `check` and `verify` fail independently.

```bash
cd /home/claude/rtdd
go run ./cmd/rtdd-gen render
# expected:
#   wrote dist/SKILL.md (nnnnn bytes)
#   wrote dist/AGENTS.md (nnnn bytes)
#   wrote dist/cursor/rules/rtdd.mdc (nnnn bytes)
go run ./cmd/rtdd-gen check
# expected: dist/ is up to date with protocol/PROTOCOL.md
go run ./cmd/rtdd-gen verify
# expected:
#   dist/SKILL.md                  ok  nnnnn / 20000 bytes
#   dist/AGENTS.md                 ok   nnnn /  1800 bytes
#   dist/cursor/rules/rtdd.mdc     ok   nnnn /  4000 bytes

# check catches staleness:
printf '\nhand edit\n' >> dist/AGENTS.md
go run ./cmd/rtdd-gen check; echo "exit=$?"
# expected: rtdd-gen: generated front-ends are stale: [dist/AGENTS.md] ... / exit=1

# verify catches wrongness that is not staleness:
git checkout dist/AGENTS.md
sed -i 's/^globs: .*/globs:/' dist/cursor/rules/rtdd.mdc
go run ./cmd/rtdd-gen verify; echo "exit=$?"
# expected: rtdd-gen: dist/cursor/rules/rtdd.mdc: missing required content "\nglobs: " / exit=1
git checkout dist/cursor/rules/rtdd.mdc
```

- [ ] Wire CI. Both checks run, plus a guard that `dist/` was not hand-edited in the diff.

```yaml
# .github/workflows/ci.yml — add these jobs
  frontends:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with: { fetch-depth: 0 }
      - uses: actions/setup-go@v5
        with: { go-version: '1.24' }
      - name: dist/ must be generated, never hand-edited
        run: |
          set -e
          base="${{ github.event.pull_request.base.sha }}"
          if [ -n "$base" ] && git diff --name-only "$base"...HEAD | grep -q '^dist/'; then
            if ! git diff --name-only "$base"...HEAD | grep -q '^protocol/PROTOCOL.md$'; then
              echo "dist/ changed without protocol/PROTOCOL.md changing — dist/ is generated" >&2
              exit 1
            fi
          fi
      - run: go run ./cmd/rtdd-gen check
      - run: go run ./cmd/rtdd-gen verify
      - run: go test ./internal/protocol/
```

- [ ] Commit.

```bash
git add cmd/rtdd-gen dist .github/workflows/ci.yml
git commit -m "gen: rtdd-gen render/check/verify plus the CI drift and validity gates

check catches staleness, verify catches per-target wrongness that staleness cannot
see, and a diff guard rejects hand edits under dist/.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

## Task 19 — `rtdd init` with a non-clobbering merge strategy

**Files:**
- `internal/install/merge.go` (new)
- `internal/install/merge_test.go` (new)
- `internal/install/install.go` (new)
- `cmd/rtdd/init.go` (edited — the existing `rtdd init` subcommand)

**Interfaces:**
```go
package install

type Action int
const (
    Create Action = iota
    ReplaceBlock
    AppendBlock
    Skip        // identical content already present
    Conflict    // present, different, and not ours to touch — needs --force
)

type Step struct {
    Path    string
    Action  Action
    Content string
    Note    string
}

// MergeBlock inserts or replaces the marker-delimited rtdd block in existing.
// Content outside the markers is never modified.
func MergeBlock(existing, block string) (string, Action, error)

func Plan(root string, files map[string]string, force bool) ([]Step, error)
func Apply(root string, steps []Step) error
```

**Steps:**

- [ ] Failing test first. The non-clobbering guarantee is the whole feature.

```go
// internal/install/merge_test.go
package install

import (
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/protocol"
)

func block(body string) string {
	return protocol.BeginMarker + "\n" + body + "\n" + protocol.EndMarker + "\n"
}

func TestMergeIntoAnEmptyFileIsTheBlockAlone(t *testing.T) {
	got, action, err := MergeBlock("", block("v1"))
	if err != nil {
		t.Fatal(err)
	}
	if action != Create {
		t.Fatalf("action = %v, want Create", action)
	}
	if got != block("v1") {
		t.Fatalf("got %q", got)
	}
}

func TestMergeAppendsToAFileWithNoMarkersAndKeepsEverything(t *testing.T) {
	existing := "# Our conventions\n\nUse tabs. Ship on Fridays.\n"
	got, action, err := MergeBlock(existing, block("v1"))
	if err != nil {
		t.Fatal(err)
	}
	if action != AppendBlock {
		t.Fatalf("action = %v, want AppendBlock", action)
	}
	if !strings.HasPrefix(got, existing) {
		t.Fatal("existing content was modified")
	}
	if !strings.Contains(got, "Ship on Fridays.") {
		t.Fatal("existing content was lost")
	}
	if !strings.Contains(got, protocol.BeginMarker) {
		t.Fatal("block was not appended")
	}
}

func TestMergeReplacesOnlyBetweenTheMarkers(t *testing.T) {
	existing := "# Ours\n\nbefore\n\n" + block("v1") + "\nafter\n"
	got, action, err := MergeBlock(existing, block("v2"))
	if err != nil {
		t.Fatal(err)
	}
	if action != ReplaceBlock {
		t.Fatalf("action = %v, want ReplaceBlock", action)
	}
	if !strings.Contains(got, "before") || !strings.Contains(got, "after") {
		t.Fatal("content outside the markers was modified")
	}
	if strings.Contains(got, "v1") {
		t.Fatal("old block survived")
	}
	if !strings.Contains(got, "v2") {
		t.Fatal("new block missing")
	}
}

func TestMergeIsIdempotent(t *testing.T) {
	existing := "# Ours\n\nbefore\n\n" + block("v1") + "\nafter\n"
	once, _, _ := MergeBlock(existing, block("v1"))
	twice, action, _ := MergeBlock(once, block("v1"))
	if once != twice {
		t.Fatal("merge is not idempotent")
	}
	if action != Skip {
		t.Fatalf("action = %v, want Skip on an unchanged block", action)
	}
}

func TestUnterminatedMarkerIsAnErrorNotAGuess(t *testing.T) {
	existing := "# Ours\n\n" + protocol.BeginMarker + "\nhalf a block\n"
	if _, _, err := MergeBlock(existing, block("v1")); err == nil {
		t.Fatal("want an error for an unterminated rtdd block")
	}
}

func TestDoubledBeginMarkerIsAnError(t *testing.T) {
	existing := block("v1") + block("v1")
	if _, _, err := MergeBlock(existing, block("v2")); err == nil {
		t.Fatal("want an error when two rtdd blocks are present")
	}
}
```

```bash
cd /home/claude/rtdd && go test ./internal/install/
# expected: no Go files in .../internal/install
```

- [ ] Implement `merge.go`.

```go
// internal/install/merge.go

// Package install writes rtdd's generated front-ends into a host repository.
//
// AGENTS.md and CLAUDE.md belong to the host project, so rtdd owns only what is
// between its markers and never rewrites a byte outside them. Anything it cannot
// interpret unambiguously — a half-written block, two blocks — is an error, not
// a guess, because guessing here destroys someone else's file.
package install

import (
	"fmt"
	"strings"

	"github.com/VocanicZ/rtdd/internal/protocol"
)

type Action int

const (
	Create Action = iota
	ReplaceBlock
	AppendBlock
	Skip
	Conflict
)

func (a Action) String() string {
	switch a {
	case Create:
		return "create"
	case ReplaceBlock:
		return "replace-block"
	case AppendBlock:
		return "append-block"
	case Skip:
		return "skip"
	case Conflict:
		return "conflict"
	}
	return "unknown"
}

func MergeBlock(existing, block string) (string, Action, error) {
	begins := strings.Count(existing, protocol.BeginMarker)
	ends := strings.Count(existing, protocol.EndMarker)

	switch {
	case begins == 0 && ends == 0:
		if strings.TrimSpace(existing) == "" {
			return block, Create, nil
		}
		trimmed := strings.TrimRight(existing, "\n")
		return trimmed + "\n\n" + block, AppendBlock, nil

	case begins == 1 && ends == 1:
		start := strings.Index(existing, protocol.BeginMarker)
		endAt := strings.Index(existing, protocol.EndMarker)
		if endAt < start {
			return "", Conflict, fmt.Errorf("rtdd END marker appears before BEGIN — fix the file by hand")
		}
		endAt += len(protocol.EndMarker)
		if endAt < len(existing) && existing[endAt] == '\n' {
			endAt++
		}
		current := existing[start:endAt]
		if current == block {
			return existing, Skip, nil
		}
		return existing[:start] + block + existing[endAt:], ReplaceBlock, nil

	default:
		return "", Conflict, fmt.Errorf(
			"found %d BEGIN and %d END rtdd markers — expected exactly one of each; fix the file by hand",
			begins, ends)
	}
}
```

- [ ] Implement `install.go`.

```go
// internal/install/install.go
package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/VocanicZ/rtdd/internal/protocol"
)

type Step struct {
	Path    string
	Action  Action
	Content string
	Note    string
}

const gitattributesLine = ".rtdd/map.jsonl merge=union"

const defaultConfig = `# .rtdd/config.yaml — rtdd defaults, see docs/specs for what each one does.
stale_commits: 50
drift_guard: 100
hub_threshold: 0.40
`

// Plan computes what `rtdd init` would do without touching the filesystem.
//
// files maps the generator's output paths to their content, i.e. exactly what
// protocol.RenderAll returns. Whole-file targets (the Claude Code skill, the
// Cursor rule) are created but never overwritten without --force, because the
// host may have edited them. Marker-delimited targets (AGENTS.md, CLAUDE.md) are
// merged, which is always safe.
func Plan(root string, files map[string]string, force bool) ([]Step, error) {
	steps := []Step{}

	// 1. Whole-file targets.
	whole := map[string]string{
		".claude/skills/rtdd/SKILL.md": files["dist/SKILL.md"],
		".cursor/rules/rtdd.mdc":       files["dist/cursor/rules/rtdd.mdc"],
	}
	for rel, content := range whole {
		abs := filepath.Join(root, rel)
		existing, err := os.ReadFile(abs)
		switch {
		case os.IsNotExist(err):
			steps = append(steps, Step{Path: rel, Action: Create, Content: content})
		case err != nil:
			return nil, err
		case string(existing) == content:
			steps = append(steps, Step{Path: rel, Action: Skip, Note: "already current"})
		case force:
			steps = append(steps, Step{Path: rel, Action: Create, Content: content, Note: "overwritten by --force"})
		default:
			steps = append(steps, Step{
				Path: rel, Action: Conflict,
				Note: "exists and differs; re-run with --force to overwrite",
			})
		}
	}

	// 2. Marker-delimited targets. CLAUDE.md is only touched if the host already
	// has one — rtdd does not introduce a CLAUDE.md into a repo that has none,
	// because the skill file is the right Claude Code surface.
	block := files["dist/AGENTS.md"]
	markerTargets := []string{"AGENTS.md"}
	if _, err := os.Stat(filepath.Join(root, "CLAUDE.md")); err == nil {
		markerTargets = append(markerTargets, "CLAUDE.md")
	}
	for _, rel := range markerTargets {
		abs := filepath.Join(root, rel)
		existing, err := os.ReadFile(abs)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		merged, action, err := MergeBlock(string(existing), block)
		if err != nil {
			steps = append(steps, Step{Path: rel, Action: Conflict, Note: err.Error()})
			continue
		}
		steps = append(steps, Step{Path: rel, Action: action, Content: merged})
	}

	// 3. .gitattributes — append one line if it is not already there.
	ga := filepath.Join(root, ".gitattributes")
	existing, err := os.ReadFile(ga)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if strings.Contains(string(existing), gitattributesLine) {
		steps = append(steps, Step{Path: ".gitattributes", Action: Skip, Note: "merge=union already set"})
	} else {
		body := strings.TrimRight(string(existing), "\n")
		if body != "" {
			body += "\n"
		}
		steps = append(steps, Step{
			Path:    ".gitattributes",
			Action:  AppendBlock,
			Content: body + gitattributesLine + "\n",
		})
	}

	// 4. .rtdd/config.yaml — created, never overwritten.
	cfg := filepath.Join(root, ".rtdd", "config.yaml")
	if _, err := os.Stat(cfg); os.IsNotExist(err) {
		steps = append(steps, Step{Path: ".rtdd/config.yaml", Action: Create, Content: defaultConfig})
	} else {
		steps = append(steps, Step{Path: ".rtdd/config.yaml", Action: Skip, Note: "keeping your config"})
	}

	return steps, nil
}

func Apply(root string, steps []Step) error {
	for _, s := range steps {
		if s.Action == Skip {
			continue
		}
		if s.Action == Conflict {
			return fmt.Errorf("%s: %s", s.Path, s.Note)
		}
		abs := filepath.Join(root, s.Path)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(abs, []byte(s.Content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Files returns the generated front-ends embedded in the binary, so `rtdd init`
// works in a host repo that has no copy of protocol/PROTOCOL.md.
func Files() (map[string]string, error) {
	doc, err := protocol.Parse(embeddedProtocol)
	if err != nil {
		return nil, err
	}
	return protocol.RenderAll(doc)
}
```

- [ ] Embed the protocol so `rtdd init` is self-contained.

```go
// internal/install/embed.go
package install

import _ "embed"

//go:embed protocol.md
var embeddedProtocol string
```

```bash
cd /home/claude/rtdd
cp protocol/PROTOCOL.md internal/install/protocol.md
# and add a CI step so the copy cannot drift:
```

```yaml
# add to the frontends job in .github/workflows/ci.yml
      - name: embedded protocol copy must match the source
        run: diff -u protocol/PROTOCOL.md internal/install/protocol.md
```

- [ ] Wire the CLI subcommand.

```go
// cmd/rtdd/init.go
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/VocanicZ/rtdd/internal/install"
)

func runInit(args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "print the plan and change nothing")
	force := fs.Bool("force", false, "overwrite whole-file front-ends that exist and differ")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "rtdd init: "+err.Error())
		return 2
	}
	files, err := install.Files()
	if err != nil {
		fmt.Fprintln(os.Stderr, "rtdd init: "+err.Error())
		return 2
	}
	steps, err := install.Plan(root, files, *force)
	if err != nil {
		fmt.Fprintln(os.Stderr, "rtdd init: "+err.Error())
		return 2
	}

	conflicts := 0
	for _, s := range steps {
		note := ""
		if s.Note != "" {
			note = "  — " + s.Note
		}
		fmt.Printf("%-14s %s%s\n", s.Action, s.Path, note)
		if s.Action == install.Conflict {
			conflicts++
		}
	}
	if *dryRun {
		return 0
	}
	if conflicts > 0 {
		fmt.Fprintf(os.Stderr, "rtdd init: %d conflict(s); nothing written\n", conflicts)
		return 2
	}
	if err := install.Apply(root, steps); err != nil {
		fmt.Fprintln(os.Stderr, "rtdd init: "+err.Error())
		return 2
	}
	fmt.Println("rtdd init: done")
	return 0
}
```

- [ ] Prove the non-clobbering behaviour end to end on a scratch repo.

```bash
cd /home/claude/rtdd && go test ./internal/install/
# expected: ok  github.com/VocanicZ/rtdd/internal/install
go build -o /tmp/rtdd ./cmd/rtdd

rm -rf /tmp/hostrepo && mkdir -p /tmp/hostrepo && cd /tmp/hostrepo && git init -q
printf '# Our conventions\n\nUse tabs. Ship on Fridays.\n' > AGENTS.md
printf '*.png binary\n' > .gitattributes
/tmp/rtdd init --dry-run
# expected:
#   create         .claude/skills/rtdd/SKILL.md
#   create         .cursor/rules/rtdd.mdc
#   append-block   AGENTS.md
#   append-block   .gitattributes
#   create         .rtdd/config.yaml
/tmp/rtdd init
grep -c 'Ship on Fridays' AGENTS.md      # expected: 1
grep -c 'BEGIN rtdd' AGENTS.md           # expected: 1
grep -c 'binary' .gitattributes          # expected: 1
grep -c 'merge=union' .gitattributes     # expected: 1
/tmp/rtdd init                            # second run must be a no-op
grep -c 'BEGIN rtdd' AGENTS.md           # expected: 1
# expected on the second run: every line reads "skip"
```

- [ ] Commit.

```bash
cd /home/claude/rtdd && git add internal/install cmd/rtdd/init.go .github/workflows/ci.yml
git commit -m "init: install front-ends into a host repo without clobbering

AGENTS.md and CLAUDE.md are merged between rtdd's own markers and nothing outside
them is touched; ambiguous marker states are errors rather than guesses. Whole-file
targets are created but never overwritten without --force.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

## Task 20 — GoReleaser cross-platform binaries

**Files:**
- `.goreleaser.yaml` (new)
- `.github/workflows/release.yml` (new)

**Interfaces:** none — build configuration.

**Steps:**

- [ ] Write the config. `CGO_ENABLED=0` matters: spec decision D4 is a static binary, and the
      SQLite reader is `modernc.org/sqlite`, which is pure Go precisely so this holds.

```yaml
# .goreleaser.yaml
version: 2

project_name: rtdd

before:
  hooks:
    - go mod tidy
    - go run ./cmd/rtdd-gen check
    - go run ./cmd/rtdd-gen verify
    - go test ./...

builds:
  - id: rtdd
    main: ./cmd/rtdd
    binary: rtdd
    env:
      - CGO_ENABLED=0
    flags:
      - -trimpath
    ldflags:
      - -s -w -X main.version={{.Version}} -X main.commit={{.FullCommit}} -X main.date={{.CommitDate}}
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    ignore:
      - goos: windows
        goarch: arm64

archives:
  - id: rtdd
    ids: [rtdd]
    name_template: "rtdd_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    format_overrides:
      - goos: windows
        formats: [zip]
    files:
      - README.md
      - LICENSE
      - dist/SKILL.md
      - dist/AGENTS.md
      - dist/cursor/rules/rtdd.mdc

checksum:
  name_template: "checksums.txt"

snapshot:
  version_template: "{{ incpatch .Version }}-next"

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^chore:"

release:
  draft: true
  prerelease: auto
  extra_files:
    - glob: bench/results/swebench/tables.md
    - glob: bench/results/swebench/summary.json
    - glob: bench/PREREGISTRATION.md
```

- [ ] Write the release workflow.

```yaml
# .github/workflows/release.yml
name: release

on:
  push:
    tags: ['v*']

permissions:
  contents: write

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with: { fetch-depth: 0 }
      - uses: actions/setup-go@v5
        with: { go-version: '1.24' }
      - uses: goreleaser/goreleaser-action@v6
        with:
          version: '~> v2'
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] Verify a snapshot build locally, including that the binary is genuinely static.

```bash
cd /home/claude/rtdd
goreleaser release --snapshot --clean
# expected tail: • release succeeded after Ns
file dist/rtdd_linux_amd64_v1/rtdd
# expected: ELF 64-bit LSB executable, x86-64, statically linked, ...
ldd dist/rtdd_linux_amd64_v1/rtdd 2>&1 | head -1
# expected: not a dynamic executable
ls dist/*.tar.gz dist/checksums.txt
```

- [ ] Note that GoReleaser writes to `dist/`, which is also the generated-front-end directory.
      Keep them apart so the drift check does not see build output.

```bash
cd /home/claude/rtdd
cat >> .gitignore <<'EOF'
# GoReleaser build output. dist/SKILL.md, dist/AGENTS.md and dist/cursor/ are
# GENERATED SOURCES and are tracked; everything else GoReleaser drops here is not.
/dist/*
!/dist/SKILL.md
!/dist/AGENTS.md
!/dist/cursor/
EOF
git status --short dist/
# expected: nothing — the three generated files are tracked and unchanged
git add .goreleaser.yaml .github/workflows/release.yml .gitignore
git commit -m "release: GoReleaser cross-platform static binaries

CGO_ENABLED=0 holds because the SQLite reader is modernc.org/sqlite (spec D4).
The release gate runs rtdd-gen check and verify before it builds anything.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

## Task 21 — `curl | sh` install script

**Files:**
- `install.sh` (new)

**Interfaces:**
```
curl -fsSL https://raw.githubusercontent.com/VocanicZ/rtdd/main/install.sh | sh
RTDD_VERSION=v0.1.0 RTDD_INSTALL_DIR=~/.local/bin sh install.sh
```

**Steps:**

- [ ] Write it. Checksum verification is not optional in a script people pipe into a shell.

```bash
cat > /home/claude/rtdd/install.sh <<'EOF'
#!/bin/sh
# rtdd installer.
#
#   curl -fsSL https://raw.githubusercontent.com/VocanicZ/rtdd/main/install.sh | sh
#
# Environment:
#   RTDD_VERSION       tag to install (default: latest release)
#   RTDD_INSTALL_DIR   where to put the binary (default: /usr/local/bin, else ~/.local/bin)
set -eu

REPO="VocanicZ/rtdd"
VERSION="${RTDD_VERSION:-}"

log()  { printf '%s\n' "$*" >&2; }
die()  { printf 'install.sh: %s\n' "$*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || die "required command not found: $1"; }

need uname
need tar
if command -v curl >/dev/null 2>&1; then
  fetch() { curl -fsSL "$1"; }
  fetch_to() { curl -fsSL -o "$2" "$1"; }
elif command -v wget >/dev/null 2>&1; then
  fetch() { wget -qO- "$1"; }
  fetch_to() { wget -qO "$2" "$1"; }
else
  die "need curl or wget"
fi

os=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$os" in
  linux|darwin) ;;
  *) die "unsupported OS: $os (Windows users: download the zip from the releases page)" ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  aarch64|arm64) arch=arm64 ;;
  *) die "unsupported architecture: $arch" ;;
esac

if [ -z "$VERSION" ]; then
  VERSION=$(fetch "https://api.github.com/repos/$REPO/releases/latest" \
    | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1)
  [ -n "$VERSION" ] || die "could not determine the latest release; set RTDD_VERSION"
fi
plain=${VERSION#v}

archive="rtdd_${plain}_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/download/$VERSION"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

log "downloading rtdd $VERSION ($os/$arch)"
fetch_to "$base/$archive" "$tmp/$archive" || die "download failed: $base/$archive"
fetch_to "$base/checksums.txt" "$tmp/checksums.txt" || die "could not fetch checksums.txt"

expected=$(grep " $archive\$" "$tmp/checksums.txt" | awk '{print $1}')
[ -n "$expected" ] || die "no checksum for $archive in checksums.txt"
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmp/$archive" | awk '{print $1}')
elif command -v shasum >/dev/null 2>&1; then
  actual=$(shasum -a 256 "$tmp/$archive" | awk '{print $1}')
else
  die "need sha256sum or shasum to verify the download"
fi
[ "$actual" = "$expected" ] || die "checksum mismatch: expected $expected, got $actual"
log "checksum ok"

tar -xzf "$tmp/$archive" -C "$tmp"
[ -f "$tmp/rtdd" ] || die "archive did not contain an rtdd binary"
chmod +x "$tmp/rtdd"

dir="${RTDD_INSTALL_DIR:-}"
if [ -z "$dir" ]; then
  if [ -w /usr/local/bin ] 2>/dev/null; then dir=/usr/local/bin; else dir="$HOME/.local/bin"; fi
fi
mkdir -p "$dir"
mv "$tmp/rtdd" "$dir/rtdd"
log "installed $dir/rtdd"

case ":$PATH:" in
  *":$dir:"*) ;;
  *) log "note: $dir is not on your PATH" ;;
esac

"$dir/rtdd" --version || die "installed binary did not run"
log ""
log "next: cd into a Python repo, then"
log "  rtdd init      install the agent front-ends and .gitattributes"
log "  rtdd seed      one full instrumented run to build the map"
log "  rtdd which     see what covers your current changes"
EOF
chmod +x /home/claude/rtdd/install.sh
```

- [ ] Verify it against a snapshot release before it is ever piped into a shell.

```bash
cd /home/claude/rtdd && sh -n install.sh && echo "syntax ok"
# expected: syntax ok
RTDD_VERSION=v0.1.0 RTDD_INSTALL_DIR=/tmp/rtdd-install sh install.sh
# expected (after the release exists):
#   downloading rtdd v0.1.0 (linux/amd64)
#   checksum ok
#   installed /tmp/rtdd-install/rtdd
#   rtdd 0.1.0
git add install.sh && git commit -m "release: curl | sh installer with checksum verification

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

## Task 22 — README, with prior art cited before results

> Take this branch when Task 14's human decision selected the "RTDD wins clearly" or "ties"
> row. If it selected "RTDD loses" or "kill criterion not met", **skip to Task 23** — that
> task replaces this README, it does not amend it.

Spec §3 says RTDD does not claim to have invented test selection, and the README will say so.
The prior-art section below is the actual text to ship, not a description of one. It comes
**before** the results table in the document, because a results table read before the prior
art invites exactly the misreading the audit's finding A9 caught in v1.

**Files:**
- `README.md` (new)
- `LICENSE` (new — MIT)

**Interfaces:** none.

**Steps:**

- [ ] Write the README. Fill the results table from `bench/results/swebench/tables.md` and
      the Axis 2 table from `bench/results/replay/tables.md`; every other word is final.

```bash
cat > /home/claude/rtdd/README.md <<'EOF'
# rtdd

**rtdd tells an agent which tests cover the code it just changed, and which of the lines it
just changed nothing covers** — derived from real execution rather than a static call graph.

It is a context provider, not a gate. `rtdd run` exits non-zero when a test fails, and for
no other reason.

```
$ rtdd run
  changed: src/auth.py:40-58, src/constants.py:1-12
  12 tests selected, ranked

  ..........✓✓                                    12 passed  1.4s

  UNCOVERED: src/auth.py:52-58  (7 changed lines, no executing test)
  import-time: src/constants.py:1-12  (executed during collection, not attributed)
```

Python only, today. See [What it does not do](#what-it-does-not-do).

## Install

```
curl -fsSL https://raw.githubusercontent.com/VocanicZ/rtdd/main/install.sh | sh
cd your-python-repo
rtdd init      # front-ends, .gitattributes merge=union, config
rtdd seed      # one full instrumented run to build the map
rtdd which     # what covers your current changes
```

`rtdd init` installs a Claude Code skill at `.claude/skills/rtdd/SKILL.md`, a Cursor rule at
`.cursor/rules/rtdd.mdc`, and a short marker-delimited block in `AGENTS.md` (and `CLAUDE.md`
if you have one). It never rewrites a byte outside its own markers.

## Prior art

RTDD did not invent test impact analysis. It is a late entry in a long line, and the
comparison against that line is the point of the project rather than a marketing frame.

- **[TDAD](https://arxiv.org/abs/2603.17973)** — *Test-Driven Agentic Development*, Alonso,
  Yovine and Braberman, March 2026 — is the closest work and the direct baseline. It builds a
  **static** source↔test dependency graph by parsing Python syntax trees, ships it to the
  agent as a skill file, and measured regressions on SWE-bench Verified falling from **6.08%
  to 1.82%**. Its reference implementation is at
  [pepealonso95/TDAD](https://github.com/pepealonso95/TDAD) and a TypeScript port is at
  [fmguerreiro/tdad-ts](https://github.com/fmguerreiro/tdad-ts). RTDD runs TDAD's own
  implementation as an arm of its benchmark rather than citing its number.

  **TDAD's second result is why this tool looks the way it does.** Adding TDD *procedural*
  instructions without targeted test context raised regressions to **9.94% — worse than no
  intervention at all**. Their conclusion is that surfacing contextual information beats
  prescribing procedural workflows, and RTDD is built on that conclusion: it reports, and it
  has no opinion about your patch.

- **[pytest-testmon](https://www.testmon.org/blog/determining-affected-tests/)** has done
  coverage-derived Python test selection since around 2016, with **method-level AST
  checksums** — finer-grained than RTDD's file-level rows. If you want test selection for a
  human's editor loop, use testmon. RTDD is not an improvement on it and does not claim to be.

- **[Wallaby.js](https://wallabyjs.com/docs/features/ai/)** already ships an MCP server
  exposing `wallaby_coveredLinesForTest` and `wallaby_allTestsForFileAndLine` to Claude Code
  and Cursor. Agent-facing coverage context is not a new idea; it is a shipping commercial
  product for JavaScript.

- **[Infinitest](https://github.com/infinitest/infinitest)** (2007) marketed classpath impact
  analysis for "tight TDD cycles" nearly two decades ago.

- **[Ekstazi](https://users.ece.utexas.edu/~gligoric/papers/GligoricETAL15Ekstazi.pdf)**
  (Gligoric, Eloussi and Marinov, 2015) is the reference result for file-level regression
  test selection, and the source of the most sobering number in this README: selecting a
  small fraction of tests still yielded only about **32% average end-to-end reduction**. That
  gap between "2% of tests selected" and "32% faster" is real, and RTDD measures itself
  against it rather than reporting selection ratio alone.

- **SonarQube's new-code coverage gate**, Codecov's patch status, and `diff-cover` are prior
  art for "changed code that no test covers is a problem". RTDD's uncovered-change report is
  the same idea moved from CI into the inner loop, and reported rather than enforced.

Also in the lineage and not claimed as novel: Bazel, Google TAP, Microsoft's Azure DevOps
Test Impact Analysis, `jest --findRelatedTests`, NCrunch, and Meta's predictive test
selection.

**What is new here, if anything, is one measurement.** Every tool above builds its map
either statically or from per-file coverage aggregates. TDAD published a static-graph number
on SWE-bench Verified with a reproducible harness. RTDD substitutes a **dynamic coverage**
map — which sees dynamic dispatch, dependency injection, plugin registries and monkeypatching
that a call graph reports as zero callers, and is blind in return to any path no test has
ever taken — and reruns the same benchmark. So the question *does execution-derived context
beat a call graph for agent regressions?* has an answer instead of an argument.

The answer is below, whichever way it fell.

## Results

### Axis 1 — agent regression rate on SWE-bench Verified

Pre-registered before the run at [`bench/PREREGISTRATION.md`](bench/PREREGISTRATION.md), git
tag `prereg-m4`: the 100-instance sample and its seed, the arms, the model, the equivalence
band, and the kill criterion. All five arms ran on one harness with one model
(Qwen3-Coder-30B-A3B-Instruct, greedy, 40 turns), so nothing in this table is a
cross-harness comparison.

<!-- PASTE bench/results/swebench/tables.md HERE -->

A regression is defined mechanically, in
[`bench/swebench/metrics.py`](bench/swebench/metrics.py): a test id in the SWE-bench
evaluation report's `tests_status.PASS_TO_PASS.failure` list — a test that passed before the
patch and fails after it. The denominator comes from the dataset rather than the report, and
instances whose patch is empty or fails to apply stay in both denominators, so no arm can
lower its regression rate by producing nothing. Resolution rate is printed beside the
regression rate in every row for the same reason.

The RTDD arm's integration is a `rtdd_which` tool and a declarative description of what it
returns. Its prompt is byte-for-byte the vanilla arm's prompt plus one `<test-context>`
block — [asserted in a test](bench/swebench/tests/test_prompts.py), along with an
imperative-phrase lint on the block itself — because TDAD measured procedural prose as
actively harmful and an arm that smuggled it in would be uninterpretable.

### Axis 2 — selection quality on real commits

Replay of real commits per repo against every baseline a reviewer will ask for:
pytest-testmon, a naive `tests/test_<module>.py` path heuristic, `pytest --lf`, a static
import graph, `pytest -n auto`, and random selection at equal selection ratio. Never pooled
across repos.

<!-- PASTE bench/results/replay/tables.md HERE -->

Both change-level and test-level recall are published, stratified by `|F_full|` with the
single-killer stratum broken out, because pooled recall is mostly a measurement of hub
coverage. Wall-clock is from disclosed hardware in
[`bench/results/swebench/config.json`](bench/results/swebench/config.json), never from a CI
runner.

## What it does not do

- **It does not enforce anything.** No gate, no policy exit code, no expected-phase flag.
- **It does not replace CI.** `rtdd verify` is a convenience.
- **It is not sound program analysis.** This is risk-managed test selection and says so.
- **It does not reduce token cost.** There are no model calls in the hot path.
- **It is Python only.** Per-test attribution does not exist in the JavaScript or Go
  ecosystems: Istanbul and v8 coverage carry aggregate counters with no test dimension
  ([vitest#6735](https://github.com/vitest-dev/vitest/issues/6735) has requested it since
  October 2024), and Go's `-coverprofile` has no test dimension either while per-test
  isolation costs a prebuilt binary driven once per test. Adding a language is engine work,
  not a config file.
- **Coverage is blind in its own way.** It only knows paths some test actually took, and it
  attributes nothing to code executed at import time — which is why the uncovered report has
  a separate import-time class instead of calling dataclasses and enums untested.
- **`COVERAGE_CORE=ctrace` is forced**, so instrumented runs pay roughly 2× tracing overhead.
  With coverage.py's `sysmon` core — the default on Python 3.14+ — dynamic contexts are
  silently dropped with a warning and a zero exit, producing a mostly-empty map; RTDD treats
  that warning as fatal.

## Documentation

- [Design](docs/specs/2026-08-26-rtdd-design.md) — the full specification, including the
  measurements that killed three earlier design decisions.
- [Design audit](docs/audits/2026-08-26-design-audit.md) — what was measured, and what it
  killed.
- [Agent protocol](protocol/PROTOCOL.md) — the single source for every generated front-end
  under `dist/`. Edit it, run `rtdd-gen render`; CI fails if `dist/` is stale or if any
  front-end is wrong for its target.

## License

MIT. See [LICENSE](LICENSE).
EOF
```

- [ ] Paste the two results tables into the marked slots, add the MIT `LICENSE`, and verify no
      placeholder survives.

```bash
cd /home/claude/rtdd
grep -n 'PASTE' README.md
# expected: two lines — replace each with the contents of the named file, then:
grep -c 'PASTE' README.md
# expected: 0
grep -n 'TBD\|TODO\|XXX' README.md
# expected: no output
```

- [ ] Commit.

```bash
git add README.md LICENSE
git commit -m "docs: README with prior art before results

TDAD, pytest-testmon, Wallaby's MCP server, Infinitest, Ekstazi, and SonarQube's
new-code gate are cited up front, and the contribution is restated as the one
measurement rather than the idea (spec §3, audit A9).

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

## Task 23 — The honest negative outcome

> Take this branch when Task 14's human decision selected the "RTDD loses" or "kill criterion
> not met" row. Spec §15: *"If TDAD's static graph matches or beats RTDD on SWE-bench
> Verified, the honest outcome is to publish that result and contribute to TDAD rather than
> ship a competitor."* This task is that outcome, written in advance so it is not negotiated
> under disappointment.

**Files:**
- `README.md` (replaced, not amended)
- `docs/results/<date>-swebench-verified.md` (already written by Task 14)
- `.github/workflows/release.yml` (edited — release disabled)

**Interfaces:** none.

**What ships in this branch:**

| artifact | ships? | why |
|---|---|---|
| `docs/results/*-swebench-verified.md` | **yes** | The result is the deliverable. |
| `bench/` in full — harness, pre-registration, raw results | **yes** | A negative result nobody can rerun is not a result. |
| `bench/PREREGISTRATION.md` + `prereg-m4` tag | **yes** | It is what makes the negative credible. |
| `protocol/PROTOCOL.md`, `dist/`, `rtdd-gen` | **yes**, in-repo | Reusable, and the front-end generator is independently useful. |
| GoReleaser binaries, `install.sh`, a tagged release | **no** | Do not ship a competitor to a tool that beat you. |
| `rtdd init` as a recommended workflow | **no** | The README stops recommending it. |
| A PR to `pepealonso95/TDAD` adding a coverage-derived backend | **yes** | The contribution goes where it measured better. |

**Steps:**

- [ ] Replace the README. This is the full text; it is not a diff against Task 22's.

```bash
cat > /home/claude/rtdd/README.md <<'EOF'
# rtdd — a negative result

**This repository is a published measurement, not a tool to install.**

RTDD tested one hypothesis: that a **dynamic coverage** map, derived from real test
execution, would beat a **static dependency graph** at reducing the regressions an AI coding
agent introduces. It does not, on this benchmark, at this scale. The numbers are below, the
harness that produced them is in `bench/`, and the pre-registration that fixed the sample,
the arms, and the kill criterion before the run is in
[`bench/PREREGISTRATION.md`](bench/PREREGISTRATION.md) at git tag `prereg-m4`.

The binaries are not released. Use **[TDAD](https://github.com/pepealonso95/TDAD)** instead.

## The result

All five arms ran on one harness with one model (Qwen3-Coder-30B-A3B-Instruct, greedy, 40
turns) over a pre-registered 100-instance sample of SWE-bench Verified.

<!-- PASTE bench/results/swebench/tables.md HERE -->

A regression is defined mechanically in
[`bench/swebench/metrics.py`](bench/swebench/metrics.py): a test id in the evaluation
report's `tests_status.PASS_TO_PASS.failure` list. The RTDD arm's prompt is byte-for-byte the
vanilla arm's prompt plus one `<test-context>` block, asserted in
[a test](bench/swebench/tests/test_prompts.py), so the arm cannot be dismissed as a
procedural-prompt confound in either direction.

## Why, as far as we can tell

Stated as hypotheses, because a losing arm does not license a confident post-hoc story:

- **Coverage only knows paths some test already took.** A static graph sees a call edge the
  moment it is written; a coverage map sees it only after a test has run through it. For a
  benchmark of one-shot patches to unfamiliar code, the graph's speculative edges appear to
  be worth more than the coverage map's verified ones.
- **Import-time attribution is a real hole, not a footnote.** Python attributes every line
  executed during collection to no test at all, so dataclasses, enums, config modules, ORM
  models, decorators, and `__init__.py` re-exports carry no coverage edge. That class of file
  is enormous, and the static import scan that RTDD falls back to for it is exactly what the
  static graph does natively for everything.
- **File-level granularity may be the binding constraint**, not the static/dynamic axis.
  pytest-testmon has done method-level checksums since around 2016; a fair rerun of this
  hypothesis would test dynamic-and-method-level against static-and-method-level, and this
  benchmark did not.
- **The 2× tracing tax is a real cost with no measured benefit here.** `COVERAGE_CORE=ctrace`
  is forced because coverage.py's faster `sysmon` core silently drops dynamic contexts, so
  every instrumented run pays for a map that did not pay for itself.

## What is still worth taking from here

- **`bench/swebench/`** — a five-arm SWE-bench Verified regression harness with a mechanical
  `PASS_TO_PASS` metric, a pre-registration gate that refuses to run unsigned, and an arm
  composition that makes procedural-prompt contamination structurally impossible. It is
  reusable for any test-context intervention, not just this one.
- **`protocol/` and `cmd/rtdd-gen`** — one source rendered into a Claude Code `SKILL.md`, an
  `AGENTS.md` block, and a Cursor `.mdc`, with per-target section sets, byte budgets, and
  per-target validity assertions, plus a CI check that catches both staleness and
  wrong-for-this-target content.
- **The uncovered-change report** — the line-granular covered / uncovered / import-time split
  has no equivalent in a static-graph tool, and it was not what this benchmark measured. It
  remains untested as a standalone intervention.

## Prior art

RTDD did not invent test impact analysis, and this result is a reason to say so more loudly
rather than less.

- **[TDAD](https://arxiv.org/abs/2603.17973)** — Alonso, Yovine and Braberman, March 2026 —
  builds the static source↔test dependency graph this project tried and failed to beat, and
  published 6.08% → 1.82% on SWE-bench Verified. Reference implementation:
  [pepealonso95/TDAD](https://github.com/pepealonso95/TDAD). TypeScript port:
  [fmguerreiro/tdad-ts](https://github.com/fmguerreiro/tdad-ts). **Use it.**
- **[pytest-testmon](https://www.testmon.org/blog/determining-affected-tests/)** — method-level
  AST checksums since around 2016, finer-grained than anything here.
- **[Wallaby.js](https://wallabyjs.com/docs/features/ai/)** — ships an MCP server exposing
  `wallaby_coveredLinesForTest` and `wallaby_allTestsForFileAndLine` to Claude Code and Cursor
  today.
- **[Infinitest](https://github.com/infinitest/infinitest)** (2007) — classpath impact
  analysis marketed for "tight TDD cycles" nearly two decades ago.
- **[Ekstazi](https://users.ece.utexas.edu/~gligoric/papers/GligoricETAL15Ekstazi.pdf)**
  (2015) — the reference result for file-level regression test selection, and the source of
  the 32%-end-to-end-reduction figure that should temper anyone's expectations here.
- **SonarQube's new-code coverage gate**, Codecov's patch status, and `diff-cover` — prior art
  for "changed code that no test covers is a problem".

Also in the lineage: Bazel, Google TAP, Azure DevOps TIA, `jest --findRelatedTests`, NCrunch,
Meta's predictive test selection.

## Documentation

- [Full result](docs/results/) — the standalone write-up.
- [Design](docs/specs/2026-08-26-rtdd-design.md) — the specification, including §15's
  commitment to publishing this outcome.
- [Design audit](docs/audits/2026-08-26-design-audit.md).

## License

MIT. See [LICENSE](LICENSE).
EOF
```

- [ ] Disable releases so a stray tag cannot ship a competitor.

```bash
cd /home/claude/rtdd
python3 - <<'EOF'
from pathlib import Path
p = Path(".github/workflows/release.yml")
text = p.read_text()
text = text.replace(
    "on:\n  push:\n    tags: ['v*']\n",
    "# Releases are disabled: the Axis 1 result was negative and spec §15 says do not\n"
    "# ship a competitor to a tool that measured better. Re-enable only after a new,\n"
    "# separately pre-registered benchmark says otherwise.\n"
    "on:\n  workflow_dispatch:\n",
)
p.write_text(text)
EOF
grep -n 'workflow_dispatch' .github/workflows/release.yml
# expected: one match
```

- [ ] Open the contribution to TDAD. This is the §15 commitment, not an optional nicety.

```bash
cd /tmp && gh repo fork pepealonso95/TDAD --clone --remote
cd /tmp/TDAD && git checkout -b coverage-backend
# Port internal/coverage (the .coverage SQLite reader and numbits decoder) and the
# import-time classification into TDAD's analyzer as an additional impact strategy
# alongside its four existing ones, keeping its CLI surface unchanged.
gh pr create --repo pepealonso95/TDAD \
  --title "Add a coverage-derived impact strategy" \
  --body "$(cat <<'BODY'
We built a coverage-derived test-impact map (RTDD) and benchmarked it head-to-head against
TDAD's static graph on SWE-bench Verified, running your reference implementation as an arm
rather than citing your numbers. Your static graph won. The full result, the harness, and the
pre-registration are at https://github.com/VocanicZ/rtdd.

Rather than ship a competitor, this PR contributes the part that might still be additive: a
coverage-derived impact strategy alongside your four existing ones, reading `.coverage`
SQLite directly for per-test contexts. It sees edges reached through dynamic dispatch,
dependency injection, and monkeypatching that an AST call graph reports as zero callers. It
is off by default and costs nothing when no `.coverage` file is present.

Also included: the import-time classification. Python attributes every line executed during
collection to no test, so dataclasses, enums, and config modules carry no coverage edge —
worth flagging explicitly rather than reporting as untested.

Happy to split this into smaller PRs or drop any part of it.
BODY
)"
```

- [ ] Commit.

```bash
git add README.md .github/workflows/release.yml
git commit -m "docs: publish the negative result and stop shipping a competitor

Spec §15 committed to this outcome before the benchmark ran. The result, the
harness, and the pre-registration ship; the binaries do not. The coverage backend
is contributed upstream to TDAD.

Co-Authored-By: Claude Opus 5 <noreply@anthropic.com>"
```

---

## Task 24 — HUMAN-IN-THE-LOOP: repo visibility and the release tag

> ## 🛑 STOP — HUMAN-IN-THE-LOOP
>
> **An autonomous agent MUST NOT run `gh repo edit --visibility public` or push a `v*` tag.**
> Making a repository public is irreversible in practice — it publishes the benchmark
> transcripts, the pre-registration, and the results to an audience that will read them
> without the surrounding conversation. Pushing a `v*` tag triggers the release workflow and
> puts binaries in front of people. Both are the human's call.

**Files:** none — this task changes repository state.

**Interfaces:** none.

**Steps:**

- [ ] Agent: run the pre-flight and print the checklist, then **stop**.

```bash
cd /home/claude/rtdd
go run ./cmd/rtdd-gen check
go run ./cmd/rtdd-gen verify
go test ./...
cd bench/swebench && uv run pytest -q && uv run python preflight.py
cd /home/claude/rtdd
grep -rn 'TBD\|TODO\|FIXME\|REPLACE_WITH' README.md protocol/PROTOCOL.md dist/ bench/PREREGISTRATION.md || echo "no placeholders"
git -C . log --oneline -1 prereg-m4
gh repo view --json visibility -q .visibility
```

- [ ] Agent: print exactly this and **stop**.

```
DECISION REQUIRED — repository visibility and release

Branch taken at Task 14: <positive | negative>
Kill criterion:        <MET | NOT MET>
Front-end checks:      <pass | fail>
Test suite:            <pass | fail>
Placeholders:          <none | list>
Pre-registration tag:  prereg-m4 -> <sha> <date>
Current visibility:    <private | public>

Two irreversible actions need your explicit go-ahead, separately:

  1. Make the repository public. This publishes the benchmark results, the raw
     per-instance records, the pre-registration, and the prior-art claims about TDAD,
     pytest-testmon, Wallaby, Infinitest, Ekstazi, and SonarQube. Read the README's
     prior-art section once more before saying yes — it names other people's work.

  2. Push a v* tag, which triggers GoReleaser and publishes binaries plus the
     curl | sh install path.
     <If the negative branch was taken, action 2 is off the table: releases are
     disabled and spec §15 says do not ship a competitor.>

I will not do either of these. Tell me which, if any, to proceed with.
```

- [ ] Human: if going public —

```bash
cd /home/claude/rtdd
gh repo edit --visibility public --accept-visibility-change-consequences
```

- [ ] Human: if releasing (positive branch only) —

```bash
cd /home/claude/rtdd
git tag -a v0.1.0 -m "rtdd v0.1.0 — Python adapter, uncovered-change report, Axis 1 and Axis 2 results"
git push origin v0.1.0
gh run watch
# The release is created as a draft. Review the notes and the attached
# bench/results/swebench/tables.md, then publish it by hand.
gh release view v0.1.0
```

- [ ] Human: verify the published install path actually works from a clean machine.

```bash
docker run --rm -it debian:12 bash -lc '
  apt-get update -qq && apt-get install -y -qq curl ca-certificates >/dev/null
  curl -fsSL https://raw.githubusercontent.com/VocanicZ/rtdd/main/install.sh | sh
  rtdd --version
'
# expected: checksum ok / installed /usr/local/bin/rtdd / rtdd 0.1.0
```

---

## Definition of Done

**M4 — SWE-bench Verified**

- [ ] `bench/PREREGISTRATION.md` has `status: SIGNED`, a numeric `stratified_recall_floor`
      written by a human, and is reachable from the `prereg-m4` tag.
- [ ] `preflight.py` refuses to launch when the pre-registration is unsigned, incomplete,
      untagged, or when `instances.txt` does not match its recorded sha256 — demonstrated,
      not assumed.
- [ ] `bench/swebench/instances.txt` is frozen at 100 ids, reproducible from seed 20260826,
      and the empty-`PASS_TO_PASS` exclusion count is published.
- [ ] "Regression" exists only as code: `metrics.from_report`, with the dataset-sourced
      denominator and the unapplied-patch rule under test.
- [ ] `test_prompts.py` passes, proving `build("rtdd") == build("vanilla") + context_block(...)`
      byte for byte, and that `RTDD_CONTEXT` trips no imperative in the blocklist.
- [ ] All five arms ran on one harness with one model: `vanilla`, `tdd`, `tdad`, `rtdd`,
      `rtdd_tdd`.
- [ ] The `tdad` arm ran TDAD's own reference implementation at a pinned sha — or, if it
      could not, `docs/results/tdad-caveat.md` ships and the tables are labelled accordingly.
- [ ] The vanilla arm is compared to TDAD's published 6.08% through the pre-registered
      equivalence band, and a divergence banner appears automatically if it falls outside.
- [ ] Both regression rates (test-level and instance-level) and the resolution rate are in
      every published table.
- [ ] Wilson CIs are published for every arm and pairwise verdicts state "no distinguishable
      difference" where the intervals overlap.
- [ ] The kill criterion is evaluated against M3's already-published stratified recall and
      reported as MET or NOT MET.
- [ ] Seed failures are published as a count, not dropped.
- [ ] `bench/results/swebench/config.json` records the RTDD commit and the disclosed
      hardware; `cost.json` records assumed and actual spend.
- [ ] A human confirmed the Ship / Don't ship row at Task 14.

**M5 — Protocol, front-ends, release**

- [ ] `protocol/PROTOCOL.md` is the only hand-edited source; `dist/SKILL.md`,
      `dist/AGENTS.md`, and `dist/cursor/rules/rtdd.mdc` are generated.
- [ ] `rtdd-gen check` fails on staleness; `rtdd-gen verify` fails on per-target wrongness
      that is not staleness — both demonstrated.
- [ ] Each target has its own section set, byte budget, and validity assertions; an
      over-budget render is an error, never a truncation.
- [ ] `AGENTS.md` is under 1800 bytes and rejects the skill body leaking in; the `.mdc` has
      `globs` and `alwaysApply: false`; the `SKILL.md` has `name:`/`description:`
      frontmatter.
- [ ] The three targets render measurably different content (`agents < mdc < skill` by
      length), asserted in a test.
- [ ] CI runs the drift check, the validity check, the golden tests, the embedded-copy diff,
      and a guard against hand edits under `dist/`.
- [ ] `rtdd init` merges into an existing `AGENTS.md`/`CLAUDE.md` between its own markers,
      never touching content outside them, is idempotent, and errors rather than guesses on
      an ambiguous marker state.
- [ ] GoReleaser produces static (`CGO_ENABLED=0`) binaries for linux/darwin amd64+arm64 and
      windows/amd64, verified with `ldd` reporting "not a dynamic executable".
- [ ] `install.sh` verifies the sha256 against `checksums.txt` before installing, and was
      tested from a clean container.
- [ ] The README's prior-art section names TDAD, pytest-testmon, Wallaby's MCP server,
      Infinitest, Ekstazi, and SonarQube's new-code gate, and appears **before** the results.
- [ ] The README contains no `TBD`, `TODO`, or unfilled paste marker.
- [ ] Whichever branch was taken, the result is published (spec §15).
- [ ] A human made the visibility and release-tag decisions at Task 24.

---

## Ship / Don't ship

Keyed on the Axis 1 outcome. The kill criterion dominates every other row: it was signed
before the run, so it is not renegotiated after it.

| # | Condition | What ships | README |
|---|---|---|---|
| 1 | **Kill criterion NOT MET** (M3 single-killer recall below the signed floor) — regardless of every Axis 1 number | Results, harness, pre-registration, `protocol/` + `rtdd-gen`. **No binaries, no release, no `rtdd init` recommendation.** | Task 23's negative README, with the failure attributed to selector recall rather than to the arms |
| 2 | Kill criterion MET, **`tdad` beats `rtdd_tdd`** with disjoint CIs | Results, harness, pre-registration, `protocol/` + `rtdd-gen`. **No binaries.** Coverage backend contributed upstream to TDAD | Task 23's negative README |
| 3 | Kill criterion MET, **CIs overlap** between `tdad` and `rtdd_tdd` | Everything, tagged `v0.1.0` | Task 22, leading with "no distinguishable difference from a static graph on this benchmark"; no superiority claim anywhere; PR the coverage backend to TDAD as an additive strategy |
| 4 | Kill criterion MET, **`rtdd_tdd` beats `tdad`** with disjoint CIs **and** `rtdd` beats `vanilla` with disjoint CIs | Everything, tagged `v0.1.0` | Task 22, with the win stated at the measured effect size and CI, scoped to one model and one 100-instance sample |
| 5 | Kill criterion MET, **`rtdd_tdd` beats `tdad`** but `rtdd` does **not** beat `vanilla` | Everything, tagged `v0.1.0` | Task 22, but the headline is the parity arm only; state plainly that context without prose did not separate from vanilla, which is a weaker result than TDAD's |
| 6 | **HARNESS DIVERGENCE** — local vanilla outside the pre-registered band — in combination with any row above | As the matching row above | The matching README **plus** the divergence banner, with TDAD's published figures removed from the comparison columns and every claim restated in local-relative form |
| 7 | The `tdad` arm could **not** be run locally (contingency in Task 7) | As row 3, never rows 4 or 5 | Task 22 with `docs/results/tdad-caveat.md` linked from the results section; a head-to-head ranking is not reportable from two deltas on two harnesses |
