"""Schema-3 test ids (`file::name`) expanded to the ids pytest collected.

Recall and cost are scored per collected id, so this is the step that decides
whether a graph test counts at all. Every case is one shape real pytest output
has; `test_every_corpus_language_has_an_expansion_case` fails the day the corpus
admits a language nobody wrote a case for.
"""
from __future__ import annotations

import pathlib

import pytest

from replay.corpus import load_corpus
from replay.graphids import expand_graph_ids

BENCH = pathlib.Path(__file__).resolve().parents[1]

PY_COLLECTED = (
    "tests/test_calc.py::test_add",
    "tests/test_calc.py::TestCalc::test_neg",
    "tests/test_calc.py::TestCalc::TestNested::test_zero",
    "tests/test_calc.py::test_mul[1-1]",
    "tests/test_calc.py::test_mul[2-4]",
    "tests/test_calc.py::test_mul[a::b]",
    "tests/test_other.py::test_add",
)

#: language -> (graph ids, collected ids, expected expansion, expected unmatched)
CASES = {
    "python": (
        (
            "tests/test_calc.py::test_add",
            "tests/test_calc.py::TestCalc::test_neg",
            "tests/test_calc.py::TestCalc::TestNested::test_zero",
            "tests/test_calc.py::test_mul",
            "tests/test_calc.py::helper",
        ),
        PY_COLLECTED,
        (
            "tests/test_calc.py::test_add",
            "tests/test_calc.py::TestCalc::test_neg",
            "tests/test_calc.py::TestCalc::TestNested::test_zero",
            "tests/test_calc.py::test_mul[1-1]",
            "tests/test_calc.py::test_mul[2-4]",
            "tests/test_calc.py::test_mul[a::b]",
        ),
        ("tests/test_calc.py::helper",),
    ),
}

#: A corpus row's language, read from its source globs' extension.
EXTENSIONS = {".py": "python"}


@pytest.mark.parametrize("language", sorted(CASES))
def test_graph_ids_expand_to_collected_ids(language):
    graph_ids, collected, want, unmatched = CASES[language]
    assert expand_graph_ids(graph_ids, collected) == (want, unmatched)


def test_every_corpus_language_has_an_expansion_case():
    corpus = load_corpus(BENCH / "corpus.yaml", BENCH / "corpus.lock")
    languages = set()
    for repo_id in corpus.ids():
        for glob in corpus.require(repo_id).source_globs:
            ext = pathlib.PurePosixPath(glob).suffix
            assert ext in EXTENSIONS, f"{repo_id}: no language known for {glob!r}"
            languages.add(EXTENSIONS[ext])
    assert languages, "the corpus names no source language"
    assert languages <= set(CASES), f"no expansion case for {sorted(languages - set(CASES))}"


def test_the_file_is_part_of_the_key():
    got, _ = expand_graph_ids(("tests/test_other.py::test_add",), PY_COLLECTED)
    assert got == ("tests/test_other.py::test_add",)


def test_the_duplicate_definition_suffix_still_matches():
    got, unmatched = expand_graph_ids(("tests/test_calc.py::test_add@12",), PY_COLLECTED)
    assert got == ("tests/test_calc.py::test_add",)
    assert unmatched == ()


def test_a_helper_in_a_test_file_matches_no_collected_id():
    got, unmatched = expand_graph_ids(("tests/test_calc.py::helper",), PY_COLLECTED)
    assert got == ()
    assert unmatched == ("tests/test_calc.py::helper",)


def test_a_graph_id_never_widens_to_its_whole_file():
    got, _ = expand_graph_ids(("tests/test_calc.py::gone",), PY_COLLECTED)
    assert got == ()


def test_output_is_deduplicated_in_first_seen_order():
    got, _ = expand_graph_ids(
        ("tests/test_calc.py::test_add", "tests/test_calc.py::test_add@9"), PY_COLLECTED
    )
    assert got == ("tests/test_calc.py::test_add",)
