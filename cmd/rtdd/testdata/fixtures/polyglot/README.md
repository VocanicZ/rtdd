# `polyglot` fixture

Proves that a repository holding two toolchains is ordinary rather than an error
(spec §4.4, PRD #232 AC10).

- **Detects EXACTLY two adapters:** `vitest` (from `vitest.config.ts`) and `maven` (from
  `pom.xml`). Never three: `package.json` is committed — AC10 asks for it — and is a
  marker for nothing, so `jest` does not join in.
- The `vitest.config.ts` is here **as well as** the `package.json` because decision 1
  makes `package.json` alone detect nothing. That is the fixture being honest about the
  rule rather than working around it.
- **Correspondence, per adapter and never merged:**
  - `src/calc.ts` → `src/calc.test.ts` (vitest)
  - `src/main/java/calc/Calc.java` → `src/test/java/calc/CalcTest.java` (maven)

Each adapter answers about its own source files. A helper that unioned the two answers
would erase exactly the split this fixture exists to demonstrate — and handing one
adapter's ids to the other's runner is what PRD #232 AC6 forbids.
