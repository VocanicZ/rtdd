# `gradle` fixture

Proves the `gradle` adapter against a real repository layout.

- **Detected by:** `build.gradle.kts`.
- **Correspondence:** `src/main/java/calc/Calc.java` →
  `src/test/java/calc/CalcTest.java`, via `test_for`'s
  `src/test/java/{subdir}/{name}Test.java` — the package is the mirrored part, as in the
  `maven` fixture.

The Gradle wrapper is absent: `./gradlew` is only ever invoked by `subset` and `list`,
and no runner is executed here.
