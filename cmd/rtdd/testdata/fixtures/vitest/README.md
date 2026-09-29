# `vitest` fixture

Proves the `vitest` adapter against a real repository layout.

- **Detected by:** `vitest.config.ts`. `package.json` is committed because a Vitest repo
  has one, and deliberately is NOT a detect marker — decision 1 keeps it out of every
  adapter's `detect`, so a JavaScript repo never matches two adapters at once.
- **Selection:** each test file is a unit run in its own process; the `TestPipeline*` test in `cmd/rtdd/` seeds this fixture and checks a changed source file selects the test file that executes it (skipped when the toolchain is absent).

Detection is proved by `TestEachFixtureRepoDetectsItsAdapter`; the pipeline test runs the real
toolchain.
