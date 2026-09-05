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
internal/importscan/  static import fallback for import-time-only files
internal/runner/     subprocess execution, argv chunking
internal/uncovered/  line classification
internal/doctor/     fan-out analysis
internal/initrepo/   rtdd init: gitattributes, config, agent front-ends
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

// RawDiff returns the full text of `git diff --unified=0 -M <base>` for the working
// tree. An empty base means HEAD, as in ChangedSet. Renames are detected so the
// new-side path is authoritative, and the git configs that rewrite the `+++ b/<path>`
// header are pinned off. Errors carry git's own stderr, never swallowed.
func RawDiff(repoRoot, base string) (string, error)

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

    // Contract v2 (spec §4.2). Optional: an omitted key defaults to SelectionCoverage,
    // so every v1 adapter keeps its meaning unedited.
    Selection     string            `yaml:"selection"` // "coverage" (default) | "static"

    // The rest of contract v2: how a static-tier adapter finds and names its tests
    // (spec §4.2) and what its runner needs installed first (spec §4.3). All optional,
    // none defaulted.
    ReportPath    string            `yaml:"report_path"` // where the runner leaves its outcome file
    IDTemplate    string            `yaml:"id_template"` // a parsed id, rendered back into a selector
    TestFor       []string          `yaml:"test_for"`    // correspondence templates, tried IN ORDER
    Importscan    *Importscan       `yaml:"importscan"`  // nil means import ranking is skipped
    Requires      []Requirement     `yaml:"requires"`

    // Src is the file this adapter was read from: an embedded name ("python.yaml") or an
    // on-disk path for a host-authored one. Never declared in YAML — it is how doctor and
    // every error message name the file, and a declaration could lie about it.
    Src           string            `yaml:"-"`
}

// Importscan is the optional per-language import scanner: the engine runs a script and
// never parses the language itself (D8, the precedent internal/importscan set). Declaring
// one half without the other is a configuration error naming the missing field.
type Importscan struct {
    Command string `yaml:"command"` // e.g. "node {script}"
    Script  string `yaml:"script"`  // shipped beside the adapter
}

// Requirement is one binary this adapter cannot work without, and why. Both fields are
// required: doctor prints the reason verbatim (spec §4.3).
type Requirement struct {
    Bin    string `yaml:"bin"`
    Reason string `yaml:"reason"`
}

// Selection fidelity. An adapter declaring SelectionStatic must declare CoverageNone and
// must NOT declare a seed; CoverageNone is legal only under SelectionStatic. All three
// rejections are configuration errors (exit 2) raised by Load.
const (
    SelectionCoverage = "coverage"
    SelectionStatic   = "static"
    CoverageNone      = "none"
)

// report takes two values. "junit-xml" is carried by the contract from M6a on and has no
// parser until M6c; it requires BOTH report_path and id_template, because a <testcase>
// that cannot round-trip into `subset` is worthless (audit A6).
//
// Template placeholders are per-field vocabularies, and an unrecognised one is exit 2
// naming the file, the field and the placeholder:
//   test_for:     {dir} {name}
//   id_template:  {file} {classname} {name}

func Load(path string) (*Adapter, error)
func LoadAll(dir string) ([]*Adapter, error)

// LoadFS reads every *.yaml under dir in fsys, sorted by adapter name.
// LoadAll is LoadFS(os.DirFS(dir), ".").
func LoadFS(fsys fs.FS, dir string) ([]*Adapter, error)

// Builtin returns the adapters embedded in the binary from the root `adapters`
// package (//go:embed *.yaml), so rtdd ships as a single static file.
func Builtin() ([]*Adapter, error)

// Host-authored adapters (spec §4.5). The shipped set is a convenience, not the
// boundary of support: a language RTDD has never heard of is supported by writing YAML
// into the host repo, with no engine change and no release.
const HostAdapterDir = ".rtdd/adapters"

// Invalid is one host adapter file that failed to load. It is a value, not a returned
// error, so a half-written adapter does not take the working ones down with it.
type Invalid struct {
    Path string
    Err  error
}

// LoadHost reads every *.yaml directly under <repoRoot>/.rtdd/adapters, discarding the
// files that do not load. A missing directory is not an error.
func LoadHost(repoRoot string) ([]*Adapter, error)

// LoadHostReport is LoadHost with the discarded files kept, so `rtdd doctor` can name the
// failing file and the failing field. The error is reserved for an unreadable directory.
func LoadHostReport(repoRoot string) ([]*Adapter, []Invalid, error)

// Available returns the built-ins overlaid with the host's adapters, sorted by name. A
// host adapter whose name equals a built-in's REPLACES it: §4.5 makes host YAML the real
// boundary of support, so a repo must be able to correct a shipped adapter without an
// RTDD release. The override is never silent — rtdd doctor names it. Two HOST files
// claiming one name is an error; there is no principled winner.
func Available(repoRoot string) ([]*Adapter, error)

