# `go` fixture

Proves the `go` adapter against a real repository layout.

- **Detected by:** `go.mod`.
- **Selection:** each test file is a unit run in its own process; the `TestPipeline*` test in `cmd/rtdd/` seeds this fixture and checks a changed source file selects the test file that executes it (skipped when the toolchain is absent).

**The nested `go.mod` is deliberate and cannot affect this repository's build.** The Go
tool ignores every directory named `testdata`, so `go build ./...` and `go vet ./...`
still see exactly one module. Without that rule a second `go.mod` inside the tree would
be a mistake; with it, it is the only honest way to commit a fixture that detects the Go
adapter.

The fixture is only ever run by the pipeline test, in a temporary copy.
