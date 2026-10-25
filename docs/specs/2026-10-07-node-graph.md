# Node graph: test what the change actually touches

Status: approved design, 2026-10-07. Supersedes [`2026-09-29-one-pipeline.md`](2026-09-29-one-pipeline.md)
(coverage pipeline, per-language adapters, file-level `map.jsonl`). Ships as **v0.3.0**, breaking.

## 1. Goal

rtdd is a **skill**: a testing process an AI agent follows on **any** codebase, with a small
binary that answers one question fast — *which tests does this change need, in what order*.

It builds a **code graph** whose nodes are functions, methods and classes, whose edges are code
relationships, and in which every test is a node linked to the code it calls. When code changes:

1. **Round 1** — the tests linked to the changed nodes (and every changed test itself).
2. **Round 2** — the tests linked to the changed nodes' **depth-1 neighbours**, minus Round 1.
3. **Round 3** — the full suite, run **once**, at the end of the task.

The agent runs every round with the project's own test command. rtdd never executes a test,
never needs a coverage tool, and has no per-language code path.

Success, measured:

- `rtdd which` on this repository answers in **under 1 s** with a warm graph cache, and a cold
  scan of a **10 000-file** repository finishes in **under 10 s**.
- The scanner fixture corpus (§4.4) covers **8 languages** — Python, Go, TypeScript/JavaScript,
  Java, Rust, Ruby, Lua, Bash — with no toolchain installed.
- On rtdd-bench, time in tests for Rounds 1+2 is reported against `/tdd` (full suite every
  cycle), and recall is reported for Round 1 alone and for Rounds 1+2. A slower or lower-recall
  row is published as it is.

## 2. Why the old design is removed

Measured on 2026-10-07 across the 12 repositories on the maintainer's machine: **none** used a
map. Two were never seeded; one host adapter (`bash`) and one (`luau`) were silently rejected
as pre-v3; Gradle's root-anchored globs found 0 of 58 tests in a multi-module build. Every
case fell back to "full suite" or, with no adapter, to *zero tests and exit 0*. Selection that
depends on a per-language coverage adapter does not survive contact with real repositories.

## 3. The graph

One internal model, whatever built it:

```
Node { id, file, name, kind (func|method|class|test), start, end, is_test }
Edge { from, to, relation (calls|method|inherits|implements|references) }
```

`file` is repo-relative, slash-separated. `start`/`end` are 1-based inclusive line numbers.
A test node's runnable id is `file::name`.

Two sources produce it (§4, §5). Only **code** nodes are used; graphify's `concept`,
`rationale` and `document` nodes and its `contains`, `conceptually_related_to`,
`semantically_similar_to`, `shares_data_with` and `cites` edges are dropped on load. `contains`
is dropped because file→everything-in-it would make every neighbour set the whole file.

## 4. The built-in scanner

Used when the project has no graphify graph, and **always** for the files in §5's stale set.
Pure text, no parser, no dependency.

### 4.1 Files

Tracked plus untracked-not-ignored files (`gitctx.ListFiles`), excluding: binary files (a NUL
byte in the first 8 KiB), files over **1 MiB**, and paths matching `scan_exclude`
(default `vendor/**`, `node_modules/**`, `third_party/**`, `**/*.min.js`, `dist/**`, `build/**`,
`.rtdd/**`, `graphify-out/**`, and the prose files `**/*.md`, `**/*.mdc`, `**/*.markdown`,
`**/*.rst`, `**/*.txt`, `**/*.adoc`). Prose is never code (§3): a code sample in a plan document must not
become a node that "calls" the real code. Like every default, `scan_exclude` in
`.rtdd/config.yaml` replaces the list wholesale.

### 4.2 Definitions

A line defines a node when it matches one of these patterns (leading whitespace allowed;
the name is the last identifier segment, so `function M.load(` and `func (s *S) Load(` both
name `load`/`Load`):

