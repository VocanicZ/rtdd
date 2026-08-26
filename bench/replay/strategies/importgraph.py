"""Baseline 4 — the static import graph.

The direct stand-in for a static dependency graph, and therefore the baseline that
carries the project's actual research question (spec §3: does dynamic coverage beat
a static graph?). It is deliberately *good-faith*: the closure is transitive, package
`__init__` files are read like any other module, and relative imports are resolved
against the importing module's own package. A weaker graph would rig the comparison.

Its one weakness is real and reportable rather than a bug: a changed non-Python file
has no module, so the graph cannot see it and the strategy selects nothing for it.
"""

from __future__ import annotations

import ast
import pathlib
import time

from replay.strategies.base import CommitContext, Selection, register

_SKIP_DIRS = frozenset({".git", ".venv", "venv", "build", "dist", "__pycache__", ".tox"})


def module_name(rel: str) -> str:
    """`src/alpha.py` -> `src.alpha`; `src/__init__.py` -> `src` (the package itself)."""
    p = pathlib.PurePosixPath(rel)
    parts = list(p.parts)
    if parts and parts[-1] == "__init__.py":
        parts = parts[:-1]
    else:
        parts[-1] = p.stem
    return ".".join(parts)


def _package_of(rel: str) -> str:
    """The package a module's relative imports resolve against.

    For `pkg/sub/__init__.py` that is `pkg.sub` — an `__init__` *is* its package, so
    `from . import x` means `pkg.sub.x`. For `pkg/sub/mod.py` it is `pkg.sub`.
    """
    mod = module_name(rel)
    if pathlib.PurePosixPath(rel).name == "__init__.py":
        return mod
    return mod.rsplit(".", 1)[0] if "." in mod else ""


def parse_imports(source: str, package: str = "") -> set[str]:
    """Absolute dotted names this source imports.

    Both the imported module and each imported name are recorded (`from src.alpha
    import add` yields `src.alpha` and `src.alpha.add`) because the name may itself be
    a submodule; the extra entry costs nothing and a missed edge would understate the
    static graph.
    """
    try:
        tree = ast.parse(source)
    except SyntaxError:
        return set()
    names: set[str] = set()
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            for a in node.names:
                names.add(a.name)
        elif isinstance(node, ast.ImportFrom):
            if node.level:
                base_parts = package.split(".") if package else []
                base_parts = base_parts[: len(base_parts) - (node.level - 1)]
                base = ".".join([*base_parts, node.module] if node.module else base_parts)
            else:
                base = node.module or ""
            if base:
                names.add(base)
                for a in node.names:
                    names.add(f"{base}.{a.name}")
    return names


def build_graph(work: pathlib.Path) -> dict[str, set[str]]:
    """module -> the modules it imports, over every Python file in the working tree."""
    graph: dict[str, set[str]] = {}
    for path in sorted(work.rglob("*.py")):
        rel_parts = path.relative_to(work).parts
        if any(part in _SKIP_DIRS for part in rel_parts):
            continue
        rel = path.relative_to(work).as_posix()
        source = path.read_text(encoding="utf-8", errors="replace")
        graph[module_name(rel)] = parse_imports(source, _package_of(rel))
    return graph


def reverse_reachable(graph: dict[str, set[str]], seeds: set[str]) -> set[str]:
    """Every module that imports a seed, transitively — the seeds included."""
    reverse: dict[str, set[str]] = {}
    for mod, deps in graph.items():
        for dep in deps:
            reverse.setdefault(dep, set()).add(mod)
    seen = set(seeds)
    stack = list(seeds)
    while stack:
        cur = stack.pop()
        for parent in reverse.get(cur, ()):
            if parent not in seen:
                seen.add(parent)
                stack.append(parent)
    return seen


class ImportGraph:
    id = "importgraph"
    needs_parent_state = False

    def prepare(self, ctx: CommitContext) -> None:
        return None

    def select(self, ctx: CommitContext) -> Selection:
        start = time.perf_counter()
        seeds = {module_name(c.path) for c in ctx.changed if c.path.endswith(".py")}
        if not seeds:
            return Selection(
                tests=(),
                escalated=False,
                reason="no Python module in the changed set",
                select_ms=int(round((time.perf_counter() - start) * 1000)),
            )
        graph = build_graph(ctx.work)
        impacted = reverse_reachable(graph, seeds)
        tests = tuple(
            sorted(t for t in ctx.all_tests if module_name(t.split("::", 1)[0]) in impacted)
        )
        return Selection(
            tests=tests,
            escalated=False,
            reason="transitive static import closure",
            select_ms=int(round((time.perf_counter() - start) * 1000)),
        )


register(ImportGraph())
