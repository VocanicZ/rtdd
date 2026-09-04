"""The corpus must meet the criteria it publishes, and something must check.

`corpus.yaml` v1 stated seven admission criteria and enforced none of them: they were
prose in a YAML header that no code read. `sqlfluff` was admitted at ~1450 s
uninstrumented — 2.4x over the same 10-minute budget cited, by name, as the reason
`pandas` was excluded — and nothing noticed for the length of a frozen corpus and
three published results.

These tests are what notices. The first one is the recorded breach: it reconstructs
`sqlfluff` at its measured v1 numbers and asserts the validator fails it. That record
is kept executable rather than only written down, because a number in a document
cannot fail a build.
"""

from __future__ import annotations

import hashlib
import pathlib

import pytest

from replay.corpus import (
    Corpus,
    CorpusNotFrozenError,
    RepoSpec,
    audit,
    load_corpus,
)

BENCH = pathlib.Path(__file__).resolve().parents[1]

ADMISSION = {
    "min_collected_tests": 400,
    "max_uninstrumented_full_suite_seconds": 600,
    "max_compiled_extensions_on_import_path": 0,
}


def _spec(repo_id: str, replay_commits: int = 2, **measured) -> RepoSpec:
    return RepoSpec(
        id=repo_id,
        url=f"https://example.invalid/{repo_id}.git",
        pin="1" * 40,
        replay_commits=replay_commits,
        python="3.12",
        install=("uv pip install -e .",),
        source_globs=("src/**/*.py",),
        test_globs=("test/**/*.py",),
        measured=measured,
    )


def _corpus(*specs: RepoSpec, admission: dict | None = None) -> Corpus:
    return Corpus(
        frozen_at="2026-08-26",
        criteria=("a criterion",),
        repos={s.id: s for s in specs},
        excluded=({"url": "https://example.invalid/x.git", "reason": "not hermetic"},),
        digest="0" * 64,
        version=1,
        admission=ADMISSION if admission is None else admission,
    )


def test_the_sqlfluff_breach_v1_never_caught():
    """The recorded breach, at the numbers actually measured on the disclosed hardware."""
    corpus = _corpus(
        _spec(
            "sqlfluff",
            collected_tests=13463,
            uninstrumented_full_suite_seconds=1450,
            compiled_extensions_on_import_path=0,
        )
    )
    findings = audit(corpus)
    assert len(findings) == 1
    (finding,) = findings
    assert finding.repo_id == "sqlfluff"
    assert finding.criterion == "uninstrumented full suite"
    assert "1450 s" in finding.detail and "600 s" in finding.detail
    assert "2.4x over" in finding.detail


def test_a_repo_under_the_collected_test_floor_is_a_breach():
    findings = audit(
        _corpus(
            _spec(
                "httpx",
                collected_tests=380,
                uninstrumented_full_suite_seconds=20,
                compiled_extensions_on_import_path=0,
            )
        )
    )
    assert [f.criterion for f in findings] == ["collected tests"]
    assert "380 collected" in findings[0].detail


def test_a_compiled_extension_on_the_import_path_is_a_breach():
    findings = audit(
        _corpus(
            _spec(
                "numpy",
                collected_tests=50000,
                uninstrumented_full_suite_seconds=300,
                compiled_extensions_on_import_path=42,
            )
        )
    )
    assert [f.criterion for f in findings] == ["compiled extensions"]


def test_an_unmeasured_repo_is_a_breach_not_a_pass():
    """Unverifiable is not the same as compliant — that conflation is the v1 defect."""
    findings = audit(_corpus(_spec("mystery")))
    assert [f.criterion for f in findings] == ["measurement"]


def test_a_partially_measured_repo_is_a_breach_per_missing_measurement():
    findings = audit(_corpus(_spec("half", collected_tests=900)))
    assert {f.criterion for f in findings} == {
        "uninstrumented full suite",
        "compiled extensions",
    }
    assert all(f.detail == "not measured" for f in findings)


