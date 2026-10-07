package main

import (
	"fmt"
	"os"

	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/graph"
	"github.com/VocanicZ/rtdd/internal/graphbuild"
)

// graphRoot is the repository the caller stands in. Its error is an environment error:
// the caller exits 3 on it (spec §8).
func graphRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	root, err := gitctx.RepoRoot(wd)
	if err != nil {
		return "", fmt.Errorf("not inside a git work tree, or git is unavailable: %w", err)
	}
	return root, nil
}

// buildGraph loads .rtdd/config.yaml and builds the node graph, as `rtdd graph` does.
// base is the ref changes are measured from ("" is HEAD): files changed against it are
// scanned rather than taken from graphify (spec §5 step 1). code is the exit code for a
// non-nil err: 2 for a malformed config (the user's to fix), 3 for a graph that cannot
// be built or read.
func buildGraph(root, base string) (cfg graph.Config, res *graphbuild.Result, code int, err error) {
	if cfg, err = graph.LoadConfig(root); err != nil {
		return cfg, nil, 2, err
	}
	if res, err = graphbuild.Build(root, cfg, graphbuild.Options{Base: base}); err != nil {
		return cfg, nil, 3, err
	}
	return cfg, res, 0, nil
}
