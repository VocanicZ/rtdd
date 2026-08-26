# RTDD — shared interface contract

**Every plan and every task binds to this file.** If a task needs a signature that is not
here, it must add it here in the same commit. Do not invent a parallel name.

Source of truth for behaviour: [`docs/specs/2026-08-26-rtdd-design.md`](../specs/2026-08-26-rtdd-design.md).

## Global constraints

- **Go 1.24+**, module `github.com/VocanicZ/rtdd`.
- **Zero non-stdlib dependencies in the engine** except: `modernc.org/sqlite` (pure-Go, no
  cgo — required so the binary stays static), and `gopkg.in/yaml.v3`. No test framework
  beyond stdlib `testing`.
- All paths inside the engine are **repo-relative, slash-separated, cleaned**. Conversion
  happens at the boundary (`internal/paths`), never ad hoc.
- Every exported function returns `error` rather than panicking. `cmd/` is the only place
  that prints to stdout/stderr.
- Table-driven tests, stdlib `testing`, golden files under `testdata/`.

## Process exit codes for `rtdd`

| Code | Meaning |
|---|---|
| 0 | Success. **Includes** an empty selection and a non-empty uncovered report — these are signals, not failures |
| 1 | A test failed during `run`/`verify` |
| 2 | Usage or configuration error (bad flag, unparseable adapter, no adapter detected) |
| 3 | Fatal environment error (`no-sysmon-context` warning observed, `.coverage` unreadable, git unavailable) |

RTDD never exits nonzero to express a policy opinion. See spec §2 non-goals.

## Package layout

```
cmd/rtdd/            CLI only — flag parsing, output formatting, exit codes
internal/paths/      path normalisation
internal/mapstore/   map.jsonl I/O, union resolution, compaction, queries
internal/gitctx/     changed set, commit distance, merge detection
internal/adapter/    YAML load, detection, file classification
internal/coverage/   .coverage SQLite reader, context normalisation
internal/report/     pytest-reportlog parser
internal/pytestfixture/  test-only: materialises a real pytest project on disk
internal/selector/   tiers + ranking
internal/runner/     subprocess execution, argv chunking
internal/uncovered/  line classification
internal/doctor/     fan-out analysis
```

---

## internal/paths

```go
// Normalize converts an absolute or runner-relative path to a cleaned,
// slash-separated path relative to repoRoot. Returns ok=false if p escapes repoRoot.
func Normalize(repoRoot, p string) (rel string, ok bool)

// StripModulePrefix removes a Go module path prefix. Unused in M1; present so the
// Go adapter (deferred) has a defined home.
func StripModulePrefix(modulePath, p string) string
```

## internal/mapstore

```go
type Row struct {
    T string   `json:"t"` // test id, exactly as the runner accepts it as a selector
    F []string `json:"f"` // repo-relative source files, sorted, deduped
    C string   `json:"c"` // short SHA of HEAD when recorded
    D int      `json:"d"` // last duration, ms
    S string   `json:"s"` // last outcome: "pass" | "fail" | "skip" | "error"
}

type Map struct { /* unexported: rows map[string]Row */ }

func New() *Map
func Load(path string) (*Map, error)   // missing file returns an empty Map and nil error
func (m *Map) Save(path string) error  // sorted by T ascending; MUST end with a trailing "\n"
func (m *Map) Get(t string) (Row, bool)
func (m *Map) Len() int
func (m *Map) Rows() []Row             // sorted by T

// Union merges r into the map: F becomes the set-union, D and S take r's values,
// and C takes the OLDER of the two commits (a row's F is only as trustworthy as its
// stalest component). Used by every path except seed.
func (m *Map) Union(r Row, older func(a, b string) string)

// Replace overwrites the row wholesale. ONLY rtdd seed may call this.
func (m *Map) Replace(r Row)

func (m *Map) Delete(t string)

// TestsCovering returns every test id whose F intersects any of files. Unsorted.
func (m *Map) TestsCovering(files []string) []string

// FanOut returns file -> number of tests whose F contains it.
func (m *Map) FanOut() map[string]int
```