def test_declaring_more_depth_than_was_reachable_is_a_breach():
    """v1 declared `replay_commits: 200` for every repo; no repo could reach it."""
    findings = audit(
        _corpus(
            _spec(
                "httpie",
                replay_commits=200,
                collected_tests=1028,
                uninstrumented_full_suite_seconds=124,
                compiled_extensions_on_import_path=0,
                replay_commits_ceiling=17,
            )
        )
    )
    assert [f.criterion for f in findings] == ["replay depth"]
    assert "declares 200" in findings[0].detail and "ceiling is 17" in findings[0].detail


def test_a_corpus_with_no_thresholds_can_decide_nothing():
    """Exactly what v1 was: criteria stated, none of them machine-checkable."""
    findings = audit(_corpus(_spec("anything"), admission={}))
    assert [f.criterion for f in findings] == ["admission"]


# ── the shipped corpus ────────────────────────────────────────────────────────


def test_the_shipped_corpus_passes_its_own_audit():
    corpus = load_corpus(BENCH / "corpus.yaml", BENCH / "corpus.lock")
    assert audit(corpus) == (), "the shipped corpus breaches its own criteria"


def test_the_shipped_corpus_no_longer_admits_sqlfluff():
    corpus = load_corpus(BENCH / "corpus.yaml", BENCH / "corpus.lock")
    assert "sqlfluff" not in corpus.ids()
    assert any("sqlfluff" in e["url"] for e in corpus.excluded), (
        "a dropped repo must appear in `excluded:` with a reason; "
        "exclusions are where cherry-picking hides"
    )


def test_every_admitted_repo_records_the_hardware_it_was_measured_on():
    corpus = load_corpus(BENCH / "corpus.yaml", BENCH / "corpus.lock")
    for spec in corpus.repos.values():
        assert spec.measured.get("on_hardware"), (
            f"{spec.id} has measurements with no hardware fingerprint; "
            f"a duration without its machine is not a measurement"
        )


# ── versioning: an old result stays reproducible ──────────────────────────────


def test_the_superseded_version_still_loads_at_its_published_digest():
    """Every result in `bench/results/` stamps this digest; it must stay resolvable."""
    v1 = load_corpus(BENCH / "corpus.yaml", BENCH / "corpus.lock", version=1)
    assert v1.version == 1
    assert v1.digest == "95cd1505766b560d0a595ee874531eb98e95522efdd06fdf6a6a1680c40a9381"
    assert "sqlfluff" in v1.ids(), "v1 is the corpus the published results were produced under"


def test_the_archived_version_is_kept_byte_for_byte():
    archived = BENCH / "corpus.d" / "v1.yaml"
    digest = hashlib.sha256(archived.read_bytes()).hexdigest()
    assert digest == "95cd1505766b560d0a595ee874531eb98e95522efdd06fdf6a6a1680c40a9381", (
        "re-freezing must not rewrite an archived version; its digest is what makes "
        "the results published under it reproducible"
    )


def test_the_lock_records_every_version_not_just_the_current_one():
    lock = (BENCH / "corpus.lock").read_text(encoding="utf-8")
    versions = {
        int(line.split()[0])
        for line in lock.splitlines()
        if line.strip() and not line.lstrip().startswith("#")
    }
    assert versions >= {1, 2}, "the lock is a history; dropping a line orphans a published result"


def test_asking_for_a_version_that_was_never_archived_refuses():
    with pytest.raises(CorpusNotFrozenError, match="not archived"):
        load_corpus(BENCH / "corpus.yaml", BENCH / "corpus.lock", version=99)


def test_published_results_all_stamp_a_digest_the_lock_still_knows():
    """The re-freeze contract, asserted: no published result is orphaned."""
    import json

    lock = (BENCH / "corpus.lock").read_text(encoding="utf-8")
    known = {
        line.split()[1]
        for line in lock.splitlines()
        if line.strip() and not line.lstrip().startswith("#")
    }
    seen = 0
    for config in sorted((BENCH / "results").glob("*/config.json")):
        raw = json.loads(config.read_text(encoding="utf-8"))
        # `results/swebench/` is Axis 1 and stamps a different config shape; it is
        # not produced from the corpus and has no digest to orphan.
        digest = raw.get("config", {}).get("corpus_digest")
        if digest is None:
            continue
        assert digest in known, (
            f"{config.parent.name} was published under corpus digest {digest[:12]}, which "
            f"corpus.lock no longer records — that result is unverifiable"
        )
        seen += 1
    assert seen, "no published results found to check"
