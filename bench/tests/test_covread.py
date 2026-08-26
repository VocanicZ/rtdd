from __future__ import annotations

import pathlib
import sqlite3
import subprocess

from replay.covread import CoverageTruth, normalize_context, numbits, read_coverage
from replay.runner import run_full

# --- numbits ----------------------------------------------------------------


def test_numbits_decodes_the_packed_bitmap():
    assert numbits(b"") == []
    assert numbits(b"\x01") == [0]
    assert numbits(b"\x02") == [1]
    assert numbits(b"\x00\x81") == [8, 15]


def test_numbits_decodes_every_bit_of_a_full_byte():
    assert numbits(b"\xff") == [0, 1, 2, 3, 4, 5, 6, 7]


# --- context normalisation --------------------------------------------------


def test_normalize_context_strips_the_phase_suffix():
    assert normalize_context("tests/test_a.py::test_x|run") == ("tests/test_a.py::test_x", True)
    assert normalize_context("tests/test_a.py::test_x[1::2]|setup") == (
        "tests/test_a.py::test_x[1::2]",
        True,
    )
    assert normalize_context("") == ("", False)


def test_normalize_context_splits_on_the_last_pipe_only():
    """A parametrised id can contain a pipe of its own."""
    assert normalize_context("tests/test_pipe.py::test_pipe[a|b]|run") == (
        "tests/test_pipe.py::test_pipe[a|b]",
        True,
    )


def test_a_static_context_with_no_phase_suffix_is_still_a_test():
    assert normalize_context("some-static-context") == ("some-static-context", True)
    assert normalize_context("tests/test_a.py::test_x|weird") == (
        "tests/test_a.py::test_x|weird",
        True,
    )


def test_a_phase_suffix_with_no_test_id_is_not_a_test():
    assert normalize_context("|run") == ("", False)


# --- reading a real .coverage ----------------------------------------------


def _instrumented_truth(synth) -> CoverageTruth:
    subprocess.run(["git", "checkout", "-q", synth.sha(2)], cwd=synth.path, check=True)
    res = run_full(synth.path, instrumented=True, source_globs=("src",))
    assert res.exit_code == 0
    return read_coverage(synth.path / ".coverage", synth.path)


def test_read_coverage_separates_per_test_from_import_time(synth):
    truth = _instrumented_truth(synth)

    assert "tests/test_alpha.py::test_add" in truth.by_test
    body = ("src/alpha.py", 2)  # `return a + b`
    assert body in truth.by_test["tests/test_alpha.py::test_add"]
    assert body in truth.covered
    # the `def` line executes at import, attributed to no test
    assert ("src/alpha.py", 1) in truth.import_time


def test_covered_is_the_union_over_non_empty_contexts_only(synth):
    """A line that only ran at import was executed but asserted on by nothing;
    counting it as covered is exactly the false signal this metric exists for."""
    truth = _instrumented_truth(synth)

    assert ("src/alpha.py", 1) not in truth.covered
    union = frozenset().union(*truth.by_test.values())
    assert truth.covered == union


def test_every_test_of_the_synthetic_repo_gets_its_own_line_set(synth):
    truth = _instrumented_truth(synth)

    assert set(truth.by_test) == {
        "tests/test_alpha.py::test_add",
        "tests/test_beta.py::test_mul",
        "tests/test_gamma.py::test_sub",
    }
    assert ("src/beta.py", 2) in truth.by_test["tests/test_beta.py::test_mul"]
    assert ("src/gamma.py", 2) in truth.by_test["tests/test_gamma.py::test_sub"]
    # test_add never touches beta's body
    assert ("src/beta.py", 2) not in truth.by_test["tests/test_alpha.py::test_add"]


def test_read_coverage_keeps_paths_repo_relative_and_drops_outsiders(synth):
    truth = _instrumented_truth(synth)

    for pairs in list(truth.by_test.values()) + [truth.import_time]:
        for path, _line in pairs:
            assert not pathlib.Path(path).is_absolute()
            assert not path.startswith("..")


# --- synthetic stores: branch mode and the sentinels ------------------------


def _write_store(path: pathlib.Path, *, has_arcs: str | None) -> sqlite3.Connection:
    conn = sqlite3.connect(path)
    conn.executescript(
        """
        CREATE TABLE meta (key text, value text);
        CREATE TABLE file (id integer primary key, path text);
        CREATE TABLE context (id integer primary key, context text);
        CREATE TABLE line_bits (file_id integer, context_id integer, numbits blob);
        CREATE TABLE arc (file_id integer, context_id integer, fromno integer, tono integer);
        """
    )
    if has_arcs is not None:
        conn.execute("INSERT INTO meta (key, value) VALUES ('has_arcs', ?)", (has_arcs,))
    return conn