// AvailableReport is Available for callers that must say WHY a host adapter is missing.
func AvailableReport(repoRoot string) ([]*Adapter, []Invalid, error)

// IsHostAuthored reports whether a came from the repo's .rtdd/adapters/ rather than the
// binary.
func IsHostAuthored(repoRoot string, a *Adapter) bool

// Fidelity is how a selection was derived (spec §6). The string values are the wire format
// for --json and must not be reworded.
type Fidelity string

const (
    FidelityExecution Fidelity = "execution-derived"
    FidelityStatic    Fidelity = "static"
    FidelityNone      Fidelity = "none"
)

// Fidelity reports the best selection this adapter can ever produce. It is DERIVED from
// `selection` and `coverage`, never asserted beside them, so an adapter declaring it
// records nothing can never report execution-derived selection. It is a property of the
// declaration and not of the repository's state: an unseeded Python repo is still
// execution-derived, it just has no map yet. A static adapter with neither a test_for
// template nor an importscan command is `none`, not `static` — all it could offer is the
// path proximity spec §7 pre-registers the static tier AGAINST. Never blank: a nil adapter
// is FidelityNone.
func (a *Adapter) Fidelity() Fidelity

// Unmet returns the requirements whose bin does not resolve, in declaration order.
// lookPath is injected rather than calling exec.LookPath directly so what happens to be
// installed on the machine running the tests never decides whether this is correct;
// cmd/rtdd passes exec.LookPath.
func (a *Adapter) Unmet(lookPath func(string) (string, error)) []Requirement

// UnmetFinding pairs one unmet requirement with the adapter that declared it. Spec §4.3:
// an unmet prerequisite surfaces at doctor/init time, never as a mid-run parse failure
// against a report file that was never written.
type UnmetFinding struct {
    Adapter string
    Req     Requirement
}

// UnmetFindings collects every unmet prerequisite of every adapter it is given, adapter
// by adapter, each in declaration order. Callers pass the DETECTED adapters: a binary
// only some other toolchain's adapter wants is not a finding about this repository.
// It lives here rather than beside one renderer because spec §4.3 requires the same
// finding at BOTH `doctor` and `init` time.
func UnmetFindings(adapters []*Adapter, lookPath func(string) (string, error)) []UnmetFinding

// Detect returns the adapter whose Detect globs match a file in repoRoot.
// Exactly one match required; zero or multiple is an error (polyglot is out of scope in v1).
func Detect(repoRoot string, adapters []*Adapter) (*Adapter, error)

// DetectAll is the single repo walk Detect is the arity check over: it reports every
// adapter with a matching marker, in the order the adapters were given. `rtdd init`
// gates on "at least one" (spec §5), which is a weaker question than selection asks —
// a repo two adapters match is still a repo RTDD can be installed into.
func DetectAll(repoRoot string, adapters []*Adapter) ([]*Adapter, error)

// UnsupportedMarkers maps a well-known toolchain marker to the language it announces,
// keyed by file name or `*.ext`. It drives ONE message — init's refusal, which spec §5
// requires to name what the repository does contain — and nothing else: it is never
// consulted by detection, selection or classification, and an entry is not a claim of
// support. Only an adapter supports a language.
var UnsupportedMarkers map[string]string

// UnsupportedToolchains names the markers present in repoRoot as sorted
// "package.json (JavaScript/TypeScript)" strings. It reads the ROOT only,
// non-recursively: a marker deep in the tree is as likely to belong to a fixture or an
// example as to the repository itself.
func UnsupportedToolchains(repoRoot string) []string

// IsTestFile: matches TestGlobs AND is not a FullEscalate match. The FullEscalate term
// is a deliberate narrowing of the predicate: a fixture module such as tests/conftest.py
// matches a broad test glob like tests/**/*.py but collects no tests, and naming it as a
// selector makes the runner exit 5 (no-tests-collected), which is fatal. A change to it
// escalates to a full run through IsFullEscalate instead.
// Side effect, and it is load-bearing: IsInstrumentable is "SourceGlobs AND not
// IsTestFile AND not IsOpaque", and escalation is none of those three — so a conftest.py
// that sits inside SourceGlobs (src/conftest.py under source_globs: ["src/**/*.py"]) is
// IsInstrumentable == true, where the unqualified predicate would have made it a test
// file and therefore not instrumentable.
func (a *Adapter) IsTestFile(rel string) bool
func (a *Adapter) IsOpaque(rel string) bool
func (a *Adapter) IsFullEscalate(rel string) bool
// IsInstrumentable: matches SourceGlobs AND is not a test file AND is not Opaque.
func (a *Adapter) IsInstrumentable(rel string) bool

