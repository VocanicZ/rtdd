package importscan

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"

	"github.com/VocanicZ/rtdd/internal/adapter"
)

// AdapterScanner runs an ADAPTER'S DECLARED importscan script — spec §4.2's optional
// `importscan` key — and returns the hop counts level 2 of the static tier ranks on.
//
// It is the per-language sibling of Scanner, which runs RTDD's own embedded Python scan.
// The wire contract is deliberately the same shape, with distances instead of bare lists,
// so one JSON request on the child's stdin and one JSON answer on its stdout serves both:
//
//	stdin :  {"root": "/abs/repo", "targets": ["src/a.ts"], "tests": ["src/a.test.ts"]}
//	stdout:  {"src/a.ts": {"src/a.test.ts": 1}}
//
// The engine never parses another language itself (decision D8): the adapter ships the
// scanner and RTDD runs it.
//
// A scanner that is absent, missing, exits non-zero or writes unparseable JSON DEGRADES
// selection to levels 1 and 3 and records the failure in Err. It never fails the command:
// spec §4.2 makes importscan optional, so a broken one may be no worse than an absent one.
type AdapterScanner struct {
	repoRoot string
	ad       *adapter.Adapter
	tests    []string
	cache    map[string]map[string]int
	err      error
	// scan is one target's underlying run, indirected so tests can observe how often a
	// child process is started; memoisation is a claim about process count, not about
	// return values. Scanner carries the same seam for the same reason.
	scan func(target string) (map[string]int, error)
}

// NewAdapterScanner returns a scanner over repoRoot for the adapter's declared
// importscan, ranking over the given candidate test files.
//
// An adapter that declares no importscan yields an INERT scanner rather than a nil one:
// "this adapter skips level 2" is the ordinary case (PRD #230 AC7), and a nil-check the
// caller could forget is a panic waiting for the first adapter without a scanner.
func NewAdapterScanner(repoRoot string, a *adapter.Adapter, tests []string) *AdapterScanner {
	s := &AdapterScanner{
		repoRoot: repoRoot,
		ad:       a,
		tests:    tests,
		cache:    map[string]map[string]int{},
	}
	s.scan = s.runScript
	return s
}

// Distances returns the tests that transitively import changed, valued by the shortest
// number of import hops. It returns nil when the adapter declares no scanner and when the
// scan failed — the two cases a caller treats identically, because both mean level 2
// contributed nothing.
func (s *AdapterScanner) Distances(changed string) map[string]int {
	if s == nil || s.ad == nil || s.ad.Importscan == nil {
		return nil
	}
	if v, ok := s.cache[changed]; ok {
		return v
	}
	v, err := s.scan(changed)
	if err != nil {
		if s.err == nil {
			s.err = err
		}
		s.cache[changed] = nil
		return nil
	}
	s.cache[changed] = v
	return v
}

// Err returns the first error any Distances call encountered, or nil. An adapter that
// declares no scanner never produces one: an absent scanner is not a failure.
func (s *AdapterScanner) Err() error {
	if s == nil {
		return nil
	}
	return s.err
}

// runScript starts one child process for one target and decodes its answer.
//
// The script path resolves against .rtdd/adapters/, beside the host YAML that declared
// it, and reaches the command template as {script} — argv is built by adapter.Expand, so
// the engine never hands a shell a string.
func (s *AdapterScanner) runScript(target string) (map[string]int, error) {
	abs, err := filepath.Abs(s.repoRoot)
	if err != nil {
		return nil, fmt.Errorf("importscan: abs %q: %w", s.repoRoot, err)
	}
	script := filepath.Join(abs, ".rtdd", "adapters", filepath.FromSlash(s.ad.Importscan.Script))
	argv, err := s.ad.Expand(s.ad.Importscan.Command, map[string]string{"script": script})
	if err != nil {
		return nil, fmt.Errorf("importscan: %w", err)
	}

	payload, err := json.Marshal(request{Root: abs, Targets: []string{target}, Tests: s.tests})
	if err != nil {
		return nil, fmt.Errorf("importscan: marshal request: %w", err)
	}

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = abs
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("importscan: %s adapter's scanner %v: %w: %s",
			s.ad.Name, argv, err, bytes.TrimSpace(stderr.Bytes()))
	}

	out := map[string]map[string]int{}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("importscan: %s adapter's scanner wrote unparseable JSON: %w",
			s.ad.Name, err)
	}
	// A target no module resolves to maps to an empty result, never a missing key: the
	// caller's "level 2 found nothing here" and "level 2 could not run" must stay apart.
	if out[target] == nil {
		return map[string]int{}, nil
	}
	return out[target], nil
}
