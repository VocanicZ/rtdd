package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// swarmTestsPlaceholder is what --run names the shard's test file by.
const swarmTestsPlaceholder = "{tests}"

// swarmFiles is the selection a swarm job shards: Round 1's test files, then Round 2's,
// each file once, in test order — the same files `which --json` lists per round.
func swarmFiles(round1, round2 []graph.Node) []string {
	var files []string
	seen := map[string]bool{}
	for _, t := range append(append([]graph.Node{}, round1...), round2...) {
		if !seen[t.File] {
			seen[t.File] = true
			files = append(files, t.File)
		}
	}
	return files
}

// swarmShardIDs gives each path a swarm-safe id ([A-Za-z0-9._-]+). The position prefix
// keeps ids unique when two paths sanitise to the same text, and keeps split order
// readable; the rest is the path with every unsafe byte as '_', for whoever reads a log.
func swarmShardIDs(paths []string) []string {
	width := len(fmt.Sprint(len(paths)))
	ids := make([]string, len(paths))
	for i, p := range paths {
		var b strings.Builder
		for _, c := range []byte(p) {
			switch {
			case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '.', c == '_', c == '-':
				b.WriteByte(c)
			default:
				b.WriteByte('_')
			}
		}
		s := b.String()
		if len(s) > 80 {
			s = s[len(s)-80:]
		}
		ids[i] = fmt.Sprintf("%0*d-%s", width, i+1, s)
	}
	return ids
}

// swarmJobName is a name the swarm contract accepts, stable for one selection and command:
// resubmitting the same job replaces it, a different selection is a different job.
func swarmJobName(runCmd string, files []string) string {
	h := sha256.Sum256([]byte(runCmd + "\x00" + strings.Join(files, "\x00")))
	return "rtdd-" + hex.EncodeToString(h[:])[:12]
}

// swarmJobDir is --swarm-job as an absolute, symlink-resolved path inside root, or an
// error: the job's workdir is the repository, and run reaches the id→path map through it
// by a relative path, so the map has to ship inside it.
func swarmJobDir(root, dir string) (string, error) {
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	// The dir may not exist yet: resolve its deepest existing ancestor, keep the rest.
	rest := ""
	for {
		if real, err := filepath.EvalSymlinks(abs); err == nil {
			abs = filepath.Join(real, rest)
			break
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			break
		}
		rest = filepath.Join(filepath.Base(abs), rest)
		abs = parent
	}
	rel, err := filepath.Rel(realRoot, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("--swarm-job %s is outside the repository %s: the job ships the repository as its workdir, so its dir must be inside it", dir, realRoot)
	}
	return abs, nil
}

// writeSwarmJob writes a swarm job (github.com/VocanicZ/swarm, skill/SKILL.md) for the
// selected test files into jobDir, and returns its name. It writes files and runs
// nothing (spec §1). root and jobDir are swarmJobDir's: resolved, jobDir inside root.
func writeSwarmJob(root, jobDir, runCmd string, files []string) (string, error) {
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, jobDir)
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(rel)
	workdir, _ := filepath.Rel(jobDir, root)
	workdir = filepath.ToSlash(workdir)

	ids := swarmShardIDs(files)
	var shardMap strings.Builder
	for i, f := range files {
		if strings.ContainsAny(f, "\t\n") {
			return "", fmt.Errorf("test file %q holds a tab or newline, which the shard map cannot carry", f)
		}
		fmt.Fprintf(&shardMap, "%s\t%s\n", ids[i], f)
	}
	name := swarmJobName(runCmd, files)
	shell := strings.ReplaceAll(runCmd, swarmTestsPlaceholder, `"$tests"`)
	toml := "# Written by `rtdd which --swarm-job`. Submit with: swarm submit <this dir>\n" +
		"name = " + tomlString(name) + "\n" +
		"workdir = " + tomlString(workdir) + "\n" +
		"split = " + tomlString("cut -f1 shards.tsv") + "\n" +
		"run = " + tomlString("sh "+shQuote(rel+"/run.sh")+" {shard} {out}") + "\n" +
		"merge = " + tomlString("sh ./merge.sh {outs}") + "\n"

	if err := os.MkdirAll(jobDir, 0o755); err != nil {
		return "", err
	}
	for _, f := range []struct{ name, body string }{
		{"shards.tsv", shardMap.String()},
		{"run.sh", strings.ReplaceAll(strings.ReplaceAll(swarmRunScript, "@MAP@", shQuote(rel+"/shards.tsv")), "@RUN@", shell)},
		{"merge.sh", swarmMergeScript},
		{"swarm.toml", toml},
	} {
		if err := os.WriteFile(filepath.Join(jobDir, f.name), []byte(f.body), 0o644); err != nil {
			return "", err
		}
	}
	return name, nil
}

// swarmRunScript runs one shard: one selected test file, under the --run command, with
// the cwd at the repository root (swarm's workdir). Its only state is $out, and a test
// failure is a result, not a retry: the exit status is recorded and the shard exits 0.
const swarmRunScript = `#!/bin/sh
# Written by ` + "`rtdd which --swarm-job`" + `: one shard runs one selected test file.
# $1 is the shard id, $2 its out dir; the cwd is the repository root.
shard=$1
out=$2
mkdir -p "$out" || exit 1
[ -f "$out/status" ] && exit 0
tests=$(awk -F '\t' -v id="$shard" '$1 == id { print $2; exit }' @MAP@)
if [ -z "$tests" ]; then
  echo "rtdd swarm job: no test file for shard $shard" >&2
  exit 1
fi
( @RUN@ ) > "$out/result.tmp" 2>&1
code=$?
mv "$out/result.tmp" "$out/result.txt"
printf '%s\t%s\n' "$code" "$tests" > "$out/status.tmp" && mv "$out/status.tmp" "$out/status"
`

// swarmMergeScript concatenates every shard's result in split order into results.txt and
// prints it; it exits 1 when any shard's test command failed, so swarm says merge: failed.
const swarmMergeScript = `#!/bin/sh
# Written by ` + "`rtdd which --swarm-job`" + `: every shard's result, in split order.
failed=0
: > results.tmp
for out in "$@"; do
  if [ -f "$out/status" ]; then
    IFS=$(printf '\t') read -r code tests < "$out/status"
  else
    code=missing tests=$out
  fi
  { printf '=== %s (exit %s)\n' "$tests" "$code"; cat "$out/result.txt" 2>/dev/null; } >> results.tmp
  [ "$code" = 0 ] || failed=1
done
mv results.tmp results.txt
cat results.txt
exit $failed
`

// shQuote is s as one single-quoted sh word.
func shQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// tomlString is s as a TOML basic string: quotes and backslashes escaped, control
// characters as \uXXXX.
func tomlString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch {
		case r == '"' || r == '\\':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r < 0x20 || r == 0x7f:
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
