package main

import (
	"bufio"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
)

// swarmShardIDRule and swarmJobNameRule are the swarm.toml contract's id and name rules.
var (
	swarmShardIDRule = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	swarmJobNameRule = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
)

// parseSwarmTOML reads the flat `key = "string"` TOML rtdd writes. Anything else in the
// file fails the test: rtdd writes no tables, arrays or bare values.
func parseSwarmTOML(t *testing.T, path string) map[string]string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	keys := map[string]string{}
	for _, line := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, " = ")
		if !ok {
			t.Fatalf("swarm.toml line is not `key = \"value\"`: %q", line)
		}
		s, err := strconv.Unquote(v)
		if err != nil || !strings.HasPrefix(v, `"`) {
			t.Fatalf("swarm.toml %s is not a basic string: %q", k, v)
		}
		if _, dup := keys[k]; dup {
			t.Fatalf("swarm.toml repeats %s", k)
		}
		keys[k] = s
	}
	return keys
}

// shellRun runs one swarm command the way swarm does: placeholders filled, under sh, in dir.
func shellRun(t *testing.T, dir, cmd string, fill map[string]string) (string, error) {
	t.Helper()
	for k, v := range fill {
		cmd = strings.ReplaceAll(cmd, k, v)
	}
	c := exec.Command("sh", "-c", cmd)
	c.Dir = dir
	out, err := c.Output()
	return string(out), err
}

