# `maven` fixture

Proves the `maven` adapter against a real repository layout.

- **Detected by:** `pom.xml`.
- **Selection:** each test file is a unit run in its own process; the `TestPipeline*` test in `cmd/rtdd/` seeds this fixture and checks a changed source file selects the test file that executes it (skipped when the toolchain is absent).

`{subdir}` rather than `{dir}` is the whole point of this fixture: the changed file's
directory is `src/main/java/calc`, and only its trailing `calc` — the package — is
mirrored into the test tree. `{subdir}` is `{dir}` or any trailing part of it, longest
first, and a candidate counts only when the file exists.

The fixture is only ever run by the pipeline test, in a temporary copy.
