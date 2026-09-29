package runner

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/coverage"
	"github.com/VocanicZ/rtdd/internal/covfmt"
	"github.com/VocanicZ/rtdd/internal/gitctx"
)

// Units is every test file the adapter claims, tracked or not (spec §4.1).
func Units(a *adapter.Adapter, repoRoot string) ([]string, error) {
	files, err := gitctx.ListFiles(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("runner: enumerate units: %w", err)
	}
	var out []string
	for _, f := range files {
		if a.IsTestFile(f) {
			out = append(out, f)
		}
	}
	return out, nil
}

type unitResult struct {
	outcome Outcome
	files   map[string][]int
	output  string
	fatal   error
}

// RunUnits runs each unit in its own process, up to Jobs at once (spec §4.2–4.5).
func RunUnits(a *adapter.Adapter, repoRoot string, units []string, failFast bool) (*RunResult, error) {
	repoFiles, err := gitctx.ListFiles(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("runner: %w", err)
	}
	jobs := a.Jobs
	if jobs <= 0 {
		jobs = runtime.NumCPU()
	}
	results := make([]unitResult, len(units))
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		stopped bool
	)
	sem := make(chan struct{}, jobs)
	for i, u := range units {
		sem <- struct{}{}
		mu.Lock()
		stop := stopped
		mu.Unlock()
		if stop {
			<-sem
			break
		}
		wg.Add(1)
		go func(i int, u string) {
			defer wg.Done()
			defer func() { <-sem }()
			r := runUnit(a, repoRoot, u, repoFiles)
			results[i] = r
			if r.fatal != nil || (failFast && (r.outcome.Status == "fail" || r.outcome.Status == "error")) {
				mu.Lock()
				stopped = true
				mu.Unlock()
			}
		}(i, u)
	}
	wg.Wait()

	res := &RunResult{Coverage: &coverage.Result{}, Output: map[string]string{}}
	for _, r := range results {
		if r.outcome.Test == "" {
			continue // never scheduled (fail-fast)
		}
		if r.fatal != nil {
			return nil, r.fatal
		}
		res.Outcomes = append(res.Outcomes, r.outcome)
		if r.files != nil {
			res.Coverage.PerTest = append(res.Coverage.PerTest, coverage.TestCoverage{Test: r.outcome.Test, Files: r.files})
		}
		switch r.outcome.Status {
		case "fail", "error":
			res.Failed = append(res.Failed, r.outcome.Test)
			res.Output[r.outcome.Test] = r.output
			res.ExitCode = 1
		}
	}
	sort.Slice(res.Coverage.PerTest, func(i, j int) bool { return res.Coverage.PerTest[i].Test < res.Coverage.PerTest[j].Test })
	return res, nil
}

func runUnit(a *adapter.Adapter, repoRoot, unit string, repoFiles []string) unitResult {
	r := unitResult{outcome: Outcome{Test: unit}}
	body, err := os.ReadFile(filepath.Join(repoRoot, filepath.FromSlash(unit)))
	if err != nil {
		r.outcome.Status, r.output = "error", err.Error()
		return r
	}
	names, ok := a.UnitNamesOf(body)
	if !ok {
		r.outcome.Status = "skip" // no runnable test in this file
		return r
	}
	tmp, err := os.MkdirTemp("", "rtdd-unit-")
	if err != nil {
		r.fatal = fmt.Errorf("runner: %w", err)
		return r
	}
	defer os.RemoveAll(tmp)
	for k, v := range a.UnitFileContents(tmp) {
		f := filepath.Join(tmp, filepath.FromSlash(k))
		var err error
		if rel, rerr := filepath.Rel(tmp, f); rerr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			err = fmt.Errorf("resolves outside the unit tmp dir")
		} else {
			err = os.MkdirAll(filepath.Dir(f), 0o755)
		}
		if err == nil {
			err = os.WriteFile(f, []byte(v), 0o644)
		}
		if err != nil {
			r.outcome.Status, r.output = "error", fmt.Sprintf("unit_files %q: %v", k, err)
			return r
		}
	}
	argv, err := a.UnitArgv(unit, tmp, names)
	if err != nil {
		r.fatal = err
		return r
	}

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = repoRoot
	cmd.Env = mergeEnv(os.Environ(), a.UnitEnv(tmp))
	start := time.Now()
	out, runErr := cmd.CombinedOutput()
	r.outcome.DurationMS = int(time.Since(start).Milliseconds())
	r.output = tail(out, 4000)

	code := 0
	if runErr != nil {
		var ee *exec.ExitError
		if !errors.As(runErr, &ee) {
			r.fatal = fmt.Errorf("runner: executing %s for %s: %w", argv[0], unit, runErr)
			return r
		}
		code = ee.ExitCode()
	}
	switch label, mapped := a.ExitCodes[code]; {
	case code == 0:
		r.outcome.Status = "pass"
	case code == 1:
		r.outcome.Status = "fail"
	case mapped && label == "no-tests-collected":
		r.outcome.Status = "skip"
		return r
	case mapped:
		fe := &FatalExitError{Unit: unit, Code: code, Label: label, Output: r.output}
		for _, req := range a.Requires {
			fe.Requires = append(fe.Requires, req.Reason)
		}
		r.fatal = fe
		return r
	default:
		r.outcome.Status = "error"
		r.output = fmt.Sprintf("exit %d\n%s", code, r.output)
		return r
	}

	f, err := os.Open(a.CoveragePath(tmp))
	if err != nil {
		// A run that says it passed but recorded nothing is not a pass (spec §4.4).
		r.outcome.Status = "error"
		r.output = fmt.Sprintf("no coverage file at %s: %v\n%s", a.CoverageFile, err, r.output)
		return r
	}
	defer f.Close()
	raw, err := covfmt.Parse(a.CoverageFormat, f)
	if err != nil {
		r.outcome.Status = "error"
		r.output = err.Error() + "\n" + r.output
		return r
	}
	kept := map[string][]int{}
	for p, ls := range covfmt.Resolve(repoRoot, raw, repoFiles) {
		if a.IsInstrumentable(p) || a.IsTestFile(p) {
			kept[p] = ls
		}
	}
	r.files = kept
	return r
}
