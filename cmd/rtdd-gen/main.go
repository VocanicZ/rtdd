// Command rtdd-gen renders protocol/PROTOCOL.md into every agent front-end and
// checks the committed dist/ tree against that render.
//
// Two checks exist because they catch different failures. check is a byte
// comparison: it catches drift, a dist/ file that no longer matches what
// PROTOCOL.md would produce. verify runs each target's Validate against the
// file on disk: it catches content that is wrong for a target even when it is
// not stale — a byte comparison alone cannot tell that AGENTS.md swallowed the
// skill body, or that a .mdc file lost its frontmatter.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/VocanicZ/rtdd/internal/protocol"
)

const usage = `rtdd-gen - generate rtdd's agent front-ends from protocol/PROTOCOL.md

usage:
  rtdd-gen render [--out <dir>] [--flat]
  rtdd-gen check
  rtdd-gen verify

exit codes:
  0  success
  1  check found drift, or verify found an invalid file
  2  usage or configuration error
`

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "render":
		return cmdRender(args[1:], stdout, stderr)
	case "check":
		return cmdCheck(args[1:], stdout, stderr)
	case "verify":
		return cmdVerify(args[1:], stdout, stderr)
	default:
		fmt.Fprint(stderr, usage)
		return 2
	}
}

// findRepoRoot walks up from start looking for a .git entry.
//
// It tests for the entry's existence, not for it being a directory: in a git
// worktree .git is a FILE holding a `gitdir:` pointer, and a directory-only
// check reports "not inside a git repository" there.
func findRepoRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", start, err)
	}
	for {
		if _, err := os.Lstat(filepath.Join(dir, ".git")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not inside a git repository (searched upward from %s)", start)
		}
		dir = parent
	}
}

// loadDoc finds the repo root and parses its protocol/PROTOCOL.md.
func loadDoc() (root string, doc *protocol.Doc, err error) {
	root, err = findRepoRoot(".")
	if err != nil {
		return "", nil, err
	}
	src, err := os.ReadFile(filepath.Join(root, "protocol", "PROTOCOL.md"))
	if err != nil {
		return "", nil, fmt.Errorf("reading protocol/PROTOCOL.md: %w", err)
	}
	doc, err = protocol.Parse(string(src))
	if err != nil {
		return "", nil, fmt.Errorf("parsing protocol/PROTOCOL.md: %w", err)
	}
	return root, doc, nil
}

func cmdRender(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("render", flag.ContinueOnError)
	fs.SetOutput(stderr)
	outFlag := fs.String("out", "", "directory to write generated files under (default: repo root)")
	flat := fs.Bool("flat", false, "write every target's basename into --out directly, ignoring its nested path")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "usage: rtdd-gen render [--out <dir>] [--flat]")
		return 2
	}

	root, doc, err := loadDoc()
	if err != nil {
		fmt.Fprintf(stderr, "rtdd-gen render: %v\n", err)
		return 2
	}
	outDir := root
	if *outFlag != "" {
		outDir = *outFlag
	}

	rendered, err := protocol.RenderAll(doc)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd-gen render: %v\n", err)
		return 2
	}

	for _, tgt := range protocol.Targets {
		rel := tgt.OutPath
		if *flat {
			rel = filepath.Base(tgt.OutPath)
		}
		dest := filepath.Join(outDir, rel)
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			fmt.Fprintf(stderr, "rtdd-gen render: %v\n", err)
			return 2
		}
		if err := os.WriteFile(dest, []byte(rendered[tgt.OutPath]), 0o644); err != nil {
			fmt.Fprintf(stderr, "rtdd-gen render: %v\n", err)
			return 2
		}
		fmt.Fprintf(stdout, "wrote %s\n", dest)
	}
	return 0
}

// diffHint reports the first line at which old and fresh disagree, so a
// human staring at a stale-file report has somewhere to start looking
// without rtdd-gen shipping a full diff implementation.
func diffHint(old, fresh string) string {
	oldLines := strings.Split(old, "\n")
	freshLines := strings.Split(fresh, "\n")
	n := len(oldLines)
	if len(freshLines) < n {
		n = len(freshLines)
	}
	for i := 0; i < n; i++ {
		if oldLines[i] != freshLines[i] {
			return fmt.Sprintf("first differs at line %d:\n  - %s\n  + %s", i+1, oldLines[i], freshLines[i])
		}
	}
	return fmt.Sprintf("on disk has %d lines, fresh render has %d lines", len(oldLines), len(freshLines))
}

func cmdCheck(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "usage: rtdd-gen check")
		return 2
	}
	root, doc, err := loadDoc()
	if err != nil {
		fmt.Fprintf(stderr, "rtdd-gen check: %v\n", err)
		return 2
	}
	rendered, err := protocol.RenderAll(doc)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd-gen check: %v\n", err)
		return 2
	}

	stale := 0
	for _, tgt := range protocol.Targets {
		onDisk, err := os.ReadFile(filepath.Join(root, tgt.OutPath))
		if err != nil {
			fmt.Fprintf(stdout, "%s: stale (missing: %v)\n", tgt.OutPath, err)
			stale++
			continue
		}
		if string(onDisk) != rendered[tgt.OutPath] {
			fmt.Fprintf(stdout, "%s: stale (%s)\n", tgt.OutPath, diffHint(string(onDisk), rendered[tgt.OutPath]))
			stale++
		}
	}
	if stale > 0 {
		fmt.Fprintf(stdout, "%d file(s) stale; run `rtdd-gen render` and commit the result\n", stale)
		return 1
	}
	fmt.Fprintln(stdout, "dist/ is up to date with protocol/PROTOCOL.md")
	return 0
}

func cmdVerify(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "usage: rtdd-gen verify")
		return 2
	}
	root, doc, err := loadDoc()
	if err != nil {
		fmt.Fprintf(stderr, "rtdd-gen verify: %v\n", err)
		return 2
	}

	invalid := 0
	for _, tgt := range protocol.Targets {
		onDisk, err := os.ReadFile(filepath.Join(root, tgt.OutPath))
		if err != nil {
			fmt.Fprintf(stdout, "%s: target %q: %v\n", tgt.OutPath, tgt.Name, err)
			invalid++
			continue
		}
		if err := tgt.Validate(doc, tgt, string(onDisk)); err != nil {
			fmt.Fprintf(stdout, "%s: target %q: %v\n", tgt.OutPath, tgt.Name, err)
			invalid++
		}
	}
	if invalid > 0 {
		fmt.Fprintf(stdout, "%d file(s) failed validation\n", invalid)
		return 1
	}
	fmt.Fprintln(stdout, "dist/ passes every target's validity assertions")
	return 0
}
