# RTDD agent protocol

This file is the single source for every generated agent front-end. Edit it, then run
`rtdd-gen render`. Do not edit anything under `dist/`.

<!-- rtdd:meta version=1 -->

<!-- rtdd:section id=setup title="Setting up a repository" targets=global,global-agents order=5 -->
Check for `.rtdd/config.yaml` first. It says whether rtdd is set up in this repository.

**It exists.** rtdd is set up. Follow the process below, and do not re-run `rtdd init`.

**It does not.** Run `rtdd init` once. It sets up every git repository, whatever the code is
written in: it writes these instructions for the agents the repository uses, a
`.rtdd/config.yaml` of defaults, and a `.gitignore` line for `.rtdd/graph.json`, rtdd's
rebuildable graph cache. In a repository an older rtdd set up, it also deletes the files
that release kept and prints one line for each file it removed. Commit what it wrote. There
is no other setup step: the graph is built the first time a command needs it.
<!-- rtdd:variant target=global-agents -->
If `.rtdd/config.yaml` is missing, run `rtdd init` once — it sets up any git repository —
and commit what it writes. Then follow the steps below.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=what title="What rtdd does" targets=skill,agents,mdc,global,global-agents order=10 -->
rtdd answers one question fast: which tests does this change need, and in what order. It
builds a graph of the repository's functions, methods and classes, links every test to the
code it calls, and reads the lines you changed against it:

- **Round 1** — the tests linked to the code you changed, and every test you changed.
- **Round 2** — the tests linked to that code's direct neighbours, its callers and its
  callees, minus Round 1.
- **Round 3** — the full suite, once, at the end of the task.

rtdd runs no tests. You run each round with the project's own test command, the one you
would use without rtdd. Nothing rtdd prints is a pass or a fail.
<!-- rtdd:variant target=agents -->
rtdd names the tests a change needs, in rounds: Round 1 tests the code you changed, Round 2
its direct neighbours, Round 3 is the full suite once at the end. rtdd runs no tests: you
run each round with the project's own test command.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=process title="The process" targets=skill,agents,mdc,global,global-agents order=20 -->
1. Edit code (test first, per TDD).
2. Run `rtdd which`. If `untested` names a node you changed, write its test first.
3. Run **Round 1** with the project's own test command. Fix until green.
4. Run **Round 2**. Fix until green; return to step 2 after any further edit.
5. When the task is done — before committing or handing off — run the **full suite once**.
<!-- rtdd:endsection -->

<!-- rtdd:section id=which title="rtdd which" targets=skill,agents,mdc,global,global-agents order=30 -->
```
rtdd which [--base <ref>] [--json]
```

Prints the changed nodes — the innermost function, method or class around each changed
line — then Round 1, Round 2, Round 3, the `untested` list and where the graph came from. It
runs nothing and answers in about a second, so ask it again after every edit.

The changed set is everything that differs from `--base` (default `HEAD`): committed since
it, staged, unstaged and untracked, so a file you just wrote counts before you commit it. A
changed line outside every function, such as an import, belongs to its file's `<module>`
node.

`untested` lists the changed nodes no test in Round 1 or Round 2 reaches. Write a test for
each one you changed before you run the rounds.
<!-- rtdd:variant target=agents -->
`rtdd which` prints the changed nodes, Rounds 1–3 and `untested`, and runs nothing.
Untracked files count. `rtdd which --json` is the machine-readable form.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=empty title="Empty rounds are not a pass" targets=skill,agents,mdc,global,global-agents order=40 -->
When Rounds 1 and 2 are empty, `rtdd which` prints `no linked test`. Read it as exactly
that — no linked test, never a pass: nothing in the graph links a test to the code you
changed. Write the test the change needs. Round 3 still runs the full suite once at the end.
<!-- rtdd:variant target=agents -->
Empty Rounds 1 and 2 print `no linked test`: no linked test, never a pass.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=json title="JSON output" targets=skill,global order=60 -->
`rtdd which --json` emits one object, schema 3:

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

Reject any `schema` other than `3`. `rounds` always holds three entries, and Round 3 is
always the full suite. `files` is a round's test files without duplicates, for a test
command that cannot select a single test. `graph.source` is `graphify+scanner` or `scanner`.
<!-- rtdd:endsection -->

<!-- rtdd:section id=commands title="The rest of the commands" targets=skill,global order=70 -->
```
rtdd graph [--json]                the graph's source, node, edge and test counts, staleness
rtdd explain <file[:line]|name>    a node's tests, callers and callees
rtdd doctor                        graph source, graphify staleness, test files found, names defined 8+ times
rtdd init [--dry-run]              set up this repository; any git repository
rtdd uninstall [--state]           remove what init wrote
rtdd update                        replace the binary with the latest release
rtdd --version                     print the version
```

Exit codes: 0 success, empty rounds included; 2 usage; 3 environment (not a git repository,
a graph that cannot be built). No exit code is a test result, because rtdd runs none.
<!-- rtdd:endsection -->

<!-- rtdd:section id=limits title="What it cannot see" targets=skill,mdc,global order=80 -->
Stated plainly, because a tool that hides its blind spots is worse than none:

- Links are by name. A call to `load(` links to every definition named `load` in files of
  the same kind, so a common name over-links and Round 2 can hold tests the change does not
  need. `rtdd doctor` lists the names defined eight or more times.
- A call the text does not show — through reflection, a string, a registry or a framework
  hook — has no edge, and its tests are in neither round.
- Depth is one. A caller's caller is in no round; Round 3 is the safety net.
- A deleted file has no lines left to own a node, so the tests that called it are in no
  round. `rtdd which` warns, and Round 3 runs them.
- The scanner reads text, not syntax: an unusual layout can give a node the wrong span.
<!-- rtdd:variant target=mdc -->
Links are by name, so a common name over-links; a call through reflection, a string or a
framework hook has no edge; depth is one, and Round 3 is the safety net.
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->