// Expand substitutes {out} and {log} into a command template and returns argv. {src} is
// NOT in the set: the amendment below dropped it in favour of a bare --cov.
// The template is tokenised on whitespace BEFORE substitution, so a substituted value
// is never re-split. A template containing {tests} is an error here; an unrecognised
// {placeholder} is an error, never a literal passed through to the runner.
//
// Expand has no closed variable set — it resolves whatever key it is handed — so nothing
// in internal/adapter structurally prevents a host adapter reintroducing --cov={src}.
// What keeps {src} out of expanded argv is the var map the caller supplies: runner.Run
// passes only {out} and {log}, and an adapter naming {src} therefore fails Expand as an
// unrecognised placeholder rather than silently narrowing coverage scope.
func (a *Adapter) Expand(tmpl string, vars map[string]string) ([]string, error)

// ExpandTests is Expand for a template containing {tests}. Each test id becomes its own
// argv element, spliced verbatim with no quoting or escaping, so ids containing spaces,
// '[', ']', '-' or '|' round-trip intact — measured real ids include
// `tests/test_a.py::test_param[1-one two]`. An empty tests slice is an error: a subset
// command with the ids dropped would run the whole suite. CONTRACT ADDITION.
func (a *Adapter) ExpandTests(tmpl string, vars map[string]string, tests []string) ([]string, error)
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

// TierTS is added between TierT1 and TierT2 by the M6b amendments at the end of this
// document; the set above is the M1a-era one. The resolution order those amendments fix
// is: T2 escalations, T1 escalations, T0, TS, empty.

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
    ImportOnly func(rel string) []string // importscan.Scanner.TestsImporting; static-import fallback
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

// Seed runs the adapter's seed template ONCE over the whole suite, with no test ids
// and no chunking. It reuses Run's execution path, so COVERAGE_CORE forcing and the
// combined-stream sysmon scan apply identically. A seed is the only operation that
// may shrink a map row, so every condition Run treats as fatal is fatal here too —
// exit 5 included: seeding an empty suite must never write an empty map.
func Seed(a *adapter.Adapter, repoRoot string) (*RunResult, error)

// List returns every test id the adapter's list command reports, in collection
// order. Needed for the T2 tier and for direct-tier discovery. An exit code
// mapped to "no-tests-collected" yields an empty list and a nil error — an empty
// suite is empty, not fatal; any other mapped code is a *FatalExitError. That is
// the asymmetry with Run, where the same code means the ids RTDD produced selected
// nothing. Collection order is preserved, never sorted.
func List(a *adapter.Adapter, repoRoot string) ([]string, error)

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

## internal/importscan

RTDD's single use of static analysis (spec §6, D14). Shells out to an embedded Python AST
script; the Go engine never parses Python itself.

```go
// Scan returns, for each target, the test files whose module transitively imports it.
// Import cycles terminate via a visited set. A target no module resolves to maps to an
// empty slice, never a missing key.
func Scan(repoRoot string, targets, tests []string) (map[string][]string, error)

// Scanner memoises Scan across repeated lookups within one command invocation.
// It is the value passed as selector.Inputs.ImportOnly.
type Scanner struct { /* unexported */ }

func NewScanner(repoRoot string, tests []string) *Scanner

// TestsImporting returns the test files whose module transitively imports rel.
// On scanner error it returns nil; the error is retained and reported by Err.
// A failed scan degrades selection, it never fails the command.
func (s *Scanner) TestsImporting(rel string) []string

// Err returns the first error any TestsImporting call encountered, or nil.
func (s *Scanner) Err() error
```

## internal/uncovered

```go
// ParseHunks parses `git diff --unified=0` output and returns, per NEW-file path,
// the line ranges that exist in the new file. Hunks whose new-side count is 0
// (pure deletions) contribute nothing. Files whose new side is /dev/null are omitted.
// The hunk header's context suffix may itself contain "@@"; the range region is cut
// at the FIRST following " @@". A missing count means 1.
func ParseHunks(diff string) map[string][]gitctx.LineRange

// WithLines returns changes with Lines populated from rawDiff. It is authoritative:
// it OVERWRITES any Lines already present, so there is exactly one source of truth.
// Deleted changes get nil Lines. A change absent from rawDiff (an untracked file git
// diff never lists) gets the whole file as one range, counted from disk; a missing or
// empty file gets nil Lines. Renamed changes are matched on their NEW path, which is
// what Change.Path holds. The input slice is never mutated.
func WithLines(repoRoot string, changes []gitctx.Change, rawDiff string) ([]gitctx.Change, error)

type Class int
const (
    Covered Class = iota
    Uncovered
    ImportTime
)

func (c Class) String() string // "covered" | "uncovered" | "import-time"

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
// Deleted changes, and changes with no Lines, produce no FileReport. Output is sorted
// by Path; each file's ranges are sorted ascending and adjacent lines of the same Class
// are coalesced. Callers pass only instrumentable changes.
//
// One exception, spec §6 / audit A1: a file coverage MEASURED but that NO test context
// touches is an import-time-only file, and every one of its changed lines is ImportTime.
// Coverage stores only executed lines, so within such a file a blank line is
// indistinguishable from a dead statement, and reporting the blanks in a changed
// dataclass/Enum/constants module as Uncovered is exactly the false positive A1 forbids.
// A file coverage never saw at all is NOT import-time-only: it is wholly Uncovered.
func Classify(changes []gitctx.Change, cov *coverage.Result) []FileReport

func (r FileReport) UncoveredLines() int

type Summary struct {
    Files           int
    CoveredLines    int
    UncoveredLines  int
    ImportTimeLines int
}

// Summarize totals a set of FileReports.
func Summarize(reports []FileReport) Summary
```

