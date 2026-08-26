"""Baseline 4 — the static import graph, the stand-in for a static dependency graph.

This baseline carries the project's actual research question (spec §3: does dynamic
coverage beat a static graph?), so it must be *good-faith*: transitive, package-aware,
and honest about relative imports. A deliberately weak static graph would rig the
comparison in RTDD's favour. Its one real weakness — a non-Python change has no module,
so the graph cannot see it — is a reportable property of static analysis, and is pinned
here rather than papered over.
"""

from __future__ import annotations

import pathlib
import subprocess

from replay.gitwork import Change, diff_changes
from replay.strategies.base import REGISTRY, CommitContext, get, validate_selection
from replay.strategies.importgraph import (
    ImportGraph,
    build_graph,
    module_name,
    parse_imports,
    reverse_reachable,
)


def _write(root: pathlib.Path, rel: str, text: str) -> None:
    target = root / rel
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(text, encoding="utf-8")


# --- the pure pieces ----------------------------------------------------------


def test_module_name():
    assert module_name("src/alpha.py") == "src.alpha"
    assert module_name("src/__init__.py") == "src"
    assert module_name("tests/test_alpha.py") == "tests.test_alpha"


def test_parse_imports_handles_both_forms():
    src = "import os\nfrom src.alpha import add\nfrom src import beta\n"
    assert parse_imports(src) == {"os", "src.alpha", "src.alpha.add", "src", "src.beta"}


def test_parse_imports_resolves_relative_imports_against_the_importing_package():
    assert parse_imports("from . import alpha\n", "pkg.sub") == {"pkg.sub", "pkg.sub.alpha"}
    assert parse_imports("from .alpha import add\n", "pkg.sub") == {
        "pkg.sub.alpha",
        "pkg.sub.alpha.add",
    }
    assert parse_imports("from .. import top\n", "pkg.sub") == {"pkg", "pkg.top"}
    assert parse_imports("from ..other import y\n", "pkg.sub") == {
        "pkg.other",
        "pkg.other.y",
    }


def test_parse_imports_survives_an_unparseable_file():
    assert parse_imports("def broken(:\n") == set()


def test_reverse_reachable_is_transitive():
    graph = {"tests.test_a": {"pkg.mid"}, "pkg.mid": {"pkg.low"}, "pkg.low": set()}
    assert reverse_reachable(graph, {"pkg.low"}) == {"pkg.low", "pkg.mid", "tests.test_a"}


def test_reverse_reachable_terminates_on_an_import_cycle():
    graph = {"a": {"b"}, "b": {"a"}}
    assert reverse_reachable(graph, {"a"}) == {"a", "b"}


# --- the graph builder --------------------------------------------------------


def test_build_graph_resolves_a_relative_import_inside_a_package_init(tmp_path):
    """`from . import alpha` in `pkg/__init__.py` is `pkg.alpha`, not `alpha`."""
    _write(tmp_path, "pkg/__init__.py", "from . import alpha\n")
    _write(tmp_path, "pkg/alpha.py", "VALUE = 1\n")
    graph = build_graph(tmp_path)
    assert "pkg.alpha" in graph["pkg"]


def test_build_graph_resolves_a_relative_import_inside_a_nested_package_init(tmp_path):
    _write(tmp_path, "pkg/__init__.py", "")
    _write(tmp_path, "pkg/sub/__init__.py", "from . import deep\n")
    _write(tmp_path, "pkg/sub/deep.py", "VALUE = 1\n")
    graph = build_graph(tmp_path)
    assert "pkg.sub.deep" in graph["pkg.sub"]


def test_build_graph_skips_vendored_and_generated_trees(tmp_path):
    _write(tmp_path, "src/alpha.py", "VALUE = 1\n")
    _write(tmp_path, ".venv/lib/thing.py", "import src.alpha\n")
    _write(tmp_path, "build/thing.py", "import src.alpha\n")
    graph = build_graph(tmp_path)
    assert set(graph) == {"src.alpha"}


def test_a_change_reaches_tests_transitively_through_a_package(tmp_path):
    _write(tmp_path, "pkg/__init__.py", "from .mid import bump\n")
    _write(tmp_path, "pkg/mid.py", "from pkg.low import base\n\n\ndef bump():\n    return base() + 1\n")
    _write(tmp_path, "pkg/low.py", "def base():\n    return 1\n")
    _write(tmp_path, "tests/test_top.py", "from pkg import bump\n\n\ndef test_bump():\n    assert bump() == 2\n")
    graph = build_graph(tmp_path)
    assert reverse_reachable(graph, {"pkg.low"}) >= {"pkg.mid", "pkg", "tests.test_top"}


