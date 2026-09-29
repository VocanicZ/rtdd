# Limitations

- **Every language uses the same pipeline, and it costs one process per test file.** Each
  test file runs alone under the language's stock coverage tool, and the map is built from
  what those runs executed. `rtdd seed` pays that for the whole suite, and `rtdd run` pays it
  for every selected file. It is cheap for Python, Go and Node and slow where a process start
  is slow: the JVM under Maven or Gradle, and .NET. Go cannot take a file, so its unit is the
  package filtered to that file's own `Test` functions.
- **Selection is file-level on both sides.** A change to one function selects every test file
  that executed any part of its file, and the smallest unit selected is a test file.
- **Test files that depend on each other can behave differently.** Shared state on disk,
  ordering, a fixed port: run one per process, they may pass or fail differently than in the
  full suite, and the map records only what each did alone.
- **Some adapters are unverified.** The gradle, phpunit and rspec adapters were written
  without a real toolchain to run them on; expect to adjust them on first use. Multi-module
  Maven reactors and multi-project Gradle or .NET solutions need a host adapter.
- **It does not enforce anything.** No gate, no policy exit code, no expected-phase flag.
- **It does not replace CI.** Run the full suite there.
- **It is not sound program analysis.** Selection can be wrong; the tier and the uncovered
  report are how you see when.
- **It does not reduce token cost.** No model calls in the hot path.
- **Coverage misses what no test ran.** A path no test file has executed is not in the map, so
  new code selects nothing until it has run once, and the uncovered report says so.

Back to the [README](../README.md).
