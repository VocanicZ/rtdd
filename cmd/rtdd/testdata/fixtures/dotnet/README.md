# `dotnet` fixture

Proves the `dotnet` adapter against a real repository layout.

- **Detected by:** `Calc.sln` (and `Calc.csproj`; either alone is enough).
- **Correspondence:** `Calc/Calc.cs` → `tests/Calc.Tests/CalcTests.cs`, via `test_for`'s
  `tests/{name}.Tests/{name}Tests.cs`.

No runner is executed. `--logger junit` resolves only because a host project references
the JUnitTestLogger package; nothing here restores or builds.