**Union-merge duplicate handling.** `Load` MUST tolerate duplicate `t` values (the union
merge driver leaves both lines) and resolve them exactly as `Union` does. A malformed line
is a **fatal error, never a silent skip** — silently dropping rows narrows selection.

## internal/gitctx

```go
type Status int
const (
    Added Status = iota
    Modified
    Deleted
    Renamed
    Untracked
)

type LineRange struct{ Start, End int } // 1-indexed, inclusive, in the NEW file

type Change struct {
    Path     string
    OldPath  string      // set only when Status == Renamed
    Status   Status
    Lines    []LineRange // empty for Deleted
}

// ChangedSet returns the union of `git diff --name-only --unified=0 <base>` and
// `git status --porcelain -uall`. Untracked files are included (Status=Added with the
// whole file as one LineRange). Deletions are retained.
func ChangedSet(repoRoot, base string) ([]Change, error)

func HeadSHA(repoRoot string) (string, error)

// CommitDistance returns the number of commits from sha to HEAD.
// Returns (-1, nil) when sha is unreachable — after a rebase, squash, or shallow clone.
// Callers MUST treat -1 as "unknown", never as "fresh".
func CommitDistance(repoRoot, sha string) (int, error)

func IsMergeCommit(repoRoot, sha string) (bool, error)

// Older returns whichever of a or b is the earlier ancestor. If either is unreachable,
// it returns that one (unknown age is treated as older, i.e. less trustworthy).
func Older(repoRoot string) func(a, b string) string
```

## internal/adapter

```go
type Adapter struct {
    Name          string            `yaml:"name"`
    Detect        []string          `yaml:"detect"`
    Env           map[string]string `yaml:"env"`
    Seed          string            `yaml:"seed"`
    Subset        string            `yaml:"subset"`
    List          string            `yaml:"list"`
    Coverage      string            `yaml:"coverage"` // "sqlite"
    Report        string            `yaml:"report"`   // "pytest-reportlog"
    FailFastFlag  string            `yaml:"failfast_flag"`
    TestGlobs     []string          `yaml:"test_globs"`
    SourceGlobs   []string          `yaml:"source_globs"`
    ExitCodes     map[int]string    `yaml:"exit_codes"`
    Opaque        []string          `yaml:"opaque"`
    FullEscalate  []string          `yaml:"full_escalate"`
}

func Load(path string) (*Adapter, error)
func LoadAll(dir string) ([]*Adapter, error)

// LoadFS reads every *.yaml under dir in fsys, sorted by adapter name.
// LoadAll is LoadFS(os.DirFS(dir), ".").
func LoadFS(fsys fs.FS, dir string) ([]*Adapter, error)

// Builtin returns the adapters embedded in the binary from the root `adapters`
// package (//go:embed *.yaml), so rtdd ships as a single static file.
func Builtin() ([]*Adapter, error)

// Detect returns the adapter whose Detect globs match a file in repoRoot.
// Exactly one match required; zero or multiple is an error (polyglot is out of scope in v1).
func Detect(repoRoot string, adapters []*Adapter) (*Adapter, error)

func (a *Adapter) IsTestFile(rel string) bool
func (a *Adapter) IsOpaque(rel string) bool
func (a *Adapter) IsFullEscalate(rel string) bool
// IsInstrumentable: matches SourceGlobs AND is not a test file AND is not Opaque.
func (a *Adapter) IsInstrumentable(rel string) bool

// Expand substitutes {tests} {src} {out} {log} into a command template and returns argv.
func (a *Adapter) Expand(tmpl string, vars map[string]string) ([]string, error)
```

## internal/coverage