def test_a_branch_mode_store_is_read_from_the_arc_table(tmp_path):
    """With `--cov-branch` coverage.py writes zero line_bits rows and puts every
    executed line in `arc`. Reading only line_bits there yields an empty map on a
    run that exited 0 — the same silent corruption shape as a lost context."""
    db = tmp_path / ".coverage"
    conn = _write_store(db, has_arcs="1")
    conn.execute("INSERT INTO file (id, path) VALUES (1, ?)", (str(tmp_path / "src/a.py"),))
    conn.execute("INSERT INTO context (id, context) VALUES (1, 'tests/test_a.py::test_x|run')")
    conn.executemany(
        "INSERT INTO arc (file_id, context_id, fromno, tono) VALUES (1, 1, ?, ?)",
        [(-1, 1), (1, 2), (2, 4), (4, -1)],
    )
    conn.commit()
    conn.close()

    truth = read_coverage(db, tmp_path)

    assert truth.by_test["tests/test_a.py::test_x"] == frozenset(
        {("src/a.py", 1), ("src/a.py", 2), ("src/a.py", 4)}
    )


def test_non_positive_line_numbers_are_never_real_source_lines(tmp_path):
    """coverage records line 0 for an empty module frame and negative numbers as
    arc scope sentinels; neither is a line anyone can assert on."""
    db = tmp_path / ".coverage"
    conn = _write_store(db, has_arcs=None)
    conn.execute("INSERT INTO file (id, path) VALUES (1, ?)", (str(tmp_path / "src/a.py"),))
    conn.execute("INSERT INTO context (id, context) VALUES (1, 'tests/test_a.py::test_x|run')")
    conn.execute(
        "INSERT INTO line_bits (file_id, context_id, numbits) VALUES (1, 1, ?)",
        (b"\x03",),  # bits 0 and 1 -> line 0 (not real) and line 1
    )
    conn.commit()
    conn.close()

    truth = read_coverage(db, tmp_path)

    assert truth.by_test["tests/test_a.py::test_x"] == frozenset({("src/a.py", 1)})


def test_the_phases_of_one_test_are_unioned_into_a_single_entry(tmp_path):
    db = tmp_path / ".coverage"
    conn = _write_store(db, has_arcs=None)
    conn.execute("INSERT INTO file (id, path) VALUES (1, ?)", (str(tmp_path / "src/a.py"),))
    conn.executemany(
        "INSERT INTO context (id, context) VALUES (?, ?)",
        [(1, "tests/test_a.py::test_x|setup"), (2, "tests/test_a.py::test_x|run")],
    )
    conn.executemany(
        "INSERT INTO line_bits (file_id, context_id, numbits) VALUES (1, ?, ?)",
        [(1, b"\x02"), (2, b"\x04")],  # line 1 in setup, line 2 in run
    )
    conn.commit()
    conn.close()

    truth = read_coverage(db, tmp_path)

    assert set(truth.by_test) == {"tests/test_a.py::test_x"}
    assert truth.by_test["tests/test_a.py::test_x"] == frozenset(
        {("src/a.py", 1), ("src/a.py", 2)}
    )


def test_lines_measured_outside_the_repo_are_dropped(tmp_path):
    db = tmp_path / ".coverage"
    conn = _write_store(db, has_arcs=None)
    conn.executemany(
        "INSERT INTO file (id, path) VALUES (?, ?)",
        [(1, str(tmp_path / "src/a.py")), (2, "/usr/lib/python3/site-packages/x.py")],
    )
    conn.execute("INSERT INTO context (id, context) VALUES (1, 'tests/test_a.py::test_x|run')")
    conn.executemany(
        "INSERT INTO line_bits (file_id, context_id, numbits) VALUES (?, 1, ?)",
        [(1, b"\x02"), (2, b"\x02")],
    )
    conn.commit()
    conn.close()

    truth = read_coverage(db, tmp_path)

    assert truth.by_test["tests/test_a.py::test_x"] == frozenset({("src/a.py", 1)})


def test_a_store_with_no_has_arcs_row_is_read_as_line_mode(tmp_path):
    db = tmp_path / ".coverage"
    conn = _write_store(db, has_arcs=None)
    conn.execute("INSERT INTO file (id, path) VALUES (1, ?)", (str(tmp_path / "src/a.py"),))
    conn.execute("INSERT INTO context (id, context) VALUES (1, '')")
    conn.execute(
        "INSERT INTO line_bits (file_id, context_id, numbits) VALUES (1, 1, ?)", (b"\x02",)
    )
    conn.commit()
    conn.close()

    truth = read_coverage(db, tmp_path)

    assert truth.import_time == frozenset({("src/a.py", 1)})
    assert truth.covered == frozenset()
