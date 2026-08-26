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
