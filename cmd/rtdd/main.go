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
  rtdd seed
  rtdd status [--adapter <path>]
  rtdd which  [--base <ref>] [--json] [--adapter <path>]

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
	case "seed":
		return cmdSeed(args[1:])
	case "status":
		return cmdStatus(args[1:], stdout, stderr)
	case "which":
		return cmdWhich(args[1:], stdout, stderr)
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return 0
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
	ad       *adapter.Adapter // nil when no adapter file is present
}

// loadEnv resolves the repo root and loads .rtdd/. The returned int is the process exit
// code to use when err is non-nil.
func loadEnv(adapterPath string) (*env, int, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, 3, err
	}
	root, err := gitctx.RepoRoot(wd)
	if err != nil {
		return nil, 3, fmt.Errorf("not inside a git work tree, or git is unavailable: %w", err)
	}

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
	if _, statErr := os.Stat(abs); statErr == nil {
		if e.ad, err = adapter.Load(abs); err != nil {
			return nil, 2, err
		}
	}
	return e, 0, nil
}
