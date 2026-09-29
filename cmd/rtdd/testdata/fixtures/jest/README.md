# `jest` fixture

Proves the `jest` adapter against a real repository layout.

- **Detected by:** `jest.config.js`. `package.json` is committed but is not a marker
  (decision 1).
- **Selection:** each test file is a unit run in its own process; the `TestPipeline*` test in `cmd/rtdd/` seeds this fixture and checks a changed source file selects the test file that executes it (skipped when the toolchain is absent).

The fixture is only ever run by the pipeline test, in a temporary copy.
