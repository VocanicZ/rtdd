# `jest` fixture

Proves the `jest` adapter against a real repository layout.

- **Detected by:** `jest.config.js`. `package.json` is committed but is not a marker
  (decision 1).
- **Correspondence:** `src/calc.js` → `src/calc.test.js`, via `test_for`'s
  `{dir}/{name}.test.js`. There is no `src/calc.test.ts`, so the earlier TypeScript
  template falls through, which is the declaration order doing its job.

No runner is executed.