```go
type TestCoverage struct {
    Test  string           // normalised id, phase suffix stripped
    Files map[string][]int // repo-relative path -> sorted covered line numbers
}

type Result struct {
    PerTest    []TestCoverage
    ImportTime map[string][]int // empty-context lines: executed, attributed to no test
}

// Merge unions other into r, sorting PerTest by Test. Used to combine the
// .coverage read after each argv chunk: pytest erases .coverage at the start of
// every run unless --cov-append is passed, so RTDD reads and merges per chunk.
// Per-test file line sets and ImportTime are unioned sorted and deduped; a test
// present only in other is appended; merging into a zero Result yields other's
// content; Merge(nil) is a no-op; nothing in r aliases other. CONTRACT ADDITION.
func (r *Result) Merge(other *Result)

// ReadSQLite reads .coverage directly:
//   SELECT DISTINCT f.path, c.context, lb.numbits FROM line_bits lb
//     JOIN file f ON f.id = lb.file_id JOIN context c ON c.id = lb.context_id
// numbits is coverage.py's packed line bitmap; decode with Numbits.
func ReadSQLite(dbPath, repoRoot string) (*Result, error)

// Numbits decodes coverage.py's numbits blob into sorted line numbers.
// Byte i, bit j set => line i*8+j is covered.
func Numbits(b []byte) []int

// NormalizeContext splits "tests/test_a.py::test_x|run" into ("tests/test_a.py::test_x", "run", true).
// An empty context returns ok=false — that is import-time coverage, not a test.
func NormalizeContext(ctx string) (testID, phase string, ok bool)
```

## internal/report

```go
type Outcome struct {
    Test       string
    Status     string // "pass" | "fail" | "skip" | "error"
    DurationMS int
}

// ReadReportLog parses pytest --report-log JSONL into one Outcome per test, in
// first-seen order. Measured phase rules (pytest 9.0.3):
//   setup=failed, no call entry            -> "error"
//   setup=skipped, no call entry           -> "skip"
//   call=passed|failed|skipped             -> "pass"|"fail"|"skip"
//   call=passed but teardown=failed        -> "error"
// DurationMS is the call phase's duration in ms, or setup+teardown when there is
// no call entry. `duration` in the JSONL is a float in seconds.
// Non-TestReport envelopes (SessionStart, CollectReport, SessionFinish) are
// ignored. A malformed line is a fatal error, never a silent skip.
func ReadReportLog(path string) ([]Outcome, error)
```

## internal/pytestfixture

Test-only. Materialises a tiny real pytest project so coverage/report/runner can be
tested against the actual toolchain. Never imported by `cmd/`.

```go
func Materialize(dir string) error
func InitGit(dir string) error
func HavePytest() bool
```

## internal/selector

```go
type Tier int
const (
    TierEmpty Tier = iota // nothing selected — reported explicitly, never silently green
    TierDirect
    TierT0
    TierT1
    TierT2
)
func (t Tier) String() string

type Config struct {
    StaleCommits int     // default 50
    DriftGuard   int     // default 100
    HubThreshold float64 // default 0.40
}
func DefaultConfig() Config

type Selection struct {
    Tier    Tier
    Tests   []string // final ranked list, direct tests first
    Direct  []string // changed/new test files, always run
    Reason  string   // human-readable escalation cause
}

type Inputs struct {
    Map        *mapstore.Map
    Changes    []gitctx.Change
    Adapter    *adapter.Adapter
    Cfg        Config
    AllTests   []string          // from adapter.List; needed for T2 and for direct-tier discovery
    Distance   func(sha string) int // wraps gitctx.CommitDistance; -1 means unknown
    Cycles     int               // from meta.json, for DriftGuard
    ImportOnly func(rel string) []string // static-import fallback; see M2
}

func Select(in Inputs) Selection

// Rank orders tests by: descending |F ∩ changed| / |F|, then S=="fail" first,
// then ascending len(F), then ascending D.
func Rank(m *mapstore.Map, tests, changedFiles []string) []string
```

## internal/runner

