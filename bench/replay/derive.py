"""The `static` arm — RTDD's `TS` tier, derived offline from committed records.

`static` is the M6e system under test, not a PRD #4 baseline, and it is never
executed: it is COMPUTED from `bench/results/<repo>/commits.jsonl`, which already
holds, per replayed commit, the `changed` set, the `all_tests` the commit collected,
and every other arm's `selected` list. Nothing here clones, provisions, materialises a
worktree or runs a test.

The tier it models admits a test on two signals and orders on three (spec §4.1,
resolved in `docs/plans/06-m6b-static-tier.md` decision 1). The rule is stated exactly,
level by level, so it is not quietly improved later:

1. **Declared `test_for` correspondence — admits.** A changed source path resolves
   through `STATIC_TEST_FOR`, in declaration order, to the FIRST template naming a test
   file this commit collected; that file contributes every id it collected. A changed
   path that is itself a collected test file contributes its own ids, as the engine's
   direct tier does. This level is a pure function of `CommitRecord.changed` and
   `CommitRecord.all_tests` — see `level1_tests`.
2. **Transitive import distance — admits.** Not a function of those two fields: it needs
   the import graph, which needs the tree. It is not recomputed and not skipped either,
   because it was already measured — the `importgraph` baseline is exactly "the test
   files that transitively import a changed file", it ran on every replayed commit, and
   its answer is committed. `derive_static` unions that committed record in, and
   `DERIVED_REASON` names it, so a reader is never left to assume the level was
   evaluated by this module.
3. **Longest shared directory prefix — orders only, never admits.** It cannot change the
   selected *set*, and change-level recall, selection ratio and selected-duration
   fraction are each a function of the set alone. Ranking is therefore irrelevant to
   every number this milestone publishes and is deliberately NOT reconstructed here.

Two consequences follow and are stated rather than left to a reader to infer:

- `static ⊇ importgraph` by construction, so the static arm beating the import-graph
  baseline is arithmetic and not a finding. The pre-registered comparison is against
  `path`, which it does not contain: `path` matches a test file by basename anywhere in
  the tree, level 1 matches only template-resolved paths, so neither set contains the
  other.
- A changed path that resolves to nothing contributes nothing. The tier under-selects
  there, and that under-selection is the property being measured.
"""

from __future__ import annotations

import posixpath
from collections.abc import Sequence, Set

from replay.records import CommitRecord, StrategyRecord

STATIC_TEST_FOR: tuple[str, ...] = (
    "{dir}/test_{name}.py",
    "{dir}/tests/test_{name}.py",
    "tests/{subdir}/test_{name}.py",
    "tests/test_{name}.py",
)
"""The `test_for` templates this model gives a Python static adapter.

`adapters/python.yaml` declares none and stays byte-frozen — it is the coverage
adapter, and the shipped tool would never reach `TS` on flask or httpie. These four
are the model, they are a PUBLISHED input rather than an implementation detail, and
changing them changes the number: the selection ratio, the selected-duration fraction
and therefore the §7 kill-condition verdict all move with them.

So they are printed beside the number they produce. `report.static_model_disclosure`
renders this tuple into every `summary.md`'s `## The static arm` section, reading it
from here rather than re-typing it, and `test_results_static_arm.py`'s
`test_the_published_markdown_prints_every_template_the_static_arm_models_with` asserts
the published markdown names every entry, so the two cannot drift (#366).
"""

DERIVED_ARMS: tuple[str, ...] = ("static",)
"""Arms that reach the published tables by derivation rather than by execution.

`static` is deliberately absent from `replay.strategies.REGISTRY`: registry membership
means the orchestrator may EXECUTE an arm against a materialised worktree, and executing
this one is the thing PRD #233 forbids. `bench/tests/test_registry_complete.py`'s
exact-equality assertion therefore stays exactly as it is.
"""

LEVEL2_SOURCE = "importgraph"

DERIVED_REASON = (
    "derived offline from committed records: level 1 test_for correspondence over "
    "all_tests, united with the committed importgraph selection for level 2; "
    "nothing was executed"
)


class DerivationError(ValueError):
    """The records cannot support a derivation, and guessing would publish a number."""


def _trailing_dirs(d: str) -> list[str]:
    """The values `{subdir}` takes, longest first — `trailingDirs` in testfor.go."""
    if not d or d == ".":
        return [d]
    segs = d.split("/")
    return ["/".join(segs[i:]) for i in range(len(segs))]


def test_for_candidate(rel: str, test_files: Set[str]) -> str | None:
    """The first template naming a file this commit COLLECTED, or None."""
    d = posixpath.dirname(rel)
    name = posixpath.splitext(posixpath.basename(rel))[0]
    subs = _trailing_dirs(d)
    for tmpl in STATIC_TEST_FOR:
        # A template naming no {subdir} has exactly one expansion, so trying it once per
        # trailing directory would ask the same question len(subs) times.
        tries = subs if "{subdir}" in tmpl else subs[:1]
        for sub in tries:
            cand = posixpath.normpath(
                tmpl.replace("{dir}", d).replace("{subdir}", sub).replace("{name}", name)
            )
            if cand in ("", "."):
                continue
            if cand in test_files:
                return cand
    return None


def level1_tests(rec: CommitRecord) -> tuple[str, ...]:
    """Every collected id in the test files this commit's changed set corresponds to."""
    test_files = {t.split("::", 1)[0] for t in rec.all_tests}
    hit: set[str] = set()
    for rel in rec.changed:
        if not rel.endswith(".py"):
            continue
        if rel in test_files:
            hit.add(rel)
            continue
        cand = test_for_candidate(rel, test_files)
        if cand is not None:
            hit.add(cand)
    return tuple(t for t in rec.all_tests if t.split("::", 1)[0] in hit)


def derive_static(records: Sequence[object]) -> list[StrategyRecord]:
    """One `static` record per `CommitRecord`, computed from the records themselves.

    A record whose level-2 half is missing is an error rather than a level-1-only
    answer: publishing half a tier under a name that claims both levels would be the
    same lie #275 filed against reporting a skipped level as one that reached nothing.
    """
    commits = [r for r in records if isinstance(r, CommitRecord)]
    level2 = {
        (r.commit, r.variant): r
        for r in records
        if isinstance(r, StrategyRecord) and r.strategy == LEVEL2_SOURCE
    }
    out: list[StrategyRecord] = []
    for rec in commits:
        src = level2.get((rec.commit, rec.variant))
        if src is None:
            raise DerivationError(
                f"{rec.repo_id} {rec.commit} ({rec.variant}): no {LEVEL2_SOURCE} record; "
                "the static arm's level 2 is that record and cannot be guessed"
            )
        known = set(rec.all_tests)
        unknown = sorted(set(src.selected) - known)
        if unknown:
            raise DerivationError(
                f"{rec.repo_id} {rec.commit} ({rec.variant}): {LEVEL2_SOURCE} selected "
                f"{unknown[0]!r}, which this commit never collected"
            )
        selected = tuple(sorted(set(level1_tests(rec)) | set(src.selected)))
        out.append(
            StrategyRecord(
                repo_id=rec.repo_id,
                commit=rec.commit,
                variant=rec.variant,
                strategy="static",
                selected=selected,
                escalated=False,
                reason=DERIVED_REASON,
                select_ms=0,
                derived=True,
            )
        )
    return out
