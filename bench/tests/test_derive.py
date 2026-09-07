"""The offline model of the `TS` static tier — the `static` arm, derived not executed.

Level 1 is the half that is a pure function of `CommitRecord`: the changed set and
the test ids the commit collected are all it reads. The engine resolves a `test_for`
template against the filesystem; here the collected test files stand in for it, which
is the more conservative reading — a test file that exists but collects nothing cannot
fail, so admitting it would inflate the selection and never the recall.

Level 2 is not a function of those fields and is not recomputed from them: the
`importgraph` baseline already measured exactly that question on every replayed commit
and its answer is committed, so `derive_static` unions that record in and refuses the
commit outright when it is missing. Level 3 orders and never admits, so it cannot move
any number this milestone publishes and is not reconstructed at all.
"""

from __future__ import annotations

import pytest

from replay.derive import (
    DERIVED_ARMS,
    STATIC_TEST_FOR,
    DerivationError,
    derive_static,
    level1_tests,
)

# Aliased under a leading underscore for the same reason `tests_in_files` is in
# test_strategy_pathheuristic.py: pytest collects any imported name matching `test*`,
# and a resolver called as a test function errors on a missing `rel` fixture.
from replay.derive import test_for_candidate as _test_for_candidate
from replay.records import CommitRecord, StrategyRecord
from replay.strategies.base import all_ids


def commit(changed, all_tests) -> CommitRecord:
    return CommitRecord(
        repo_id="synth", commit="c1", parent="c0", variant="natural",
        all_tests=tuple(all_tests), durations_ms={t: 1 for t in all_tests},
        f_full=(), pre_existing_failures=(), changed=tuple(changed),
    )


def test_the_templates_are_declared_in_confidence_order():
    """The order IS the model, and it is published in summary.md. A co-located test is
    more specific evidence than a same-named file in a central tests/ directory, so it is
    tried first — the same reason `Adapter.TestForCandidate` tries templates in the order
    the adapter author declared them."""
    assert STATIC_TEST_FOR == (
        "{dir}/test_{name}.py",
        "{dir}/tests/test_{name}.py",
        "tests/{subdir}/test_{name}.py",
        "tests/test_{name}.py",
    )


def test_the_first_template_naming_a_collected_file_wins():
    files = {"src/flask/tests/test_app.py", "tests/test_app.py"}
    assert _test_for_candidate("src/flask/app.py", files) == "src/flask/tests/test_app.py"


def test_a_template_that_names_nothing_collected_is_skipped():
    """Expansion alone is not evidence: every template always expands, so a candidate
    counts only when the commit actually collected that file."""
    assert _test_for_candidate("src/flask/app.py", {"tests/test_other.py"}) is None


def test_subdir_mirrors_a_suffix_of_the_source_tree_longest_first():
    """`{subdir}` takes the changed file's directory, then that directory with one leading
    segment dropped, and so on — `internal/adapter/testfor.go`'s `trailingDirs`. A test
    tree usually mirrors a suffix of the source tree, not the whole of it."""
    files = {"tests/flask/test_app.py"}
    assert _test_for_candidate("src/flask/app.py", files) == "tests/flask/test_app.py"


def test_a_changed_test_file_contributes_its_own_collected_ids():
    """The engine's direct tier always runs a changed test file. The model does the same,
    and only for a file the commit collected — a deleted test cannot run."""
    rec = commit(["tests/test_app.py"], ["tests/test_app.py::a", "tests/test_other.py::b"])
    assert level1_tests(rec) == ("tests/test_app.py::a",)


def test_a_non_python_change_contributes_nothing():
    """A changed CHANGES.rst has no module and no correspondence. The tier under-selects
    there, and that is the property being measured, not a bug to paper over."""
    rec = commit(["CHANGES.rst"], ["tests/test_app.py::a"])
    assert level1_tests(rec) == ()


def test_level1_expands_a_resolved_file_to_every_id_it_collected():
    rec = commit(
        ["src/flask/app.py"],
        ["tests/test_app.py::a", "tests/test_app.py::b", "tests/test_cli.py::c"],
    )
    assert level1_tests(rec) == ("tests/test_app.py::a", "tests/test_app.py::b")


# --- level 2, and the whole arm: derive_static over committed records ---------