```go
type RunResult struct {
    Outcomes []report.Outcome
    Coverage *coverage.Result
    Failed   []string
    ExitCode int
}

// Run executes the adapter's subset (or seed) command. It sets Adapter.Env,
// chunks test ids across multiple invocations when argv would exceed MaxArgvBytes,
// and merges the results. A chunk that exits with a mapped ExitCode (4=bad-selector,
// 5=no-tests-collected) is a fatal error, not a test failure.
func Run(a *adapter.Adapter, repoRoot string, tests []string, failFast bool) (*RunResult, error)
func Seed(a *adapter.Adapter, repoRoot string) (*RunResult, error)

const MaxArgvBytes = 100_000 // conservative; Windows CMD is 8191 chars, Linux ARG_MAX is 2MB

func Chunk(tests []string, maxBytes int) [][]string

// ErrSysmonContext is returned when the run emitted coverage.py's
// "no-sysmon-context" warning. Callers MUST exit 3. Never proceed with the map.
// MEASURED: pytest prints this warning on STDOUT, in its warnings summary, and
// stderr is empty — the runner scans the COMBINED stream.
var ErrSysmonContext = errors.New("dynamic contexts unavailable: COVERAGE_CORE=sysmon")

// FatalExitError is a chunk that exited with a code mapped in Adapter.ExitCodes
// (4=bad-selector, 5=no-tests-collected). It is NOT a test failure; the CLI
// recovers it with errors.As and exits 2.
type FatalExitError struct {
    Chunk int
    Code  int
    Label string
}
func (e *FatalExitError) Error() string
```

## internal/uncovered

```go
type Class int
const (
    Covered Class = iota
    Uncovered
    ImportTime
)

type ClassifiedRange struct {
    Range gitctx.LineRange
    Class Class
}

type FileReport struct {
    Path   string
    Ranges []ClassifiedRange
}

// Classify intersects each Change's line ranges with fresh post-run coverage.
// A line covered by any test is Covered. A line present only in Result.ImportTime is
// ImportTime and MUST NOT be reported as Uncovered. Everything else is Uncovered.
func Classify(changes []gitctx.Change, cov *coverage.Result) []FileReport

func (r FileReport) UncoveredLines() int
```

## internal/doctor

```go
type Hub struct {
    Path      string
    TestCount int
    Fraction  float64 // TestCount / total tests in the map
}

// Hubs returns files sorted by descending TestCount.
func Hubs(m *mapstore.Map) []Hub
```

## .rtdd/meta.json

```json
{"v":1,"adapter":"python","seeded_at":"a3f21e0","cycles":7}
```

`cycles` increments on every completed `run`, pass or fail — a failing streak must still
buy an eventual `DriftGuard` full run.

## CLI surface

```
rtdd status
rtdd seed
rtdd which   [--base <ref>] [--json]
rtdd run     [--base <ref>] [--fail-fast] [--json]
rtdd verify
rtdd doctor
rtdd explain <file>
rtdd map compact
rtdd init
```

`--json` emits a machine-readable object for agent consumption. Its schema is defined in
plan M2, task "JSON output", and is the interface the agent front-ends depend on.

---

# Amendments — 2026-08-26, post-planning reconciliation

Symbols the five milestone plans require, plus corrections where **measurement contradicted
the original contract**. Measured items are marked **[M]** and were reproduced on a real
pytest/coverage.py install. These override anything above them in this file.

## Corrections to the contract above

### coverage.ReadSQLite — branch mode empties `line_bits` **[M]**

With `[run] branch = True` in the host repo's coverage config, `line_bits` has **zero rows**
and all data lands in the `arc` table (measured: 0 `line_bits`, 60 `arc`). The original query
would return an empty map on a run that exits 0 — a silent-corruption path identical in shape
to the sysmon bug.

`ReadSQLite` MUST branch on `meta.has_arcs`:

