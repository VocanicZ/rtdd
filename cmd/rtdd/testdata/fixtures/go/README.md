# `go` fixture

Proves the `go` adapter against a real repository layout.

- **Detected by:** `go.mod`.
- **Correspondence:** `calc/calc.go` → `calc/calc_test.go`, via `test_for`'s
  `{dir}/{name}_test.go`.

**The nested `go.mod` is deliberate and cannot affect this repository's build.** The Go
tool ignores every directory named `testdata`, so `go build ./...` and `go vet ./...`
still see exactly one module. Without that rule a second `go.mod` inside the tree would
be a mistake; with it, it is the only honest way to commit a fixture that detects the Go
adapter.

No runner is executed.