## internal/doctor

```go
type Hub struct {
    Path      string
    TestCount int
    Fraction  float64 // TestCount / total tests in the map
}

// Hubs returns files sorted by descending TestCount, ties broken by ascending Path.
// Fraction is TestCount / m.Len(); an empty map yields an empty, non-nil slice.
func Hubs(m *mapstore.Map) []Hub

// Caveat is the fan-out warning `rtdd doctor` MUST print alongside its table (spec §9).
// It is a const, not a function: the limitation belongs in the tool's own output, and a
// const cannot be forgotten at a call site the way a rendering step can. Anything
// executed once per process (@lru_cache, module singletons, DI containers,
// session-scoped fixtures) has a fan-out of 1, so the most coupled file in the repo can
// appear as the cleanest.
const Caveat = "CAVEAT: anything executed once per process — ..."

// StaticCaveat is the selection-fidelity warning `rtdd doctor` MUST print alongside any
// `static` or `none` row (spec §6). A static selection is derived from declaration rather
// than from a recorded run, so it can miss a test execution-derived selection would have
// caught, and passing it is weaker evidence. Like Caveat it is a const so the text cannot
// drift between the surfaces that print it.
const StaticCaveat = "CAVEAT: a static selection is derived from declared correspondence ..."
```

## internal/initrepo

```go
type Action struct {
    Path string // repo-relative
    Kind string // "created" | "updated" | "unchanged"
}

const (
    BeginMarker = "<!-- BEGIN RTDD -->"
    EndMarker   = "<!-- END RTDD -->"
)

// MergeManagedBlock returns existing with block installed between the markers.
// Never clobbers: content outside the markers is preserved byte for byte. With no
// markers present, block is appended after a blank line and the original comes first.
func MergeManagedBlock(existing, block string) string

func EnsureGitAttributes(repoRoot string) (Action, error) // adds ".rtdd/map.jsonl merge=union"
func EnsureConfig(repoRoot string) (Action, error)        // never overwrites an existing config
func EnsureFrontEnd(repoRoot, rel, block string) (Action, error)

// Block returns the managed agent front-end text, marker lines included.
func Block() string

// Run installs .gitattributes, .rtdd/config.yaml, AGENTS.md, CLAUDE.md and
// .cursor/rules/rtdd.mdc. Actions are returned in installation order.
func Run(repoRoot string) ([]Action, error)
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
rtdd doctor  [--limit <n>]
rtdd explain <file>
rtdd map compact
rtdd init
```

`--json` emits a machine-readable object for agent consumption. It is the interface the
agent front-ends depend on, so it is defined in full here. `cmd/rtdd/jsonout.go` builds it
(`BuildOutput`/`Output`); `cmd/rtdd/jsonout_test.go` holds it to every rule below.

### The schema, version 1

```json
{
  "schema": 1,
  "command": "run",
  "base": "HEAD",
  "adapter": "python",
  "tier": "T0",
  "reason": "3 map rows intersect the changed set",
  "complete": true,
  "warnings": [],
  "changed": [
    {"path": "src/logic.py", "status": "modified", "instrumentable": true,
     "lines": [{"start": 8, "end": 9}]},
    {"path": "src/constants.py", "status": "modified", "instrumentable": true,
     "lines": [{"start": 1, "end": 15}]}
  ],
  "selection": {
    "count": 2,
    "direct": ["tests/test_new.py"],
    "tests": ["tests/test_new.py", "tests/test_it.py::test_logic"],
    "import_fallback": {"src/constants.py": ["tests/test_it.py"]}
  },
  "run": {
    "executed": true,
    "passed": 2, "failed": 0, "skipped": 0, "errored": 0,
    "failures": [],
    "duration_ms": 1400
  },
  "uncovered": {
    "available": true,
    "files": [
      {"path": "src/constants.py",
       "ranges": [{"start": 1, "end": 15, "class": "import-time"}],
       "uncovered_lines": 0},
      {"path": "src/logic.py",
       "ranges": [{"start": 8, "end": 8, "class": "import-time"},
                  {"start": 9, "end": 9, "class": "uncovered"}],
       "uncovered_lines": 1}
    ],
    "summary": {"files": 2, "covered_lines": 0, "uncovered_lines": 1,
                "import_time_lines": 16}
  },
  "unmapped_files": ["src/constants.py"],
  "exit_code": 0
}
```

