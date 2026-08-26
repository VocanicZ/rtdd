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

`bench/swebench/run_arm.py` is the per-arm driver. It calls `preflight` before it spends a
token, walks the frozen `instances.txt` in file order, writes one JSON record per instance
under `bench/results/swebench/raw/<arm>/`, and is resumable: an instance that already has a
record is skipped, so a crash costs the current instance only. `budget.py` holds the
ceilings — estimated dollars and wall-clock hours — and the run writes
`bench/results/swebench/cost/<arm>/cost.json` however it ends, with the assumed per-token
prices beside the actual token counts. A resume reads that file back and restores the
spend before it charges anything new, so `--max-usd` is a ceiling on the benchmark rather
than on each restart, and the rewritten cost file is the whole arm's spend rather than the
last invocation's.

Every instance's prompt is checked before it reaches the model: all five arms are
re-assembled from that instance's real workspace root and each context arm must still be
its control plus exactly one `<test-context>` block. The workspace path carries no arm name
for that reason — it is interpolated into the prompt's `Repository root:` line, so a
per-arm path would ship the arm's own identity inside every system prompt. A violation
stops the arm with exit 5 rather than being recorded as a failed instance.

```bash
cd bench/swebench
uv run python run_arm.py --dry-run     # all five arms, whole list, no model, no network
uv run python run_arm.py rtdd --max-usd 75
```

`--dry-run` is what CI runs. It assembles every arm's prompt for every instance through the
same code path a real run uses, against the same workspace root a real run would use, and
asserts the composition guarantee on those bytes — each
context arm is its control plus exactly one `<test-context>` block, and the block carries no
imperative — exiting non-zero if any arm violates it. It needs no signed pre-registration
because it spends nothing.

[docs/bench/swebench-harness.md](docs/bench/swebench-harness.md) is the reviewer's page for
that harness: how to reproduce it locally with no model, network or Docker, what should
refuse and with which message, the deliberately failing fixtures that prove the
non-procedural guarantee is load-bearing, and which plan tasks are out of scope because they
are human-in-the-loop. `bench/swebench/tests/test_acceptance.py` is the executable form of
the same page, and CI runs it.