| kind | pattern (informal) | covers |
|---|---|---|
| func | `(async )?(def\|fn\|func\|function\|fun\|sub\|proc) [receiver] Name` | Python, Rust, Go, JS, Kotlin, Perl, Nim, Lua (`local function`) |
| class | `(export )?(abstract )?(class\|struct\|interface\|trait\|impl\|module\|object\|enum) Name` | most OO languages |
| func | `(const\|let\|var) Name = (async )?(function\|(...) =>)` | JS/TS assigned functions |
| func | `Name() {` / `function Name` | shell |
| method | `<type tokens> Name(<params>) <modifiers> {?` with no trailing `;`, Name not a control keyword, line not inside a func's or test's body | Java, C#, C, C++, Swift, PHP, Dart |
| test | `(it\|test\|describe\|context)(\s*)\((\s*)['"\`]Label` | JS/TS, Ruby-style block tests |

A captured name that is itself a definition keyword (`def fn func function fun sub proc
class struct interface trait impl module object enum`) is an anonymous literal — Go
`func(x int) {`, JS `onDone: function (err) {` — not a node; its body belongs to the node
that encloses it. Inside a func's or test's body the method row does not apply, so a
statement such as `if n := len(x); n > 0 {` or a call's continuation line
`a, len(b), f(c),` is never read as a definition; nested definitions are still found by
the other rows.

The full regex set lives in one Go file and is the only place language shapes appear.

### 4.3 Spans and edges

- **End line.** If the definition line opens a `{`, the node ends where braces balance
  (braces inside string and comment literals are skipped best-effort). Otherwise it ends at the
  line before the next non-blank line indented at or below the definition, or at EOF. Nested
  definitions are their own nodes; a line belongs to the **innermost** node containing it.