**Field contract.**

| Field | Type | Meaning |
|---|---|---|
| `schema` | int | Always `1` for this version. Consumers MUST reject an unknown value rather than guess. |
| `command` | string | `"run"` or `"which"`. |
| `base` | string | The `--base` ref actually used. |
| `adapter` | string | Detected adapter name. |
| `tier` | string | `"empty"`, `"direct"`, `"T0"`, `"T1"`, `"T2"` — `selector.Tier.String()`. |
| `reason` | string | Human-readable escalation cause; `""` when none. |
| `complete` | bool | Whether `selection.tests` is the WHOLE run. `false` exactly when `tier` is `"T2"` and the suite was not enumerated — `rtdd which` never enumerates it, so `which` reports `false` on every T2. Direct tests present in the list do NOT make it complete. Every other tier names its tests exhaustively and reports `true`, the empty tier included. |
| `warnings` | array of string | The caveats saying the selection is narrower, or less authoritative, than it looks — a missing adapter (file classification disabled), an empty selection, an unenumerated T2 suite, a failed import scan. Verbatim, in the order the command produced them. Never null; `[]` means there are none. The same sentences also go to stderr for a human, but a `--json` consumer normally discards stderr, so the document carries them too. |
| `changed[].path` | string | Repo-relative, slash-separated. |
| `changed[].status` | string | `"added"`, `"modified"`, `"deleted"`, `"renamed"`, `"untracked"`. |
| `changed[].instrumentable` | bool | Whether the adapter would instrument it. Only instrumentable files can appear in `uncovered.files`. |
| `changed[].lines` | array | New-file line ranges, 1-indexed inclusive. Empty for `deleted`. |
| `selection.count` | int | `len(selection.tests)`. |
| `selection.direct` | array | Changed/new test files, always run first. Never null. |
| `selection.tests` | array | Final ranked list, direct first. Never null. Empty is a legitimate outcome and is reported as `tier: "empty"`. |
| `selection.import_fallback` | object | file → tests chosen by the static import scan. `{}` when the fallback did not fire. |
| `run.executed` | bool | `false` for `which`, which runs nothing. |
| `run.passed`/`failed`/`skipped`/`errored` | int | Outcome counts; all `0` when `executed` is `false`. |
| `run.failures` | array of string | Failing test ids. Never null. |
| `run.duration_ms` | int | Sum of executed test durations. |
| `uncovered.available` | bool | `true` only when fresh post-run coverage exists. `which` always emits `false`. |
| `uncovered.reason` | string | Present only when `available` is `false`; explains why. |
| `uncovered.files` | array | Omitted when `available` is `false`. Sorted by `path`. |
| `uncovered.files[].ranges[].class` | string | `"covered"`, `"uncovered"`, `"import-time"`. **`"import-time"` is never `"uncovered"`.** |
| `uncovered.summary` | object | Totals over `files`. |
| `unmapped_files` | array | Changed instrumentable files no map row covers — the import-fallback trigger set. Never null. |
| `exit_code` | int | The process exit code. **`0` even when `uncovered.summary.uncovered_lines > 0`.** |

**Invariant, and it is tested:** `exit_code` is `1` if and only if `run.failed + run.errored
> 0`. A non-empty uncovered report never changes it.

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

### gittest — `internal/pytestfixture` shells out to git inline **[M]**

Two M1a rules collide on one file. `internal/gitctx` is the only part of the tree
permitted to invoke git; and nothing outside a `_test.go` file may import
`internal/gitctx/gittest`. `internal/pytestfixture` is a **non-test** file that builds a
real fixture repository, so it can satisfy only one of them — and importing `gittest` is
the worse violation, because `gittest` imports `testing` and that puts the testing
package on a non-test dependency graph (`go list -deps ./internal/pytestfixture`).

`pytestfixture.InitGit` therefore shells out to git **inline**, as the plan's Task 11
always specified, and does not import `gittest`. The shell-out rule is amended to name
`internal/pytestfixture` alongside `internal/gitctx` — a two-site allowance, not an open
one, and it is paid for by two guards in `internal/contract`: no non-`_test.go` file
imports `gittest`, and `go list -deps` for `./internal/pytestfixture`, `./cmd/rtdd` and
`./internal/adapter` contains no `testing`.

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
// RawDiff returns `git diff --unified=0 -M <base>` output verbatim, for hunk parsing.
// See the `## internal/gitctx` section above for the full entry: -M is not optional,
// because a rename must report its NEW-side path or its changed lines attach to nothing.
func RawDiff(repoRoot, base string) (string, error)

