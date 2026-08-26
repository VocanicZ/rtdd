// Package importscan is RTDD's single use of static analysis (spec §6, D14).
//
// A file that appears only in coverage's empty context cannot be reached through the
// coverage relation, so selection falls back to a Python AST import scan that selects
// tests whose module transitively imports it. The engine is Go and must not parse Python
// itself, so the scan runs as an embedded Python script.
package importscan

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed scan.py
var scanScript []byte

type request struct {
	Root    string   `json:"root"`
	Targets []string `json:"targets"`
	Tests   []string `json:"tests"`
}

// Scan returns, for each target, the test files whose module transitively imports it.
// Import cycles terminate via a visited set. A target no module resolves to maps to an
// empty slice, never a missing key.
func Scan(repoRoot string, targets, tests []string) (map[string][]string, error) {
	if len(targets) == 0 {
		return map[string][]string{}, nil
	}
	bin, err := pythonBin()
	if err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("", "rtdd-importscan-")
	if err != nil {
		return nil, fmt.Errorf("importscan: temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	script := filepath.Join(dir, "scan.py")
	if err := os.WriteFile(script, scanScript, 0o600); err != nil {
		return nil, fmt.Errorf("importscan: write script: %w", err)
	}

	abs, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("importscan: abs %q: %w", repoRoot, err)
	}
	payload, err := json.Marshal(request{Root: abs, Targets: targets, Tests: tests})
	if err != nil {
		return nil, fmt.Errorf("importscan: marshal request: %w", err)
	}

	cmd := exec.Command(bin, script)
	cmd.Dir = abs
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("importscan: %s scan.py: %w: %s", bin, err, strings.TrimSpace(stderr.String()))
	}

	out := map[string][]string{}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("importscan: decode result: %w", err)
	}
	for _, t := range targets {
		if out[t] == nil {
			out[t] = []string{}
		}
	}
	return out, nil
}

func pythonBin() (string, error) {
	for _, c := range []string{"python3", "python"} {
		if p, err := exec.LookPath(c); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("importscan: no python3 or python on PATH")
}
