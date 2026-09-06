# `vitest` fixture

Proves the `vitest` adapter against a real repository layout.

- **Detected by:** `vitest.config.ts`. `package.json` is committed because a Vitest repo
  has one, and deliberately is NOT a detect marker — decision 1 keeps it out of every
  adapter's `detect`, so a JavaScript repo never matches two adapters at once.
- **Correspondence:** `src/calc.ts` → `src/calc.test.ts`, via `test_for`'s
  `{dir}/{name}.test.ts`.

No runner is executed. The fixture proves detection and selection, which is what
`rtdd which` answers.