// internal/adapter
// Both loaders return the whole set: detection picks one from it, and the same
// validation runs over a host repo's adapters and the embedded ones.
func LoadFS(fsys fs.FS, dir string) ([]*Adapter, error) // reads adapters/ embedded via go:embed
func Builtin() ([]*Adapter, error)                      // "python" resolves without a filesystem
// ExpandTests exists because Expand's map[string]string cannot carry test ids containing
// spaces, brackets or "::" — argv elements must not be re-split by the shell. It is a
// METHOD on *Adapter and vars precedes tests, exactly as in the internal/adapter section
// above; the package-level `ExpandTests(tmpl, tests, vars)` form this block first carried
// never existed in code.
func (a *Adapter) ExpandTests(tmpl string, vars map[string]string, tests []string) ([]string, error)

// internal/coverage
// Merge folds other into r: per-test file/line sets union, ImportTime unions.
func (r *Result) Merge(other *Result)

// internal/runner
// FatalExitError wraps a mapped exit code (4 bad-selector, 5 no-tests-collected)
// from one chunk. These are FATAL, never a test failure — a bad selector means the
// map is stale and silently reporting "0 failures" would be a false green. Label is
// the adapter's name for the code; there is no separate Meaning field.
type FatalExitError struct{ Chunk int; Code int; Label string }
func (e *FatalExitError) Error() string
func List(a *adapter.Adapter, repoRoot string) ([]string, error)

// internal/uncovered — SUPERSEDED by the `## internal/uncovered` section above, which is
// the shipped shape. The planning sketch that stood here gave WithLines no repoRoot and no
// error, and summed the three classes as `Summary{Covered, Uncovered, ImportTime}`. Both
// were wrong in ways that matter: WithLines must read an untracked file from disk (git diff
// never lists one), which needs the repo root and can fail; and the shipped Summary counts
// Files alongside CoveredLines/UncoveredLines/ImportTimeLines, because the --json summary
// reports a file count the three line totals cannot reconstruct. ParseHunks, Classify,
// Class.String, FileReport.UncoveredLines and Summarize are all recorded in that section.

// internal/doctor — SUPERSEDED by the `## internal/doctor` section above, which is the
// shipped shape. The planning sketch that stood here made the fan-out warning a `Caveat()`
// function returning a string; it ships as `const Caveat` so the text is one immutable
// string every caller shares.

// internal/importscan — SUPERSEDED by the `## internal/importscan` section above, which is
// the shipped shape. The planning sketch that stood here declared a `Scanner` struct with
// exported RepoRoot/Python fields and a per-target `Scan` method; the shipped package is a
// single package-level `Scan` taking every target at once, because one Python subprocess
// that walks the tree once is the whole reason the scan is shelled out rather than inlined.
// The memoising `Scanner` that satisfies selector.Inputs.ImportOnly has since landed and is
// recorded in that section.

// internal/initrepo — SUPERSEDED by the `## internal/initrepo` section above, which is
// the shipped shape. The planning sketch that stood here named the enum `Action` and the record
// `Block`; the implementation inverts that (`Action` is the record, `Block()` returns the
// managed front-end text) and drops `force`, because nothing is ever clobbered and there
// is therefore nothing to force.

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
    ImportOnly func(rel string) []string // importscan.Scanner.TestsImporting; static-import fallback
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
the JSON, and the M2 schema freeze inherits both — see the `complete` and `warnings` rows of
the v1 field contract above, which are these fields as frozen. `rtdd run` emits both as well,
under the same rules:

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
a `_test.go` file may import it — `gittest` imports `testing`, and a non-test importer puts
`testing` on a production dependency graph. `internal/contract` enforces both halves: no
non-`_test.go` file imports this package, and no package's `go list -deps` names `testing`.
The one non-test fixture builder, `internal/pytestfixture.InitGit`, shells out to git
inline instead; see the amendment above.

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

// FidelityRow is one resolved adapter as `rtdd doctor` reports it: where it came from,
// whether it overrode a built-in, the selection fidelity this repository can achieve with
// it, and Why — the clause naming what determined that fidelity. A verdict with no cause
// attached is not an honesty surface, so Why is always printed (spec §6).
type FidelityRow struct {
    Name     string
    Src      string // repo-relative path for a host adapter, the embedded name otherwise
    Host     bool
    Override bool
    Fidelity adapter.Fidelity
    Why      string
}

// RenderFidelity formats the selection-fidelity block printed above the fan-out table.
// Pure. A `none` row always names the consequence (the full suite) and the fix; any
// `static` or `none` row is accompanied by doctor.StaticCaveat; an unloadable host adapter
// is named with the field that failed rather than being fatal (spec §4.5).
func RenderFidelity(rows []FidelityRow, invalid []adapter.Invalid) string

