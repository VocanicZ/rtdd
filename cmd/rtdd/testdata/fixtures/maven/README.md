# `maven` fixture

Proves the `maven` adapter against a real repository layout.

- **Detected by:** `pom.xml`.
- **Correspondence:** `src/main/java/calc/Calc.java` →
  `src/test/java/calc/CalcTest.java`, via `test_for`'s
  `src/test/java/{subdir}/{name}Test.java`.

`{subdir}` rather than `{dir}` is the whole point of this fixture: the changed file's
directory is `src/main/java/calc`, and only its trailing `calc` — the package — is
mirrored into the test tree. `{subdir}` is `{dir}` or any trailing part of it, longest
first, and a candidate counts only when the file exists.

No runner is executed.