def ig(selected, commit_id="c1") -> StrategyRecord:
    return StrategyRecord("synth", commit_id, "natural", "importgraph", tuple(selected),
                          False, "static import closure", 7)


def test_static_is_level1_united_with_the_committed_import_graph():
    """Level 2 — transitive imports — is not a function of CommitRecord's fields, and it
    does not have to be: the importgraph baseline already measured exactly that question
    on every replayed commit, and its answer is committed."""
    rec = commit(["src/flask/app.py"], ["tests/test_app.py::a", "tests/test_cli.py::c"])
    out = derive_static([rec, ig(["tests/test_cli.py::c"])])
    assert len(out) == 1
    assert out[0].strategy == "static"
    assert set(out[0].selected) == {"tests/test_app.py::a", "tests/test_cli.py::c"}


def test_a_derived_record_carries_no_measurement_and_says_so():
    rec = commit(["src/flask/app.py"], ["tests/test_app.py::a"])
    out = derive_static([rec, ig([])])
    assert out[0].derived is True
    assert out[0].select_ms == 0
    assert out[0].escalated is False
    assert "derived" in out[0].reason and "importgraph" in out[0].reason


def test_the_selection_is_ordered_so_the_record_is_byte_stable():
    """bench/results/ is committed and diffable; a set iterated in hash order would make
    the same derivation produce a different file on a different run."""
    rec = commit(["src/flask/app.py"], ["tests/test_app.py::b", "tests/test_app.py::a"])
    first = derive_static([rec, ig(["tests/test_app.py::a"])])
    second = derive_static([rec, ig(["tests/test_app.py::a"])])
    assert first[0].selected == second[0].selected
    assert list(first[0].selected) == sorted(first[0].selected)


def test_an_id_the_commit_never_collected_is_refused():
    """Scoring an id that cannot match ground truth would silently inflate the
    denominator — `UnknownTestError`'s argument in strategies/base.py, applied to a
    derivation that reads a peer's record rather than a strategy's answer."""
    rec = commit(["src/flask/app.py"], ["tests/test_app.py::a"])
    with pytest.raises(DerivationError, match="never collected"):
        derive_static([rec, ig(["tests/test_ghost.py::z"])])


def test_a_commit_with_no_import_graph_record_is_refused_not_guessed():
    """A missing level-2 half is missing DATA. Treating it as an empty selection would
    publish a level-1-only arm under a name that claims both levels."""
    rec = commit(["src/flask/app.py"], ["tests/test_app.py::a"])
    with pytest.raises(DerivationError, match="importgraph"):
        derive_static([rec])


def test_re_deriving_replaces_rather_than_duplicates():
    rec = commit(["src/flask/app.py"], ["tests/test_app.py::a"])
    once = derive_static([rec, ig([])])
    twice = derive_static([rec, ig([]), *once])
    assert twice == once


def test_static_is_not_in_the_strategy_registry():
    """REGISTRY membership means the orchestrator may EXECUTE this arm against a
    materialised worktree, and executing it is the one thing PRD #233 forbids. The arm
    reaches the published tables through DERIVED_ARMS instead, and
    test_registry_complete.py's exact-equality assertion stays exactly as it is."""
    assert "static" in DERIVED_ARMS
    assert "static" not in all_ids()


def test_a_derived_selection_only_ever_names_ids_the_commit_collected():
    """The union of both levels is checked against `all_tests`, not just the peer's half:
    a level-1 resolution is drawn from the collected files by construction, and this is
    the test that keeps it that way if the resolver ever stops reading them."""
    rec = commit(
        ["src/flask/app.py", "tests/test_app.py"],
        ["tests/test_app.py::a", "tests/test_cli.py::c"],
    )
    out = derive_static([rec, ig(["tests/test_cli.py::c"])])
    assert set(out[0].selected) <= set(rec.all_tests)


def test_a_changed_source_file_with_no_correspondence_selects_nothing():
    """The under-selection IS the property being measured. A tier that fell back to a
    directory or a package would answer more often and measure something else."""
    rec = commit(["src/flask/orphan.py"], ["tests/test_app.py::a"])
    out = derive_static([rec, ig([])])
    assert out[0].selected == ()
    assert out[0].escalated is False