// RenderRequirements formats the unmet-prerequisite findings — binary, adapter, reason.
// Pure; no findings renders "" because a heading over an empty list reads as a problem.
func RenderRequirements(findings []adapter.UnmetFinding) string
func cmdSeed(args []string) int
func cmdRun(args []string) int
```

`main` is a one-liner around `run` so every command is testable with in-memory writers and
an asserted exit code. `env.ad` is nil when no adapter file is present — that is a reported
state (`adapter: none`), not an error, because `adapter.Detect` is M1b.

`cmdSeed` and `cmdRun` print to `os.Stdout`/`os.Stderr` rather than taking writers: both
stream a subprocess's progress, and both are asserted through their exit code and the
`.rtdd/` they leave behind. `cmdRun` calls `(*Map).Union` and never `(*Map).Replace` —
only `cmdSeed` may shrink a row (spec §4, D11, audit A4), and
`TestOnlySeedCallsMapstoreReplace` enforces it across the whole tree.

---

## M6a amendments — `init` reports what it detected and what it could not load

Added by [`06-m6a-adapter-contract-v2.md`](06-m6a-adapter-contract-v2.md) Tasks 5 and 7.
These are part of the contract.

### internal/install

```go
// AdapterRecord is one DETECTED adapter as .rtdd/config.yaml records it. Fidelity is a
// string, not adapter.Fidelity, so internal/install keeps depending on nothing: this
// package renders text and never resolves an adapter.
type AdapterRecord struct {
    Name      string
    Selection string
    Fidelity  string
}

// ConfigWithAdapters renders .rtdd/config.yaml with a record of what `rtdd init`
// detected (spec §5, the ≥1-adapter branch). The record is a HUMAN-READABLE trace of the
// last first-install, never an input: nothing reads it back, and `rtdd doctor` re-derives
// fidelity from .rtdd/adapters/ live and remains the source of truth. No records renders
// the unchanged three-key default — an empty `adapters:` key would claim RTDD looked and
// found none, which is a different thing from an install that made no such promise.
func ConfigWithAdapters(recs []AdapterRecord) string

// Plan gains the detected set. It is recorded into a NEWLY CREATED .rtdd/config.yaml and
// ignored when one already exists, because an existing config is one someone tuned.
func Plan(root string, files map[string]string, force bool, detected []AdapterRecord) ([]Step, error)
```

### cmd/rtdd — internal to `main`

```go
// lookPath resolves a prerequisite binary. A package variable rather than a direct
// exec.LookPath call so tests fix what is installed: a guard that asked the real PATH
// would pass on a laptop with node installed and fail on a minimal CI image.
var lookPath = exec.LookPath

// RenderRequirements moves out of doctor.go into requires.go, unchanged, because spec
// §4.3 requires the block at BOTH `doctor` and `init` time and a copy in each is two
// chances to drift apart. `doctor`'s private unmetFindings is replaced by the exported
// adapter.UnmetFindings for the same reason.

// adapterRecords derives the install.AdapterRecord set from the detected adapters:
// name, declared selection, and the fidelity that selection derives.
func adapterRecords(detected []*adapter.Adapter) []install.AdapterRecord

// writeNoAdapterRefusal gains the invalid host adapters, and names each failing file and
// field above the remedy text (PRD #229 AC5). "Write an adapter" is unusable advice to
// someone who wrote one.
func writeNoAdapterRefusal(stderr io.Writer, root string, invalid []adapter.Invalid)
```

`cmdInit` calls `adapter.AvailableReport` rather than `adapter.Available`, warns on every
host adapter that did not load exactly as `which` and `run` do, and prints
`RenderRequirements(adapter.UnmetFindings(detected, lookPath))` on **stderr** after a
successful install — still exiting **0**. An unmet prerequisite is not the "no adapter
detected" refusal: the repo has an adapter, so §5's gate is satisfied, and a binary
missing from this machine is no reason to decline to install, because the CI that runs the
suite may install it later.

`adapter.AvailableReport` returns the `[]Invalid` it has already collected **alongside**
the duplicate-host-name error. A repo can be wrong in two ways at once, and the duplicate
must not swallow the malformed-file report — that report is the only thing naming the file
and the field to edit.

---

## M6b amendments — the `TS` static selection tier

Added by [`06-m6b-static-tier.md`](06-m6b-static-tier.md). These are part of the contract.
The `TS` ranking's later levels and the `cmd/rtdd` wiring land with the sibling slices;
what is recorded here is what exists.

### internal/selector

```go
// TierTS is the static tier (spec §4.1), ordered between TierT1 and TierT2. Its String()
// is "TS". The documented resolution order in select.go is now: T2 escalations, T1
// escalations, T0, TS, empty.
//
// TS is attempted when, and only when, the coverage relation cannot answer:
//
//     (in.Adapter != nil && in.Adapter.Selection == adapter.SelectionStatic) || map.Len() == 0
//
// A SEEDED map that selects nothing stays an explicit TierEmpty. A static adapter that
// can produce no candidate returns the full suite at T2 with a reason naming what it
// lacks — never an empty TS, and never advice to run `rtdd seed`.

