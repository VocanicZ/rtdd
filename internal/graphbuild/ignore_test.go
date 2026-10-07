package graphbuild

import (
	"reflect"
	"testing"

	"github.com/VocanicZ/rtdd/internal/gitctx/gittest"
	"github.com/VocanicZ/rtdd/internal/graph"
)

var fourFiles = map[string]string{
	"a.py": "def a():\n    return 1\n", "b.py": "def b():\n    return 2\n",
	"c.py": "def c():\n    return 3\n", "d.py": "def d():\n    return 4\n",
}

// fourGraphify records one function per file of fourFiles at builtAt.
func fourGraphify(t *testing.T, root, builtAt string) {
	t.Helper()
	all := []string{"a.py", "b.py", "c.py", "d.py"}
	var nodes []gfNode
	for i, f := range all {
		nodes = append(nodes, gfNode{f, string(rune('a'+i)) + "()", f, 1})
	}
	writeGraphify(t, root, builtAt, nodes, nil, all)
}

// edit rewrites files of fourFiles after graphify was built, uncommitted, each gaining a node.
func edit(t *testing.T, root string, files ...string) {
	t.Helper()
	for _, f := range files {
		gittest.Write(t, root, f, fourFiles[f]+"\n\ndef extra():\n    return 0\n")
	}
}

// PRD #409 AC6: each trigger alone makes graphify ignored, with its own reason, and the
// scanner builds the whole graph.
func TestGraphifyIsIgnoredWithAReason(t *testing.T) {
	cases := []struct {
		name    string
		builtAt func(t *testing.T, root string) string
		edits   []string
		want    string
	}{
		{"built_at_commit missing", func(*testing.T, string) string { return "" }, nil, IgnoredNoCommit},
		{"built_at_commit unknown to git", func(*testing.T, string) string { return "0123456789abcdef0123456789abcdef01234567" }, nil, IgnoredUnknownCommit},
		{"more than half stale", fullHead, []string{"a.py", "b.py", "c.py"}, IgnoredTooStale},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := repo(t, fourFiles)
			fourGraphify(t, root, c.builtAt(t, root))
			edit(t, root, c.edits...)
			res := build(t, root)
			if res.Source != SourceScanner || res.GraphifyIgnored != c.want {
				t.Errorf("Source %q, GraphifyIgnored %q; want scanner, %q", res.Source, res.GraphifyIgnored, c.want)
			}
			if len(res.Graph.Nodes) != 4+len(c.edits) {
				t.Errorf("%d nodes, want the scanner's whole graph (%d)", len(res.Graph.Nodes), 4+len(c.edits))
			}
			if !reflect.DeepEqual(res.Scanned, []string{"a.py", "b.py", "c.py", "d.py"}) {
				t.Errorf("Scanned = %v, want every file", res.Scanned)
			}
		})
	}
}

// The too_stale reason reports the stale set it measured, so the output can say how stale.
func TestTooStaleKeepsTheStaleSetItMeasured(t *testing.T) {
	root := repo(t, fourFiles)
	fourGraphify(t, root, fullHead(t, root))
	edit(t, root, "a.py", "b.py", "c.py")
	res := build(t, root)
	if !reflect.DeepEqual(res.StaleFiles, []string{"a.py", "b.py", "c.py"}) || res.GraphifyFiles != 4 {
		t.Errorf("StaleFiles %v of GraphifyFiles %d, want a.py, b.py, c.py of 4", res.StaleFiles, res.GraphifyFiles)
	}
}

// "Exceeds 50 %" (spec §5): exactly half stale keeps graphify.
func TestExactlyHalfStaleKeepsGraphify(t *testing.T) {
	root := repo(t, fourFiles)
	fourGraphify(t, root, fullHead(t, root))
	edit(t, root, "a.py", "b.py")
	res := build(t, root)
	if res.Source != SourceGraphifyScanner || res.GraphifyIgnored != "" || !reflect.DeepEqual(res.StaleFiles, []string{"a.py", "b.py"}) {
		t.Errorf("Source %q, GraphifyIgnored %q, StaleFiles %v; want graphify+scanner with a.py, b.py stale", res.Source, res.GraphifyIgnored, res.StaleFiles)
	}
}

// A graphify graph with no code files cannot vouch for any file: always too stale.
func TestGraphifyWithNoCodeFilesIsTooStale(t *testing.T) {
	root := repo(t, fourFiles)
	writeGraphify(t, root, fullHead(t, root), nil, nil, nil)
	res := build(t, root)
	if res.Source != SourceScanner || res.GraphifyIgnored != IgnoredTooStale {
		t.Errorf("Source %q, GraphifyIgnored %q; want scanner, %q", res.Source, res.GraphifyIgnored, IgnoredTooStale)
	}
}

func TestMaxStaleRatioOverride(t *testing.T) {
	cases := []struct {
		ratio string
		want  string
	}{
		{"0.25", IgnoredTooStale}, // 2 of 4 stale exceeds 25 %
		{"1", ""},                 // 3 of 4 stale is within 100 %
	}
	for _, c := range cases {
		t.Run(c.ratio, func(t *testing.T) {
			root := repo(t, fourFiles)
			fourGraphify(t, root, fullHead(t, root))
			gittest.Write(t, root, ".rtdd/config.yaml", "max_stale_ratio: "+c.ratio+"\n")
			if c.want == "" {
				edit(t, root, "a.py", "b.py", "c.py")
			} else {
				edit(t, root, "a.py", "b.py")
			}
			cfg, err := graph.LoadConfig(root)
			if err != nil {
				t.Fatal(err)
			}
			res, err := Build(root, cfg, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if res.GraphifyIgnored != c.want {
				t.Errorf("GraphifyIgnored = %q with max_stale_ratio %s, want %q", res.GraphifyIgnored, c.ratio, c.want)
			}
		})
	}
}