# --- the strategy over the synthetic repo -------------------------------------


def test_graph_over_the_synth_repo_selects_the_right_tests(synth):
    subprocess.run(["git", "checkout", "-q", synth.sha(2)], cwd=synth.path, check=True)
    graph = build_graph(synth.path)
    assert "src.alpha" in graph["tests.test_alpha"]

    ctx = CommitContext(
        repo_id="synth",
        variant="natural",
        commit=synth.sha(2),
        parent=synth.sha(1),
        work=synth.path,
        changed=(Change("src/beta.py", "M"), Change("src/gamma.py", "M")),
        all_tests=(
            "tests/test_alpha.py::test_add",
            "tests/test_beta.py::test_mul",
            "tests/test_gamma.py::test_sub",
        ),
        source_globs=("src/**/*.py",),
        test_globs=("tests/**/*.py", "**/test_*.py"),
        python="python",
    )
    sel = ImportGraph().select(ctx)
    assert sel.tests == ("tests/test_beta.py::test_mul", "tests/test_gamma.py::test_sub")
    assert sel.escalated is False
    assert sel.reason == "transitive static import closure"
    validate_selection(sel, ctx)


def test_non_python_change_is_invisible_to_the_static_graph(synth):
    subprocess.run(["git", "checkout", "-q", synth.sha(2)], cwd=synth.path, check=True)
    ctx = CommitContext(
        repo_id="synth",
        variant="natural",
        commit=synth.sha(2),
        parent=synth.sha(1),
        work=synth.path,
        changed=(Change("data/fixtures.yaml", "M"),),
        all_tests=("tests/test_alpha.py::test_add",),
        source_globs=("src/**/*.py",),
        test_globs=("tests/**/*.py", "**/test_*.py"),
        python="python",
    )
    sel = ImportGraph().select(ctx)
    assert sel.tests == ()
    assert sel.escalated is False
    assert sel.reason == "no Python module in the changed set"


# --- verified against the synthetic repo, hand-computed per commit -------------
#
# | Commit | Changed set                                     | Import closure reaches       |
# |--------|-------------------------------------------------|------------------------------|
# | c1     | src/alpha.py                                    | tests.test_alpha             |
# | c2     | src/alpha.py, src/gamma.py, tests/test_gamma.py | tests.test_alpha, .test_gamma|
# | c3     | src/beta.py, src/gamma.py                       | tests.test_beta, .test_gamma |
ALPHA = "tests/test_alpha.py::test_add"
BETA = "tests/test_beta.py::test_mul"
GAMMA = "tests/test_gamma.py::test_sub"

SYNTH_EXPECTED = {
    1: ((ALPHA, BETA), (ALPHA,)),
    2: ((ALPHA, BETA, GAMMA), (ALPHA, GAMMA)),
    3: ((ALPHA, BETA, GAMMA), (BETA, GAMMA)),
}


def test_import_selection_over_the_synth_repo_matches_the_hand_computed_table(synth):
    for index, (collected, expected) in SYNTH_EXPECTED.items():
        subprocess.run(["git", "checkout", "-q", synth.sha(index)], cwd=synth.path, check=True)
        ctx = CommitContext(
            repo_id="synth",
            variant="natural",
            commit=synth.sha(index),
            parent=synth.sha(index - 1),
            work=synth.path,
            changed=tuple(diff_changes(synth.path, synth.sha(index - 1), synth.sha(index))),
            all_tests=collected,
            source_globs=("src/**/*.py",),
            test_globs=("tests/**/*.py", "**/test_*.py"),
            python="python",
        )
        sel = ImportGraph().select(ctx)
        assert sel.tests == expected, f"c{index}"
        assert sel.escalated is False, f"c{index}"
        validate_selection(sel, ctx)


def test_the_graph_passes_no_tuning_flags(synth):
    ctx = CommitContext(
        repo_id="synth",
        variant="natural",
        commit=synth.sha(3),
        parent=synth.sha(2),
        work=synth.path,
        changed=(Change("src/beta.py", "M"),),
        all_tests=(BETA,),
        source_globs=("src/**/*.py",),
        test_globs=("tests/**/*.py", "**/test_*.py"),
        python="python",
    )
    assert ImportGraph().select(ctx).exec_args == ()


def test_importgraph_is_registered_under_the_id_the_plan_fixes():
    assert ImportGraph.id == "importgraph"
    assert ImportGraph.needs_parent_state is False
    assert isinstance(get("importgraph"), ImportGraph)
    assert REGISTRY["importgraph"] is get("importgraph")
