# Limitations

rtdd v0.3.0 names the tests a change needs from a graph of the repository's functions,
methods and classes. Every limit below is a way that graph can be missing an edge or holding
one too many. Round 3 — the full suite, once, at the end of the task — is the safety net for
all of them.

- **Links are by name, narrowed without a parser.** A call to `load(` links first to the
  `load` of its own class (or a supertype), or of its receiver's declared type
  (`Store s; s.load(`), then to the ones in its own file, in the files it imports, and in
  its package. The narrowing reads text, so it can pick wrong: an interface call whose
  implementation is elsewhere, a local whose type the line does not state. A name that
  still matches more than five definitions links to **none** — `rtdd graph` counts those
  as `ambiguous_calls` — so its tests are in neither round. `rtdd doctor` lists the names
  defined eight or more times, which are where this happens.
- **A call the text does not show has no edge.** A call through reflection, a string, a
  registry, dependency injection or a framework hook links nothing, so its tests are in
  neither Round 1 nor Round 2.
- **Depth is one.** Round 2 is the tests of the changed code's direct callers and callees. A
  caller's caller is in no round.
- **A deleted file owns no node.** It has no lines left, so the tests that called it are in
  no round. `rtdd which` warns when a changed file was deleted, and Round 3 runs them.
- **The scanner reads text, not syntax.** It needs no toolchain and no parser, and pays for
  that: an unusual layout — a definition split across lines, a nested function the indent
  does not show — can give a node the wrong span, and a changed line can then land in the
  wrong node.
- **graphify's graph goes stale.** rtdd never runs graphify. It rescans every file that
  changed since graphify built its graph, so changed code is always read fresh, but the
  unchanged rest is only as current as the last time you ran graphify — an edge graphify
  saw that no longer exists still links. When more than half of graphify's graph is stale
  (`max_stale_ratio` in `.rtdd/config.yaml`), rtdd ignores it and says so.
- **It runs nothing and enforces nothing.** No gate, no policy exit code, no pass or fail:
  you run each round with the project's own test command.
- **It does not replace CI.** Run the full suite there.
- **It is not sound program analysis.** A round can be wrong in both directions; the
  `untested` list and Round 3 are how a wrong one is caught.

Back to the [README](../README.md).
