# `gradle` fixture

Proves the `gradle` adapter against a real repository layout.

- **Detected by:** `build.gradle.kts`.
- **Selection:** each test file is a unit run in its own process; the `TestPipeline*` test in `cmd/rtdd/` seeds this fixture and checks a changed source file selects the test file that executes it (skipped when the toolchain is absent).

The Gradle wrapper is absent from the repository; the pipeline test generates it with
`gradle wrapper` in a temporary copy.