// readShardMap reads the id→path map rtdd writes beside swarm.toml.
func readShardMap(t *testing.T, jobDir string) map[string]string {
	t.Helper()
	f, err := os.Open(filepath.Join(jobDir, "shards.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	m := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		id, path, ok := strings.Cut(sc.Text(), "\t")
		if !ok {
			t.Fatalf("shards.tsv line has no tab: %q", sc.Text())
		}
		m[id] = path
	}
	return m
}

// selectedFiles is Round 1's files then Round 2's, de-duplicated: what `which --json` says.
func selectedFiles(t *testing.T, dir string) []string {
	t.Helper()
	code, out, errOut := rtdd(t, dir, "which", "--json")
	if code != 0 {
		t.Fatalf("rtdd which --json = %d, stderr %q", code, errOut)
	}
	var doc struct {
		Rounds []struct {
			Files []string `json:"files"`
		} `json:"rounds"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, r := range doc.Rounds {
		for _, f := range r.Files {
			if !slices.Contains(files, f) {
				files = append(files, f)
			}
		}
	}
	return files
}

// splitIDs runs the job's split in its job dir and returns the ids it prints.
func splitIDs(t *testing.T, jobDir string, toml map[string]string) []string {
	t.Helper()
	out, err := shellRun(t, jobDir, toml["split"], nil)
	if err != nil {
		t.Fatalf("split %q: %v", toml["split"], err)
	}
	return strings.Fields(out)
}

// #512 AC1, AC2: swarm.toml holds exactly the contract's keys, run carries {shard} and
// {out}, merge carries {outs}, and stdout is the job name alone.
func TestWhichSwarmJobWritesTheContractKeysAndPrintsTheName(t *testing.T) {
	dir := whichRepo(t)
	editTotalAndUnused(t, dir)
	code, out, errOut := rtdd(t, dir, "which", "--swarm-job", ".rtdd/swarm", "--run", "pytest {tests}")
	if code != 0 {
		t.Fatalf("rtdd which --swarm-job = %d, stderr %q", code, errOut)
	}
	toml := parseSwarmTOML(t, filepath.Join(dir, ".rtdd", "swarm", "swarm.toml"))
	for _, k := range []string{"name", "split", "run", "merge"} {
		if toml[k] == "" {
			t.Errorf("swarm.toml has no %s: %v", k, toml)
		}
	}
	for k := range toml {
		if !slices.Contains([]string{"name", "workdir", "split", "run", "merge"}, k) {
			t.Errorf("swarm.toml key %q is outside what rtdd writes", k)
		}
	}
	if !strings.Contains(toml["run"], "{shard}") || !strings.Contains(toml["run"], "{out}") {
		t.Errorf("run = %q, want {shard} and {out}", toml["run"])
	}
	if !strings.Contains(toml["merge"], "{outs}") {
		t.Errorf("merge = %q, want {outs}", toml["merge"])
	}
	name := toml["name"]
	if !swarmJobNameRule.MatchString(name) || strings.HasSuffix(name, ".prev") {
		t.Errorf("name %q breaks the swarm name rule", name)
	}
	if out != name+"\n" {
		t.Errorf("stdout = %q, want the job name %q alone", out, name)
	}
}

// #512 AC2: split's ids map back, through shards.tsv, to exactly the selection, in order.
func TestWhichSwarmJobSplitMapsBackToTheSelection(t *testing.T) {
	dir := whichRepo(t)
	editTotalAndUnused(t, dir)
	want := selectedFiles(t, dir)
	if len(want) < 2 {
		t.Fatalf("fixture should select two files, got %v", want)
	}
	if code, _, errOut := rtdd(t, dir, "which", "--swarm-job", ".rtdd/swarm", "--run", "pytest {tests}"); code != 0 {
		t.Fatalf("rtdd which --swarm-job = %d, stderr %q", code, errOut)
	}
	jobDir := filepath.Join(dir, ".rtdd", "swarm")
	ids := splitIDs(t, jobDir, parseSwarmTOML(t, filepath.Join(jobDir, "swarm.toml")))
	m := readShardMap(t, jobDir)
	var got []string
	for _, id := range ids {
		got = append(got, m[id])
	}
	if !slices.Equal(got, want) {
		t.Errorf("split ids map to %v, want the selection %v", got, want)
	}
}

// #512 AC2: ids are safe and unique even for test paths with /, spaces, quotes and other
// characters swarm forbids; and they still map back to exactly the selection.
func TestWhichSwarmJobIDsAreSafeForAnyPath(t *testing.T) {
	dir := gittest.Init(t)
	gittest.Write(t, dir, "src/calc.py", "def add(a, b):\n    return a + b\n")
	odd := []string{"tests/a b/test_calc.py", "tests/it's $x/test_calc.py", "tests/a_b/test_calc.py", "tests/ü/test_calc.py"}
	for _, p := range odd {
		gittest.Write(t, dir, p, "from src.calc import add\n\n\ndef test_add():\n    assert add(1, 2) == 3\n")
	}
	gittest.Commit(t, dir, "init")
	gittest.Write(t, dir, "src/calc.py", "def add(a, b):\n    return b + a\n")
	want := selectedFiles(t, dir)
	if len(want) != len(odd) {
		t.Fatalf("fixture should select all %d test files, got %v", len(odd), want)
	}
	if code, _, errOut := rtdd(t, dir, "which", "--swarm-job", ".rtdd/swarm", "--run", "cat {tests}"); code != 0 {
		t.Fatalf("rtdd which --swarm-job = %d, stderr %q", code, errOut)
	}
	jobDir := filepath.Join(dir, ".rtdd", "swarm")
	ids := splitIDs(t, jobDir, parseSwarmTOML(t, filepath.Join(jobDir, "swarm.toml")))
	m := readShardMap(t, jobDir)
	seen := map[string]bool{}
	var got []string
	for _, id := range ids {
		if !swarmShardIDRule.MatchString(id) {
			t.Errorf("shard id %q breaks [A-Za-z0-9._-]+", id)
		}
		if seen[id] {
			t.Errorf("shard id %q is not unique", id)
		}
		seen[id] = true
		got = append(got, m[id])
	}
	if !slices.Equal(got, want) {
		t.Errorf("split ids map to %v, want the selection %v", got, want)
	}
}

// #512: two paths that sanitise to the same text still get different ids.
func TestSwarmShardIDsAreUniqueWhenPathsSanitiseAlike(t *testing.T) {
	ids := swarmShardIDs([]string{"a/b.py", "a_b.py", "a b.py", "a?b.py", ""})
	seen := map[string]bool{}
	for _, id := range ids {
		if !swarmShardIDRule.MatchString(id) || seen[id] {
			t.Errorf("id %q is unsafe or repeated in %v", id, ids)
		}
		seen[id] = true
	}
	if len(ids) != 5 {
		t.Errorf("want one id per path, got %v", ids)
	}
}

// #512 AC1: run resolves {shard} to its file through the map, runs the --run command on
// it from the workdir, results land in {out}; merge concatenates the shards in order.
func TestWhichSwarmJobRunAndMergeExecuteTheSelection(t *testing.T) {
	dir := gittest.Init(t)
	gittest.Write(t, dir, "src/calc.py", "def add(a, b):\n    return a + b\n")
	gittest.Write(t, dir, "tests/test_add.py", "from src.calc import add\n\n\ndef test_add():\n    assert add(1, 2) == 3\n")
	gittest.Write(t, dir, "tests/my tests/test_more.py", "from src.calc import add\n\n\ndef test_more():\n    assert add(2, 2) == 4\n")
	gittest.Commit(t, dir, "init")
	gittest.Write(t, dir, "src/calc.py", "def add(a, b):\n    return b + a\n")
	if code, _, errOut := rtdd(t, dir, "which", "--swarm-job", ".rtdd/swarm", "--run", "grep -c def {tests} && test {tests} != tests/test_add.py"); code != 0 {
		t.Fatalf("rtdd which --swarm-job = %d, stderr %q", code, errOut)
	}
	jobDir := filepath.Join(dir, ".rtdd", "swarm")
	toml := parseSwarmTOML(t, filepath.Join(jobDir, "swarm.toml"))
	workdir := filepath.Join(jobDir, toml["workdir"])
	if real, _ := filepath.EvalSymlinks(workdir); real != mustEval(t, dir) {
		t.Fatalf("workdir %q resolves to %q, want the repository root %q", toml["workdir"], real, dir)
	}
	m := readShardMap(t, jobDir)
	var outs []string
	for _, id := range splitIDs(t, jobDir, toml) {
		out := filepath.Join(t.TempDir(), "out")
		fill := map[string]string{"{shard}": id, "{out}": out}
		if _, err := shellRun(t, workdir, toml["run"], fill); err != nil {
			t.Fatalf("run for %s (%s): %v", id, m[id], err)
		}
		b, err := os.ReadFile(filepath.Join(out, "result.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(b), "1\n") {
			t.Errorf("shard %s ran on the wrong file: result %q", id, b)
		}
		// A finished shard resumes as finished: a second start leaves {out} as it is.
		if _, err := shellRun(t, workdir, toml["run"], fill); err != nil {
			t.Fatalf("rerun for %s: %v", id, err)
		}
		outs = append(outs, out)
	}
	merged, err := shellRun(t, jobDir, toml["merge"], map[string]string{"{outs}": strings.Join(outs, " ")})
	if err == nil {
		t.Errorf("merge exited 0, but the shard for tests/test_add.py failed:\n%s", merged)
	}
	for _, f := range []string{"tests/test_add.py (exit 1)", "tests/my tests/test_more.py (exit 0)"} {
		if !strings.Contains(merged, f) {
			t.Errorf("merge output lacks %q:\n%s", f, merged)
		}
	}
	var order []string
	for _, id := range splitIDs(t, jobDir, toml) {
		order = append(order, "=== "+m[id]+" ")
	}
	if strings.Index(merged, order[0]) > strings.Index(merged, order[1]) {
		t.Errorf("merge is not in split order %v:\n%s", order, merged)
	}
	if b, err := os.ReadFile(filepath.Join(jobDir, "results.txt")); err != nil || string(b) != merged {
		t.Errorf("results.txt = %q (%v), want merge's output", b, err)
	}
}

func mustEval(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// #512 AC1: --run and --swarm-job come together; --run must name {tests}; --json does not
// mix with a job; a job dir outside the repository cannot reach the workdir.
func TestWhichSwarmJobUsageErrors(t *testing.T) {
	dir := whichRepo(t)
	editTotalAndUnused(t, dir)
	for _, c := range []struct {
		name string
		args []string
	}{
		{"--run alone", []string{"which", "--run", "pytest {tests}"}},
		{"--swarm-job alone", []string{"which", "--swarm-job", ".rtdd/swarm"}},
		{"--run without {tests}", []string{"which", "--swarm-job", ".rtdd/swarm", "--run", "pytest"}},
		{"--json with a job", []string{"which", "--json", "--swarm-job", ".rtdd/swarm", "--run", "pytest {tests}"}},
		{"job dir outside the repository", []string{"which", "--swarm-job", t.TempDir(), "--run", "pytest {tests}"}},
	} {
		code, out, errOut := rtdd(t, dir, c.args...)
		if code != 2 {
			t.Errorf("%s: exit %d, want 2", c.name, code)
		}
		if out != "" || errOut == "" {
			t.Errorf("%s: stdout %q, stderr %q; want the reason on stderr only", c.name, out, errOut)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".rtdd", "swarm")); !os.IsNotExist(err) {
		t.Errorf("a usage error wrote the job dir: %v", err)
	}
}

// #512: empty rounds write no job — there is nothing to shard, and an empty job is no pass.
func TestWhichSwarmJobWithEmptyRoundsWritesNothing(t *testing.T) {
	dir := whichRepo(t)
	gittest.Write(t, dir, "src/calc.py", "def add(a, b):\n    return a + b\n\n\ndef total(xs):\n    return add(xs[0], xs[1])\n\n\ndef unused():\n    return 2\n")
	code, out, errOut := rtdd(t, dir, "which", "--swarm-job", ".rtdd/swarm", "--run", "pytest {tests}")
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut)
	}
	if out != "" || !strings.Contains(errOut, noLinkedTest) {
		t.Errorf("stdout %q, stderr %q; want no job name and %q on stderr", out, errOut, noLinkedTest)
	}
	if _, err := os.Stat(filepath.Join(dir, ".rtdd", "swarm")); !os.IsNotExist(err) {
		t.Errorf("empty rounds wrote a job dir: %v", err)
	}
}

// #512 AC4: writing the job runs nothing but git — no test, no swarm.
func TestWhichSwarmJobSpawnsNoProcessButGit(t *testing.T) {
	dir := whichRepo(t)
	editTotalAndUnused(t, dir)
	marker := tripwirePATH(t)
	if code, _, errOut := rtdd(t, dir, "which", "--swarm-job", ".rtdd/swarm", "--run", "pytest {tests}"); code != 0 {
		t.Fatalf("rtdd which --swarm-job = %d, stderr %q", code, errOut)
	}
	if b, err := os.ReadFile(marker); err == nil {
		t.Errorf("rtdd which --swarm-job ran a tool: %s", b)
	}
}

// #512: a long path keeps its tail in the id, inside the id rule.
func TestSwarmShardIDsKeepALongPathsTail(t *testing.T) {
	long := strings.Repeat("deep/", 40) + "test_tail.py"
	id := swarmShardIDs([]string{long})[0]
	if !swarmShardIDRule.MatchString(id) || !strings.HasSuffix(id, "test_tail.py") || len(id) > 90 {
		t.Errorf("id %q for a %d-byte path: want a short, safe id ending in its file name", id, len(long))
	}
}

// #512: every string rtdd writes into swarm.toml reads back as itself.
func TestTOMLStringRoundTrips(t *testing.T) {
	for _, s := range []string{`plain`, `say "hi"`, `back\slash`, "tab\there", "nl\nx", "del\x7f", "ü"} {
		got, err := strconv.Unquote(tomlString(s))
		if err != nil || got != s {
			t.Errorf("tomlString(%q) = %s reads back as %q (%v)", s, tomlString(s), got, err)
		}
	}
}

// #512: a test path the tab-separated map cannot carry is an error, not a corrupt map.
func TestWriteSwarmJobRefusesAPathWithATab(t *testing.T) {
	root := t.TempDir()
	if _, err := writeSwarmJob(root, filepath.Join(root, "job"), "cat {tests}", []string{"tests/a\tb.py"}); err == nil {
		t.Error("a path with a tab was written into shards.tsv")
	}
}

// #512: a job written for a change that deletes a file still warns that Round 3 runs its tests.
func TestWhichSwarmJobKeepsTheDeletedFileWarning(t *testing.T) {
	dir := whichRepo(t)
	editTotalAndUnused(t, dir)
	if err := os.Remove(filepath.Join(dir, "src", "report.py")); err != nil {
		t.Fatal(err)
	}
	code, _, errOut := rtdd(t, dir, "which", "--swarm-job", ".rtdd/swarm", "--run", "pytest {tests}")
	if code != 0 || !strings.Contains(errOut, "warning: src/report.py was deleted") {
		t.Errorf("exit %d, stderr %q; want the deleted-file warning", code, errOut)
	}
}