type Inputs struct {
    // … M1a fields …

    // Exists reports whether the repository has this repo-relative path; it resolves
    // test_for templates without the selector touching a filesystem. nil skips level 1.
    Exists func(rel string) bool

    // ImportDistance maps a changed file to the tests that transitively import it,
    // valued by the shortest number of hops. nil skips level 2.
    ImportDistance func(changed string) map[string]int
}
```

Ranking inside `TS`, most to least confident: declared `test_for` correspondence; import
distance, shortest first; longest shared directory prefix. **The third level orders a
selection and never contributes to one** — proximity alone is the `path` baseline spec §7
pre-registers this tier against, so a test that only sits near a changed file is not a
candidate.

`internal/selector` imports nothing that touches the world, and
`TestSelectorPackageStaysPure` enforces it across the package: every question about the
repository arrives through `Inputs` as an injected function.

### internal/adapter

```go
// TestForCandidate resolves this adapter's test_for templates against one changed source
// file and returns the first template naming a file the repository actually has.
// Templates are tried in declaration order; exists is injected so every caller up to
// selector.Select stays pure. {dir} and {name} are the only placeholders, already
// enforced by validateTemplates at load time.
func (a *Adapter) TestForCandidate(rel string, exists func(string) bool) (string, bool)
```

### internal/importscan

```go
// AdapterScanner runs an adapter's DECLARED importscan script and returns hop counts.
// The wire contract matches the embedded Python scanner's: a JSON request on the child's
// stdin, a JSON answer on its stdout, with distances instead of bare lists.
//
//     stdin :  {"root": "...", "targets": ["src/a.ts"], "tests": ["src/a.test.ts"]}
//     stdout:  {"src/a.ts": {"src/a.test.ts": 1}}
//
// `script` resolves against .rtdd/adapters/, beside the host YAML that declared it, and
// reaches the command template as {script}; argv is built by (*Adapter).Expand, so the
// engine never hands a shell a string. An adapter declaring no importscan yields an INERT
// scanner, not a nil one. A missing, failing or unparseable scanner degrades selection to
// levels 1 and 3 and is reported through Err; it never fails the command.
func NewAdapterScanner(repoRoot string, a *adapter.Adapter, tests []string) *AdapterScanner
func (s *AdapterScanner) Distances(changed string) map[string]int
func (s *AdapterScanner) Err() error
```

### cmd/rtdd — internal to `main`

```go
// repoExists and adapterImportDistance are the impure halves of the static tier: they are
// built in cmd/ and injected into selector.Inputs by BOTH `which` and `run`, through
// staticResolvers, so the advisory command and the executing command cannot wire them
// differently. A resolver left nil is a level SKIPPED, never a level that failed.
//
// adapterImportDistance returns the resolver AND a reader for the first scan failure:
// a declared scanner that fails narrows the selection, and a narrowing nobody reports
// reaches the reader as "no import reaches the changed set" — a sentence about a level
// that could not run. Both commands turn a non-nil error into a warning instead.
func repoExists(root string) func(rel string) bool
func adapterImportDistance(root string, ad *adapter.Adapter, tests []string) (func(string) map[string]int, func() error)
func staticResolvers(root string, ad *adapter.Adapter) (func(string) bool, func(string) map[string]int, func() error)

// staticTestCandidates enumerates the repository's own test files, by the adapter's test
// globs, for a declared scanner to rank against. The map cannot supply them — a static
// adapter records no coverage — and `which` never enumerates the suite. It is called only
// when the adapter declares an importscan, so a coverage repository pays no walk.
func staticTestCandidates(root string, ad *adapter.Adapter) []string
```

```go
// RenderNextStep is the line `rtdd init` closes with, derived from the DETECTED adapters
// because seeding is advice that only applies to an adapter that records coverage. A
// coverage-only repository gets the pre-M6b line byte for byte; a static-only repository
// is pointed at `rtdd which`; a mixed one is told both, each scoped by adapter name. A
// repository that matched nothing (--force) keeps the seed line.
func RenderNextStep(detected []*adapter.Adapter) string
```

`rtdd seed` against a `selection: static` adapter exits **2** naming the adapter, saying it
records nothing, and writes no map — it points at `rtdd which` instead. It is a
configuration error, not a run failure: a seed that silently succeeds having built no map
is what PRD #230 AC9 closes.
