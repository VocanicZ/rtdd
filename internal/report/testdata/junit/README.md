# Captured JUnit XML fixtures

Nine JUnit XML reports, each written by a real test runner executing a three-test suite —
one pass, one fail, one skip. They are the ground truth `internal/report`'s junit-xml
parser is tested against, in `junit_fixtures_test.go`, and the evidence that every shipped
adapter's `id_template` round-trips over its own runner's output, in `shipped_id_test.go`.

Six of them back the parser. The three added for the shipped adapter set —
`nextest.xml`, `dotnet.xml` and `jest-filepath.xml` — back the id templates: an
`id_template` that renders an id the runner will not accept back is only discoverable by
round-tripping it against that runner's real output.

None of them is hand-written, and none may be hand-edited. A hand-written fixture records
what its author believes JUnit XML looks like; the nine below record what nine ecosystems
actually emit, which is what keeps disagreeing — about the root element, about suite
nesting, about whether `file=` exists at all, about whether a skip carries a message. When
a fixture contradicts the parser, the parser is what changes.

Regenerate them with `scripts/capture-junit-fixtures.sh [runner ...]`, which builds each
project, runs it in the pinned image below, and copies the runner's own file out. Read the
diff before committing it: a re-capture moves timestamps, hostnames and durations, and the
durations are asserted by name in the test table.

## Provenance

Captured in the pinned image named, with the command the script runs. The first six were
captured 2026-09-05; the three the shipped adapter set added were captured 2026-09-06.

| fixture | runner | version | image | command |
|---|---|---|---|---|
| `vitest.xml` | Vitest | vitest 3.2.7, node v22.23.2 | `node:22-slim` | `npx vitest run --reporter=junit --outputFile=./junit.xml` |
| `jest.xml` | Jest + jest-junit | jest 29.7.0, jest-junit 16.0.0 | `node:22-slim` | `npx jest --reporters=jest-junit` |
| `go-junit-report.xml` | `go test -json` \| go-junit-report | go-junit-report v2.1.0, Go 1.24 | `golang:1.24` | `go test -json ./... \| go-junit-report -parser gojson` |
| `surefire.xml` | Maven Surefire | maven 3.9.16, maven-surefire-plugin 3.2.5, JUnit Jupiter 5.10.2, Temurin 21 | `maven:3.9-eclipse-temurin-21` | `mvn -B test`, then `target/surefire-reports/TEST-calc.CalcTest.xml` |
| `rspec.xml` | RSpec | rspec 3.13.0, rspec_junit_formatter 0.6.0 | `ruby:3.3-slim` | `rspec --format RspecJunitFormatter --out junit.xml` |
| `phpunit.xml` | PHPUnit | phpunit 10.5.20 | `php:8.3-cli` | `php phpunit.phar --log-junit junit.xml tests` |
| `jest-filepath.xml` | Jest + jest-junit, `classNameTemplate` reconfigured | jest 29.7.0, jest-junit 16.0.0, node v22.23.2 | `node:22-slim` | `JEST_JUNIT_CLASSNAME="{filepath}" npx jest --reporters=jest-junit` |
| `nextest.xml` | cargo-nextest | cargo-nextest 0.9.78, rustc 1.98.0 | `rust:1-slim` | `cargo nextest run --profile ci`, with `[profile.ci.junit] path = "junit.xml"` in `.config/nextest.toml`, then `target/nextest/ci/junit.xml` |
| `dotnet.xml` | `dotnet test` + JunitXml.TestLogger | .NET SDK 8.0.424, xunit 2.7.0, xunit.runner.visualstudio 2.5.7, JunitXml.TestLogger 3.1.12 | `mcr.microsoft.com/dotnet/sdk:8.0` | `dotnet test --logger junit --results-directory /work/TestResults`, then `TestResults/TestResults.xml` |

## What the nine disagree about

The checklist a tenth ecosystem gets measured against. It is repeated, with the
consequences for the reader, in the comment above the test table in
`internal/report/junit_fixtures_test.go`.

| runner | root | nesting | `file=` | skip spelling |
|---|---|---|---|---|
| Vitest | `<testsuites>` | one `<testsuite>` per test file | absent | `<skipped/>`, empty |
| Jest | `<testsuites>` | one `<testsuite>` per `describe` block | absent | `<skipped/>`, empty |
| go-junit-report | `<testsuites>` | one `<testsuite>` per package | absent | `<skipped message="Skipped">` |
| Maven Surefire | `<testsuite>` | bare root, no wrapper, plus a `<properties>` block | absent | `<skipped message="not implemented yet"/>` |
| RSpec | `<testsuite>` | bare root, no wrapper | present, `./spec/calc_spec.rb` | `<skipped/>`, empty |
| PHPUnit | `<testsuites>` | `<testsuite>` nested inside `<testsuite>` | present, absolute path | `<skipped/>`, empty |
| cargo-nextest | `<testsuites>` | one `<testsuite>` per test binary | absent | **not written at all** — an `#[ignore]`d test is counted in the run summary and omitted from the report |
| JunitXml.TestLogger | `<testsuites>` | one `<testsuite>` per test assembly | absent | `<skipped/>`, empty |
| Jest, `classNameTemplate` reconfigured | `<testsuites>` | one `<testsuite>` per `describe` block | absent | `<skipped/>`, empty |

Six more, which the columns above cannot show:

- `classname=` is the file path (Vitest), the package (Go), the class (Surefire, PHPUnit),
  the spec file dotted (RSpec) — and in jest-junit a copy of the test's own full name, so
  `classname` and `name` are identical strings.
- jest-junit writes `<failure>` with **no** `message=` attribute; PHPUnit writes one with
  `type=` but no `message=`. A reader keying "did this fail" off `message=` reads both as
  passing.
- The suite name is the file, the describe block, the package, the class, the test binary,
  the test assembly, or the literal string `rspec`. Nothing may depend on it being any one
  of those.
- **nextest's `classname=` is the BINARY id** (`calc`), not a class and not a path, and its
  `name=` is the test's full Rust path (`tests::adds_two_numbers`). That is why
  `adapters/cargo-nextest.yaml` renders `{name}` alone: `{classname}` would give every test
  in a crate the same id.
- **`JunitXml.TestLogger` writes a `<testsuites>` root and orders cases by OUTCOME** —
  failures, then passes, then skips — so the first `<testcase>` in `dotnet.xml` is the
  failing one. Nothing may depend on a report's cases being in declaration order.
- **`jest-filepath.xml` differs from `jest.xml` only in `classname=`**, and that is the
  whole point of it. jest-junit's default `classNameTemplate` copies the test's own full
  name, so `jest.xml`'s `classname` and `name` are the same string and no template over it
  names the FILE; `JEST_JUNIT_CLASSNAME='{filepath}'` makes `classname` the test file's
  path, which is what `adapters/jest.yaml`'s `id_template: "{classname}"` renders.
  `jest.xml` is left as it is — it is the default-config capture the parser is tested
  against, and re-capturing it under the reconfigured template would delete the evidence
  for the default.

The paths inside these files (`/work/tests/CalcTest.php`, `./spec/calc_spec.rb`) are the
capture container's, not this repo's. That is the point: they are what the runner wrote.
