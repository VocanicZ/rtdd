# Development

## Toolchain

Go 1.24+ is required. On this machine the toolchain is installed at `/home/claude/goroot`
but is **not** on the default `PATH`. It is symlinked into `~/bin`:

```bash
ln -sf /home/claude/goroot/bin/go   ~/bin/go
ln -sf /home/claude/goroot/bin/gofmt ~/bin/gofmt
go version   # go1.24.4 linux/amd64
```

If `go: command not found`, re-run the symlinks above before doing anything else.
Do not hand-review Go code in place of compiling it.

## Verify the build

`scripts/ci-local.sh` is the authoritative CI gate. Run it before merging anything:

```bash
scripts/ci-local.sh
```

It runs exactly what `.github/workflows/ci.yml` runs:

```bash
go build ./... && go vet ./... && gofmt -l . && go test ./... -count=1
CGO_ENABLED=0 go build -o /tmp/rtdd ./cmd/rtdd   # must produce a STATIC binary
file /tmp/rtdd | grep -q 'statically linked' || echo "FAIL: not static"
```

`CGO_ENABLED=0` is not optional. Spec D4 promises a binary with no runtime dependencies
forced into the host repo; `modernc.org/sqlite` is chosen over `mattn/go-sqlite3`
specifically because it is pure Go. A cgo dependency breaks that promise.

## Dependencies

Only two third-party modules are permitted in the engine:

- `modernc.org/sqlite` — reads `.coverage` directly (spec §4)
- `gopkg.in/yaml.v3` — adapter definitions

Adding a third requires a spec amendment. No test framework beyond stdlib `testing`.

## Benchmark harnesses

`bench/` is Python, managed with `uv`, and is deliberately outside the Go module.
`scripts/ci-local.sh` runs `scripts/ci-prereg.sh` as part of the gate, so `uv` must be on
PATH alongside `go`:

```bash
cd bench/swebench && uv sync && uv run pytest -q
```

`bench/PREREGISTRATION.md` gates every M4 arm. It ships `status: UNSIGNED` with
`stratified_recall_floor:` deliberately empty — that number is a human decision, and
`bench/swebench/preflight.py` refuses to launch (exit 3) until a human writes it, signs the
file, and the signing commit is reachable from the `prereg-m4` tag. The bench gate asserts
that refusal while the file is unsigned, so it is green on an unsigned repo.
