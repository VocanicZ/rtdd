// Command rtdd reports which tests cover the code that just changed.
// This package is the only place in the engine that prints.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
)

const usage = `rtdd - relational test-driven development

usage:
  rtdd init
  rtdd seed
  rtdd run    [--base <ref>] [--fail-fast] [--json]
  rtdd status [--adapter <path>]
  rtdd which  [--base <ref>] [--json] [--adapter <path>]
  rtdd explain <file>
  rtdd doctor [--limit <n>]
  rtdd --version

exit codes:
  0  success - an empty selection is a signal, not a failure
  1  a test failed
  2  usage or configuration error
  3  fatal environment error (git unavailable, unreadable coverage)
`

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

// run is main's whole body, parameterised on its writers so every command is testable
// with in-memory buffers and an asserted exit code.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "init":
		return cmdInit(args[1:], stdout, stderr)
	case "seed":
		return cmdSeed(args[1:])
	case "run":
		return cmdRun(args[1:])
	case "status":
		return cmdStatus(args[1:], stdout, stderr)
	case "which":
		return cmdWhich(args[1:], stdout, stderr)
	case "explain":
		return cmdExplain(args[1:], stdout, stderr)
	case "doctor":
		return cmdDoctor(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
	case "--version", "version":
		return cmdVersion(stdout)
	default:
		fmt.Fprintf(stderr, "rtdd: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

// env is everything a command needs from the host repository: where it is, and what
// .rtdd/ currently says about it.
type env struct {
	root     string
	mapPath  string
	metaPath string
	adPath   string
	m        *mapstore.Map
	meta     mapstore.Meta
	ad       *adapter.Adapter // nil only when no file was present AND detection resolved nothing
	// adDetected records that ad came from detection rather than from adPath, so every
	// message about the adapter names where it actually came from.
	adDetected bool
	// adErr is why detection resolved no adapter. It is the reason the missing-adapter
	// warning states, so a repo with no recognisable toolchain still says so.
	adErr error
}

// adapterSource names where the loaded adapter came from, for human output.
func (e *env) adapterSource() string {
	if e.adDetected {
		return "detected"
	}
	return e.adPath
}

// noAdapterReason explains an absent adapter: either detection ran and found nothing, or
// no detection was attempted because the configured file is what was missing.
func (e *env) noAdapterReason() string {
	if e.adErr != nil {
		return e.adErr.Error()
	}
	return fmt.Sprintf("%s not found", e.adPath)
}

// loadEnv resolves the repo root and loads .rtdd/. The returned int is the process exit
// code to use when err is non-nil.
//
// An explicit --adapter path is an OVERRIDE, not a hint: a path the caller named and that
// does not exist is a configuration error, never a silent fall back to detection.
func loadEnv(adapterPath string, warn io.Writer) (*env, int, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, 3, err
	}
	root, err := gitctx.RepoRoot(wd)
	if err != nil {
		return nil, 3, fmt.Errorf("not inside a git work tree, or git is unavailable: %w", err)
	}

	explicit := adapterPath != ""
	e := &env{
		root:     root,
		mapPath:  mapPath(root),
		metaPath: metaPath(root),
		adPath:   adapterPath,
	}
	if e.adPath == "" {
		e.adPath = filepath.Join(".rtdd", "adapter.yaml")
	}

	// Duplicate `t` lines left by a union merge are resolved with real commit ages.
	if e.m, err = mapstore.LoadWith(e.mapPath, gitctx.Older(root)); err != nil {
		return nil, 2, err
	}
	if e.meta, err = mapstore.LoadMeta(e.metaPath); err != nil {
		return nil, 2, err
	}

	abs := e.adPath
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, abs)
	}
	// Detection replaces the file default (docs/plans/00-interfaces.md:912): `rtdd init`
	// writes no .rtdd/adapter.yaml, so on the documented setup path the file is absent and
	// only detection can answer. Without this fallback these commands classified nothing
	// while `rtdd run` and `rtdd seed`, which call detectAdapter directly, classified the
	// same repo as python — the advisory command and the executing command disagreeing
	// about one tree.
	switch _, statErr := os.Stat(abs); {
	case statErr == nil:
		if e.ad, err = adapter.Load(abs); err != nil {
			return nil, 2, err
		}
	case explicit:
		return nil, 2, fmt.Errorf("--adapter %s: %w", adapterPath, statErr)
	default:
		if ad, derr := detectAdapter(root, warn); derr != nil {
			e.adErr = derr
		} else {
			e.ad, e.adDetected = ad, true
		}
	}
	return e, 0, nil
}
