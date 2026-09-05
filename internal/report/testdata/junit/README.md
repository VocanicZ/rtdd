# Captured JUnit XML fixtures

Six JUnit XML reports, each written by a real test runner executing a three-test suite —
one pass, one fail, one skip. They are the ground truth `internal/report`'s junit-xml
parser is tested against, in `junit_fixtures_test.go`.

None of them is hand-written, and none may be hand-edited. A hand-written fixture records
what its author believes JUnit XML looks like; the six below record what six ecosystems
actually emit, which is what keeps disagreeing — about the root element, about suite
nesting, about whether `file=` exists at all, about whether a skip carries a message. When
a fixture contradicts the parser, the parser is what changes.

Regenerate them with `scripts/capture-junit-fixtures.sh [runner ...]`, which builds each
project, runs it in the pinned image below, and copies the runner's own file out. Read the
diff before committing it: a re-capture moves timestamps, hostnames and durations, and the
durations are asserted by name in the test table.

## Provenance

Captured 2026-09-05, each in the pinned image named, with the command the script runs.

| fixture | runner | version | image | command |
|---|---|---|---|---|
| `vitest.xml` | Vitest | vitest 3.2.7, node v22.23.2 | `node:22-slim` | `npx vitest run --reporter=junit --outputFile=./junit.xml` |
| `jest.xml` | Jest + jest-junit | jest 29.7.0, jest-junit 16.0.0 | `node:22-slim` | `npx jest --reporters=jest-junit` |
| `go-junit-report.xml` | `go test -json` \| go-junit-report | go-junit-report v2.1.0, Go 1.24 | `golang:1.24` | `go test -json ./... \| go-junit-report -parser gojson` |
| `surefire.xml` | Maven Surefire | maven 3.9.16, maven-surefire-plugin 3.2.5, JUnit Jupiter 5.10.2, Temurin 21 | `maven:3.9-eclipse-temurin-21` | `mvn -B test`, then `target/surefire-reports/TEST-calc.CalcTest.xml` |
| `rspec.xml` | RSpec | rspec 3.13.0, rspec_junit_formatter 0.6.0 | `ruby:3.3-slim` | `rspec --format RspecJunitFormatter --out junit.xml` |
| `phpunit.xml` | PHPUnit | phpunit 10.5.20 | `php:8.3-cli` | `php phpunit.phar --log-junit junit.xml tests` |

## What the six disagree about

The checklist a seventh ecosystem gets measured against. It is repeated, with the
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

Three more, which the columns above cannot show:

- `classname=` is the file path (Vitest), the package (Go), the class (Surefire, PHPUnit),
  the spec file dotted (RSpec) — and in jest-junit a copy of the test's own full name, so
  `classname` and `name` are identical strings.
- jest-junit writes `<failure>` with **no** `message=` attribute; PHPUnit writes one with
  `type=` but no `message=`. A reader keying "did this fail" off `message=` reads both as
  passing.
- The suite name is the file, the describe block, the package, the class, or the literal
  string `rspec`. Nothing may depend on it being any one of those.

The paths inside these files (`/work/tests/CalcTest.php`, `./spec/calc_spec.rb`) are the
capture container's, not this repo's. That is the point: they are what the runner wrote.