```go
// line mode  (has_arcs = 0):
//   SELECT f.path, c.context, lb.numbits FROM line_bits lb
//     JOIN file f ON f.id=lb.file_id JOIN context c ON c.id=lb.context_id
//
// branch mode (has_arcs = 1):
//   SELECT f.path, c.context, a.fromno, a.tono FROM arc a
//     JOIN file f ON f.id=a.file_id JOIN context c ON c.id=a.context_id
//   covered lines = {fromno | fromno > 0} ∪ {tono | tono > 0}
//
// The arc rule reproduces the line-mode answer exactly on every measured pair.
// Negative fromno/tono encode entry/exit pseudo-arcs and are excluded.
```

### runner — the sysmon warning is on STDOUT, not stderr **[M]**

pytest prints `no-sysmon-context` in its warnings summary on **stdout**; stderr was empty
(measured: stdout hits 1, stderr hits 0). A stderr-only scan finds nothing and the corruption
ships. `Run` and `Seed` MUST scan **combined** stdout+stderr and return `ErrSysmonContext`.

### adapter — `{src}` is dropped; use bare `--cov` **[M]**

`--cov=` with an empty value makes pytest exit 1 and record nothing, and any RTDD-guessed
`{src}` reintroduces exactly the "seed and subset disagree on scope" failure the spec warns
about. Measured: bare `--cov` honours the host's `[run] source` **and** `omit` identically
for both commands. `{src}` is removed from `Seed`/`Subset` templates and from `Expand`'s
variable set. Spec §8's sample YAML is corrected accordingly.

### runner — never use `--cov-append`; merge in Go **[M]**

Each chunk's pytest run **erases** `.coverage` (measured: after chunk 2, only chunk 2's
contexts remained). `--cov-append` would fix that but also absorbs any stale pre-existing
`.coverage`. `Run` instead reads and merges each chunk's `.coverage` in Go via
`coverage.Result.Merge` before the next chunk starts.

### report.ReadReportLog — the call-phase-only rule loses tests **[M]**

The original comment ("only call-phase TestReport entries produce an Outcome") is
self-contradictory: a **skipped** test and a **fixture-error** test emit no `call` entry at
all, so both would vanish from the map. Corrected rule:

| phase outcome | Outcome.Status |
|---|---|
| `call` passed | `pass` |
| `call` failed | `fail` |
| `call` skipped (`pytest.skip()` in the body) | `skip` |
| `setup` skipped (no call entry) | `skip` |
| `setup` failed (no call entry) | `error` |
| `call` passed but `teardown` failed | `error` |

`DurationMS` is the `call` phase's duration when there is one, and the sum of the
phases present (setup+teardown) when there is not. The JSONL's `duration` is a float
in seconds; `DurationMS` is milliseconds.

## Additions

