"""RTDD static import scan.

Reads a JSON request on stdin:
    {"root": "<abs repo root>", "targets": ["rel/path.py", ...], "tests": ["rel/test.py", ...]}
Writes a JSON object on stdout:
    {"<target>": ["<test rel path>", ...], ...}   sorted, one key per target.

A test is selected for a target when the test module transitively imports the target
module. Import cycles terminate via a visited set. Unparseable files contribute no edges
rather than aborting the scan.
"""
import ast
import json
import os
import sys

SKIP_DIRS = {".git", ".venv", "venv", "__pycache__", ".tox", "node_modules", ".rtdd",
             ".mypy_cache", ".pytest_cache", "build", "dist", ".eggs"}


def mod_name(rel):
    parts = rel[:-3].split("/")
    if parts[-1] == "__init__":
        parts = parts[:-1]
    return ".".join(parts)


def py_files(root):
    out = []
    for dirpath, dirnames, filenames in os.walk(root):
        dirnames[:] = [d for d in dirnames if d not in SKIP_DIRS]
        for fn in filenames:
            if fn.endswith(".py"):
                rel = os.path.relpath(os.path.join(dirpath, fn), root)
                out.append(rel.replace(os.sep, "/"))
    return sorted(out)


def imports_of(root, rel, self_mod):
    path = os.path.join(root, rel)
    try:
        with open(path, "rb") as fh:
            tree = ast.parse(fh.read(), filename=rel)
    except (SyntaxError, OSError, ValueError):
        return set()
    if rel == "__init__.py" or rel.endswith("/__init__.py"):
        pkg = self_mod
    else:
        pkg = self_mod.rsplit(".", 1)[0] if "." in self_mod else ""
    out = set()
    for node in ast.walk(tree):
        if isinstance(node, ast.Import):
            for al in node.names:
                out.add(al.name)
        elif isinstance(node, ast.ImportFrom):
            if node.level:
                base = pkg.split(".") if pkg else []
                if node.level > 1:
                    base = base[: len(base) - (node.level - 1)]
                prefix = ".".join([p for p in base if p])
                mod = prefix + ("." + node.module if node.module else "")
            else:
                mod = node.module or ""
            if mod:
                out.add(mod)
            for al in node.names:
                out.add((mod + "." + al.name) if mod else al.name)
    return out


def main():
    req = json.load(sys.stdin)
    root = req["root"]
    targets = req.get("targets") or []
    tests = req.get("tests") or []

    files = py_files(root)
    mod2rel = {}
    for rel in files:
        mod2rel.setdefault(mod_name(rel), rel)

    edges = {}
    for rel in files:
        raw = imports_of(root, rel, mod_name(rel))
        edges[rel] = sorted({mod2rel[x] for x in raw if x in mod2rel})

    result = {}
    for tgt in targets:
        hits = []
        for t in tests:
            if t not in edges:
                continue
            seen = {t}
            stack = [t]
            found = False
            while stack:
                cur = stack.pop()
                if cur == tgt:
                    found = True
                    break
                for nxt in edges.get(cur, ()):
                    if nxt not in seen:
                        seen.add(nxt)
                        stack.append(nxt)
            if found:
                hits.append(t)
        result[tgt] = sorted(hits)

    json.dump(result, sys.stdout, sort_keys=True)
    sys.stdout.write("\n")


main()