- **`method` edge** from a class node to every node nested directly inside it.
- **`calls` edge** from node A to the node(s) named `X` when `X(` appears as a whole word in
  A's body (excluding A's own definition line). The candidates are the nodes named `X` of
  A's **own file type**: if A's file has extension `.e` and any node named `X` lives in a
  file with extension `.e`, only those — a Go call never reaches a Python definition.
  Otherwise every node named `X` whose file's extension **shares a directory** with `.e`
  somewhere in the graph (some directory holds a node-bearing file of each), which keeps
  `.ts`→`.js`, `.c`→`.h` and similar cross-extension calls while a name no `.e` file
  defines — most often a builtin such as Python's `len(` — is not captured by a stray
  definition in another language. No language list is involved: extensions are compared
  as strings. *(Amended by issue #508: the original rule linked every candidate —
  "over-linking is accepted" — and on a 700-file Java project one node was linked from
  145 tests.)* The candidates are then **narrowed**, cheapest signal first, no parser; the
  first rule that finds a definition wins:
  1. **Receiver type.** For `obj.X(` (also `obj?.X(`, `obj->X(`, `obj::X(`, `obj:X(`),
     where `obj` is declared in A or an enclosing node as `Type obj`, `obj: Type` or
     `obj = new Type(` / `obj = Type(` / `obj := &Type{` (`const`/`let`/`var` are never
     a type): `Type`'s own `X`, else its supertypes', else every `X` in a class below
     `Type` (an interface's implementations). If A's file imports `Type` by an import no
     file resolves, `Type` is a library's and the call links to **nothing**; if it imports
     `Type` from a file holding no class `Type` (a Java record), only that file's `X`. When
     `obj` has no declared type but itself names a class (`Calc::new(`, `Store.open(`),
     that class's `X` the same way. Otherwise the call goes on to rule 3.
  2. **Scope.** A bare `X(` or `self.X(` / `this.X(` links to the `X` nested directly in
     the innermost node enclosing A that has one — A's own class first — else in that
     class's supertypes. `super.X(` / `super().X(` goes to the supertypes only, and links
     to nothing when none is in the graph. A itself is never a target: a method `load`
     whose body calls `load()` goes on to the next rule (a delegation, not a recursion).
  3. **Same file**, then **imported files and A's own directory**, then the **same
     package** — files whose directory has the same last two segments
     (`src/test/java/a/b` and `src/main/java/a/b`). A call on a receiver (`x.X(`) never
     links this way to a member of A's own class. Import lines are read with one regex
     per shape (`import a.b.C;`, `import a.b as c, d`, `from a.b import c`,
     `import x from './y'`, `require('./y')`, Go `import "m/p"`, `use crate::a::b`,
     `mod x;`, `#include "x.h"`, `using A.B;`, Lua/Ruby `require`, shell `source` / `.`);
     a spec resolves to the files whose path ends with it, else the directories that do,
     else — for a module path — the directory a trailing run of its segments is; a
     one-segment spec (`import utils`, Go's `import "errors"`) names only a file or
     directory at the root or one directory down; a relative spec is joined to the
     importing file's directory (`./b.js` also finds `b.ts`). These file→file
     **imports** are not edges of the node graph (files are not nodes); they steer this
     rule and are counted by `rtdd graph`.
  4. **Whatever is left.** Past A's own file, a set of more than **5** definitions links
     to **none**, and `rtdd graph` counts the call in `ambiguous_calls`. A name that
     common carries no signal about which definition is meant.

  The same rule links the scanner's calls from stale files against the merged graph in the
  §5 overlay (step 3's re-pointing stays within one file, so one extension).
- **`inherits` / `implements` edges** from a class node to the class nodes its definition
  line names: `class A extends B implements C`, `interface A extends B`, `class A(B):`,
  `class A : public B` / `struct A : B` / `class A(x: T) : B()`, `class A < B`, and
  `impl Trait for A` (implements). A supertype name resolves to class nodes by rules 3-4.

### 4.4 Fixture corpus

`internal/scan/testdata/<lang>/` holds one small project per language in §1 with a
`want.json` of expected nodes (name, kind, start, end) and call edges. The scanner test
compares exactly. No toolchain is invoked.

### 4.5 Cache

`.rtdd/graph.json` holds the scanner's graph in graphify's `graph.json` shape plus
`built_at_commit` and, per file, the git blob id it was scanned from. On every command a file
is re-scanned only if its blob id changed or it is in the working-tree changed set.
`.rtdd/graph.json` is **gitignored** (`rtdd init` adds the line): it is a cache, rebuilt in
seconds, and committing it would conflict on every branch.

## 5. graphify, and the fact that it goes stale

If `graphify-out/graph.json` exists (path overridable as `graphify_path`), rtdd loads it.
**graphify does not update when code changes** — a graph built once is silently wrong after
the next edit (one repository measured on 2026-10-07 was **121 commits** behind). rtdd never
trusts graphify for a file that may have changed since it was built:

1. **Stale set** = files changed between graphify's `built_at_commit` and the working tree
   (`git diff --name-only <built_at_commit>` ∪ untracked) ∪ the current changed set ∪ code
   files absent from graphify's `manifest.json`.
2. For every file in the stale set, **drop** graphify's nodes and edges from that file and
   **replace** them with the scanner's (§4).
3. graphify edges from unchanged files that pointed into a dropped node are **re-pointed by
   name** to the scanner node of the same name in the same file; with no such node (renamed,
   deleted) the edge is dropped.
4. If `built_at_commit` is missing, unknown to git, or the stale set exceeds **50 %**
   (`max_stale_ratio`) of graphify's code files, graphify is **ignored entirely** and the
   scanner builds the whole graph. The output says so and suggests `graphify --update`.

rtdd never runs graphify itself. Every changed file is therefore always scanned by rtdd, as
is the file `explain <file>:<line>` names (it joins the stale set for that build), so
line→node mapping never uses graphify's start-only locations.

`--json` reports `graph.source` (`graphify+scanner` or `scanner`), `graph.built_at_commit`,
`graph.stale_files` and, when graphify was ignored, the reason.

## 6. Tests

A node is a test when its file matches `test_files` (default: `**/test_*`, `**/*_test.*`,
`**/*.test.*`, `**/*.spec.*`, `**/*Test.*`, `**/*Tests.*`, `**/tests/**`, `**/test/**`,
`**/spec/**`, `**/__tests__/**`) **and** it is a func/method/test node (not a class). Files
under `**/testdata/**` and `**/fixtures/**` are never test files. Both lists live in
`.rtdd/config.yaml` and are generic, not per language.

## 7. Rounds

Pure function `Rounds(graph, changedRanges) → {changed_nodes, round1, round2, untested}`:

- **changed_nodes** — the innermost node containing each changed line; a changed line in no
  node (top-level code) maps to a synthetic `file::<module>` node whose callers are the nodes
  of other files that call any node in that file.
- **round1** — changed test nodes, plus every test node with an edge **to** a changed node.
- **neighbours** — nodes one edge from a changed node, **both directions**, over
  `calls|method|inherits|implements|references`, excluding test nodes.
- **round2** — test nodes with an edge to a neighbour, minus round1.
- **untested** — changed non-test nodes with no test in round1 or round2.

Within a round, tests are ordered by file then line. Round 3 is always "the full suite".

## 8. Commands (v0.3.0)

| command | does |
|---|---|
| `rtdd which [--base <ref>] [--json]` | changed nodes, Round 1, Round 2, untested, graph source. Runs nothing. |
| `rtdd graph [--json]` | builds/refreshes the graph cache; prints source, node/edge/test counts, the `imports`, `inherits` and `implements` counts and `ambiguous_calls` (§4.3), staleness. |
| `rtdd explain <file[:line]\|name>` | a node's tests, callers and callees. |
| `rtdd doctor` | graph source, graphify staleness, test-file detection counts, names defined ≥ **8** times (over-link hubs). |
| `rtdd init`, `update`, `uninstall`, `version`, `skill` | as today, minus adapter detection. `init` no longer refuses: every repository is supported. |

**Removed:** `seed`, `run`, `verify`, `status`, `map compact`; `internal/{adapter,covfmt,runner,mapstore,selector,uncovered}`; `adapters/`; `.rtdd/map.jsonl`, `.rtdd/meta.json`, `.rtdd/adapters/`; the merge driver line in `.gitattributes`. `rtdd init` on a v0.2 repository deletes `map.jsonl`, `meta.json` and the gitattributes line, and says it did.

Exit codes: **0** success (including empty rounds), **2** usage, **3** environment (not a git
repository, unreadable graph). No exit code reports a test result — rtdd runs none.

## 9. `--json`, schema 3

```json
{
  "schema": 3,
  "command": "which",
  "base": "HEAD",
  "graph": {"source": "graphify+scanner", "built_at_commit": "fab6c1a", "stale_files": 4},
  "changed": [{"path": "src/auth.py", "lines": [{"start": 52, "end": 58}]}],
  "changed_nodes": [{"id": "src/auth.py::login", "file": "src/auth.py", "name": "login", "start": 48, "end": 70}],
  "rounds": [
    {"round": 1, "tests": [{"id": "tests/test_auth.py::test_login", "file": "tests/test_auth.py", "name": "test_login"}], "files": ["tests/test_auth.py"]},
    {"round": 2, "tests": [], "files": []},
    {"round": 3, "full_suite": true}
  ],
  "untested": ["src/auth.py::login_hint"],
  "warnings": []
}
```

Consumers reject any `schema` other than `3`. `files` is the de-duplicated test files of the
round, for runners that cannot select a single test.

## 10. The skill (the agent's process)

Generated from `protocol/PROTOCOL.md` as today. The process it states:

1. Edit code (test first, per TDD).
2. `rtdd which`. If `untested` names a node you changed, write its test first.
3. Run **Round 1** with the project's test command. Fix until green.
4. Run **Round 2**. Fix until green; return to step 2 after any further edit.
5. When the task is done — before committing or handing off — run the **full suite once**.

Empty Rounds 1 and 2 are stated as "no linked test", never as a pass. The skill names no
language and no framework.

## 11. Non-goals

- Executing tests, collecting outcomes, or reporting pass/fail.
- Coverage collection of any kind, and line-level "uncovered" reports.
- Depth beyond 1 (Round 3 is the safety net).
- Running or updating graphify.
- Per-language adapters, now or as a plug-in point.

## 12. Defaults the implementation must not re-decide

| question | default |
|---|---|
| depth-1 direction | both (callees and callers) |
| `.rtdd/graph.json` | gitignored cache |
| graphify too stale | ignore it past 50 % stale; never run it |
| ambiguous names | narrow by receiver type, scope, file, imports, package; past **5** candidates link none and count `ambiguous_calls` (issue #508, superseding "link all"); `doctor` lists names defined ≥ 8 times |
| version | v0.3.0; tagging and release remain a human action |