```go
// internal/paths
func MatchGlob(pattern, rel string) bool   // doublestar semantics: ** crosses separators

// internal/mapstore
type Meta struct {
    V        int    `json:"v"`
    Adapter  string `json:"adapter"`
    SeededAt string `json:"seeded_at"`
    Cycles   int    `json:"cycles"`
}
func LoadMeta(path string) (Meta, error)   // missing file => zero Meta, nil error
func SaveMeta(path string, m Meta) error
// LoadWith is Load parameterised by the commit-age comparator, so duplicate-row
// resolution can pick the OLDER commit without mapstore importing gitctx.
func LoadWith(path string, older func(a, b string) string) (*Map, error)

// internal/gitctx
func RepoRoot(dir string) (string, error)
// RawDiff returns `git diff --unified=0 <base>` output verbatim, for hunk parsing.
func RawDiff(repoRoot, base string) (string, error)

// internal/adapter
// Both loaders return the whole set: detection picks one from it, and the same
// validation runs over a host repo's adapters and the embedded ones.
func LoadFS(fsys fs.FS, dir string) ([]*Adapter, error) // reads adapters/ embedded via go:embed
func Builtin() ([]*Adapter, error)                      // "python" resolves without a filesystem
// ExpandTests exists because Expand's map[string]string cannot carry test ids containing
// spaces, brackets or "::" — argv elements must not be re-split by the shell.
func ExpandTests(tmpl string, tests []string, vars map[string]string) ([]string, error)

// internal/coverage
// Merge folds other into r: per-test file/line sets union, ImportTime unions.
func (r *Result) Merge(other *Result)

// internal/runner
// FatalExitError wraps a mapped exit code (4 bad-selector, 5 no-tests-collected).
// These are FATAL, never a test failure — a bad selector means the map is stale and
// silently reporting "0 failures" would be a false green.
type FatalExitError struct{ Code int; Meaning string }
func (e *FatalExitError) Error() string
func List(a *adapter.Adapter, repoRoot string) ([]string, error)

// internal/uncovered
func ParseHunks(diff string) map[string][]gitctx.LineRange
// WithLines attaches parsed hunk ranges to changes lacking them.
func WithLines(changes []gitctx.Change, diff string) []gitctx.Change
type Summary struct{ Covered, Uncovered, ImportTime int }
func Summarize(reports []FileReport) Summary

// internal/doctor
// Caveat returns the fan-out warning text. It MUST appear in doctor's output:
// anything executed once per process (@lru_cache, singletons, DI containers,
// session-scoped fixtures) has a fan-out of 1, so the most-coupled file can
// appear as the cleanest.
func Caveat() string

// internal/importscan
// Scanner shells out to an embedded Python AST script. The engine is Go and must
// never parse Python itself. Satisfies selector.Inputs.ImportOnly.
type Scanner struct{ RepoRoot string; Python string }
func (s *Scanner) Scan(target string, testFiles []string) ([]string, error)

// internal/initrepo
type Action int
const (
    Created Action = iota
    Merged      // appended a delimited block to an existing AGENTS.md/CLAUDE.md
    Unchanged
)
type Block struct{ Path string; Action Action }
func Install(repoRoot string, force bool) ([]Block, error)

// internal/pytestfixture — TEST-ONLY helper, never imported by non-test code.
// Shipped shape, see the internal/pytestfixture section above: the fixture's contents
// are pinned by the package, so callers pass a directory rather than a file map, and
// nothing takes a *testing.T.
func Materialize(dir string) error          // writes the pinned pytest project into dir
func InitGit(dir string) error              // real `git init` + initial commit
func HavePytest() bool                      // reports whether pytest is on PATH
```

## Rule for future additions

A task that needs a symbol not defined here MUST add it here **in the same commit** that
uses it. Two packages with the same name and different shapes is the specific failure this
file exists to prevent, and it is the failure mode that parallel agents produce by default.

---

## M1a amendments

Added by [`01-m1a-core.md`](01-m1a-core.md). These are part of the contract.

### internal/paths

```go
// MatchGlob reports whether rel matches a slash-separated glob pattern.
// "*" and "?" match within one path segment; "**" matches zero or more whole segments.
func MatchGlob(pattern, rel string) bool

// ValidateGlob reports whether pattern is a well-formed glob.
func ValidateGlob(pattern string) error
```

`adapter.Load` runs `ValidateGlob` over every glob field (`test_globs`, `source_globs`,
`opaque`, `full_escalate`), so a malformed pattern is a configuration error (exit 2) that
names the offending field and pattern. `MatchGlob` panics on a pattern `ValidateGlob`
rejects: returning `false` would hide a typo behind a plausible "this is not a test file",
which is what let a bad `test_globs` classify nothing and empty the direct tier.

### internal/mapstore

```go
// LoadWith is Load with an explicit commit-age comparator, used to resolve duplicate
// `t` lines left by a union merge. Load(path) is LoadWith(path, nil); with a nil
// comparator the first line's C wins — deterministic, but age-blind.
func LoadWith(path string, older func(a, b string) string) (*Map, error)

// Meta is .rtdd/meta.json.
type Meta struct {
    V        int    `json:"v"`
    Adapter  string `json:"adapter"`
    SeededAt string `json:"seeded_at"`
    Cycles   int    `json:"cycles"`
}

func LoadMeta(path string) (Meta, error) // missing file returns the zero Meta and a nil error
func SaveMeta(path string, m Meta) error
```

