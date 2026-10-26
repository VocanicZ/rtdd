package main

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/graphbuild"
	"github.com/VocanicZ/rtdd/internal/install"
	"github.com/VocanicZ/rtdd/internal/rounds"
)

// eventLogVar names the file every invocation appends one JSON line to when it exits, so
// a caller can count rtdd's work without scraping its output (#503). Unset, rtdd records
// nothing.
const eventLogVar = "RTDD_EVENT_LOG"

// maxEventLine bounds a line, newline included. One O_APPEND write this small is never
// interleaved with another process's, so concurrent invocations each leave a whole line.
const maxEventLine = 4096

// event is one line of RTDD_EVENT_LOG, schema v1. A field that does not apply to the
// command is omitted; the counts are the ones `rtdd which --json` (schema 3) reports.
type event struct {
	V          int               `json:"v"`
	TS         string            `json:"ts"`
	Cmd        string            `json:"cmd"`
	Argv       []string          `json:"argv,omitempty"`
	Truncated  bool              `json:"truncated,omitempty"` // argv, ctx or cwd dropped to fit maxEventLine
	Cwd        string            `json:"cwd,omitempty"`
	Repo       string            `json:"repo,omitempty"` // the git toplevel, when there is one
	Version    string            `json:"version"`
	Base       string            `json:"base,omitempty"`
	Head       string            `json:"head,omitempty"`
	Exit       int               `json:"exit"`
	DurationMS int64             `json:"duration_ms"`
	Graph      *eventGraph       `json:"graph,omitempty"`
	Changed    *int              `json:"changed_nodes,omitempty"`
	Round1     *int              `json:"round1,omitempty"`
	Round2     *int              `json:"round2,omitempty"`
	Untested   *int              `json:"untested,omitempty"`
	Deleted    *int              `json:"deleted_warnings,omitempty"`
	Findings   *int              `json:"findings,omitempty"` // doctor: over-linked names plus legacy files
	Init       *eventInit        `json:"init,omitempty"`
	Ctx        map[string]string `json:"ctx,omitempty"` // every HARNESS_* variable present

	start time.Time
	path  string
}

type eventGraph struct {
	Source     string `json:"source"`
	BuiltAt    string `json:"built_at"`
	StaleFiles int    `json:"stale_files"`
	Nodes      int    `json:"nodes"`
	Edges      int    `json:"edges"`
	Cache      string `json:"cache"` // warm | cold
}

// eventInit is what `rtdd init` created, migrated or reported: how many steps of each
// action its plan held.
type eventInit struct {
	DryRun bool           `json:"dry_run,omitempty"`
	Steps  map[string]int `json:"steps"`
}

// current is the event of the invocation run is dispatching, which commands fill as they
// go. It is nil when RTDD_EVENT_LOG is unset, and every recorder is a no-op on nil.
var current *event

// startEvent opens the invocation's event, or returns nil when nothing is to be recorded.
func startEvent(args []string) *event {
	path := os.Getenv(eventLogVar)
	if path == "" {
		return nil
	}
	e := &event{V: 1, Argv: args, Version: version, start: time.Now(), path: path}
	if len(args) > 0 {
		e.Cmd = args[0]
	}
	e.Cwd, _ = os.Getwd()
	for _, kv := range os.Environ() {
		if k, v, ok := strings.Cut(kv, "="); ok && strings.HasPrefix(k, "HARNESS_") {
			if e.Ctx == nil {
				e.Ctx = map[string]string{}
			}
			e.Ctx[k] = v
		}
	}
	current = e
	return e
}

// finish stamps the exit code and appends the line. A failure to write is ignored: rtdd's
// exit code and output never depend on the log.
func (e *event) finish(code int) {
	if e == nil {
		return
	}
	current = nil
	e.Exit = code
	e.DurationMS = time.Since(e.start).Milliseconds()
	e.TS = e.start.UTC().Format("2006-01-02T15:04:05.000Z")
	if e.Cwd != "" {
		if root, err := gitctx.RepoRoot(e.Cwd); err == nil {
			e.Repo = root
			e.Head, _ = gitctx.HeadSHA(root)
		}
	}
	line, ok := e.line()
	if !ok {
		return
	}
	f, err := os.OpenFile(e.path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return
	}
	f.Write(line) // one write: O_APPEND keeps it whole
	f.Close()
}

// line is the event as one newline-terminated JSON line under maxEventLine, dropping
// argv, then ctx and cwd, when a long command line or environment would overflow it.
func (e *event) line() ([]byte, bool) {
	shrink := []func(){
		func() { e.Argv, e.Truncated = nil, true },
		func() { e.Ctx, e.Cwd = nil, "" },
	}
	for i := 0; ; i++ {
		b, err := json.Marshal(e)
		if err != nil {
			return nil, false
		}
		if len(b) < maxEventLine {
			return append(b, '\n'), true
		}
		if i == len(shrink) {
			return nil, false
		}
		shrink[i]()
	}
}

// recordGraph notes the graph a command built.
func (e *event) recordGraph(res *graphbuild.Result) {
	if e == nil {
		return
	}
	cache := "cold"
	if res.CacheWarm {
		cache = "warm"
	}
	e.Graph = &eventGraph{Source: res.Source, BuiltAt: res.BuiltAtCommit, StaleFiles: len(res.StaleFiles),
		Nodes: len(res.Graph.Nodes), Edges: len(res.Graph.Edges), Cache: cache}
}

// recordWhich notes `rtdd which`'s answer: the counts its --json reports.
func (e *event) recordWhich(base string, r rounds.Result, warnings []string) {
	if e == nil {
		return
	}
	e.Base = base
	e.Changed, e.Round1, e.Round2 = intp(len(r.ChangedNodes)), intp(len(r.Round1)), intp(len(r.Round2))
	e.Untested, e.Deleted = intp(len(r.Untested)), intp(len(warnings))
}

// recordFindings notes how many findings `rtdd doctor` printed.
func (e *event) recordFindings(n int) {
	if e != nil {
		e.Findings = intp(n)
	}
}

// recordInit notes `rtdd init`'s plan, by action.
func (e *event) recordInit(steps []install.Step, dryRun bool) {
	if e == nil {
		return
	}
	e.Init = &eventInit{DryRun: dryRun, Steps: map[string]int{}}
	for _, s := range steps {
		e.Init.Steps[s.Action.String()]++
	}
}

func intp(n int) *int { return &n }