Compaction (`rtdd map compact`) is defined as `LoadWith` followed by `Save`: `LoadWith`
resolves duplicates and `Save` writes exactly one line per `t`. There is no separate
entry point.

### internal/gitctx

```go
// RepoRoot returns the absolute, cleaned top level of the git work tree containing start.
func RepoRoot(start string) (string, error)

// String returns "added" | "modified" | "deleted" | "renamed" | "untracked".
func (s Status) String() string
```

`Untracked` is reserved for callers that need the distinction. Per the `ChangedSet` doc
comment, `ChangedSet` itself reports an untracked file as `Added` with the whole file as
one `LineRange`.

### internal/selector

```go
type Inputs struct {
    Map        *mapstore.Map
    Changes    []gitctx.Change
    Adapter    *adapter.Adapter
    Cfg        Config
    AllTests   []string             // from adapter.List; needed for T2 and for direct-tier discovery
    Distance   func(sha string) int // wraps gitctx.CommitDistance; -1 means unknown
    Cycles     int                  // from meta.json, for DriftGuard
    Merge      bool                 // ADDED: HEAD is a merge commit; escalates to T1 (spec §4)
    ImportOnly func(rel string) []string // static-import fallback; see M2
}
```

A zero `Config` (every field zero) is treated as `DefaultConfig()`.

### CLI surface

```
rtdd status [--adapter <path>]
rtdd which  [--base <ref>] [--json] [--adapter <path>]
```

`--adapter` defaults to `.rtdd/adapter.yaml`. It exists because `adapter.Detect` and
`adapter.LoadAll` are M1b; detection replaces the default in M1b and the flag stays as an
override. `rtdd which --json` output is **provisional in M1a**; plan M2 task "JSON output"
freezes the schema.

`rtdd which` may never narrow the selection silently. Two fields carry that guarantee into
the JSON, and the M2 schema freeze inherits both:

- `"complete"` — `false` when the tier is T2 and the suite was not enumerated, i.e. when
  `tests` is a partial list of the run. Direct tests present in the list do not make it
  complete. The human output prints the matching note under the same condition.
- `"warnings"` — non-empty when the selection is narrower than it looks. In M1a the one
  warning is a missing adapter, which disables file classification entirely; `"adapter"`
  is `""` in that case. The human output prints the same text as a `WARNING:` line.

### internal/gitctx/gittest — test-support helpers

Git is never mocked, so every git-dependent test needs a real `git init` in `t.TempDir()`;
at the same time `internal/gitctx` is the only part of the tree permitted to invoke git.
`gittest` reconciles the two: it lives under `internal/gitctx/`, and it is the single place
outside that package's own tests where a test fixture repository is built. Nothing outside
a `_test.go` file may import it.

```go
package gittest

func Init(t *testing.T) string                                // real `git init` in t.TempDir()
func Run(t *testing.T, dir string, args ...string) string     // a git subcommand, pinned env
func Write(t *testing.T, dir, rel, content string)            // write dir/rel, mkdir -p
func Commit(t *testing.T, dir, msg string) string             // stage all + commit, returns short SHA
func HeadShort(t *testing.T, dir string) string
```

### cmd/rtdd — internal to `main`

```go
func run(args []string, stdout, stderr io.Writer) int
type env struct { root, mapPath, metaPath, adPath string; m *mapstore.Map; meta mapstore.Meta; ad *adapter.Adapter }
// loadEnv resolves the repo root and loads .rtdd/. The returned int is the process exit
// code to use when err is non-nil: 3 for a fatal environment error, 2 for a bad config.
func loadEnv(adapterPath string) (*env, int, error)
func cmdStatus(args []string, stdout, stderr io.Writer) int
```

`main` is a one-liner around `run` so every command is testable with in-memory writers and
an asserted exit code. `env.ad` is nil when no adapter file is present — that is a reported
state (`adapter: none`), not an error, because `adapter.Detect` is M1b.
