package scan

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// fixtureLangs is spec §4.4's corpus. Each directory under testdata/ is one small project
// and its want.json; all eight languages of spec §1.
var fixtureLangs = []string{"python", "go", "typescript", "java", "rust", "ruby", "lua", "bash"}

type wantNode struct {
	ID    string     `json:"id"`
	Name  string     `json:"name"`
	Kind  graph.Kind `json:"kind"`
	Start int        `json:"start"`
	End   int        `json:"end"`
}

type wantCall struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type want struct {
	Nodes []wantNode `json:"nodes"`
	Calls []wantCall `json:"calls"`
}

// PRD #409 AC3 / spec §4.4: the scanner's nodes and calls edges for each fixture project
// are EXACTLY want.json — nothing missing, nothing extra. No toolchain is invoked.
func TestScannerFixturesExact(t *testing.T) {
	for _, lang := range fixtureLangs {
		t.Run(lang, func(t *testing.T) {
			dir := filepath.Join("testdata", lang)
			b, err := os.ReadFile(filepath.Join(dir, "want.json"))
			if err != nil {
				t.Fatal(err)
			}
			var w want
			if err := json.Unmarshal(b, &w); err != nil {
				t.Fatal(err)
			}

			var files []string
			err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() || d.Name() == "want.json" {
					return err
				}
				rel, _ := filepath.Rel(dir, p)
				files = append(files, filepath.ToSlash(rel))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			g := Assemble(ScanFiles(dir, files))

			var gotNodes []wantNode
			for _, n := range g.Nodes {
				gotNodes = append(gotNodes, wantNode{n.ID, n.Name, n.Kind, n.Start, n.End})
			}
			var gotCalls []wantCall
			for _, e := range g.Edges {
				if e.Relation == graph.RelCalls {
					gotCalls = append(gotCalls, wantCall{e.From, e.To})
				}
			}
			if !reflect.DeepEqual(gotNodes, w.Nodes) {
				t.Errorf("nodes:\n got %+v\nwant %+v", gotNodes, w.Nodes)
			}
			if !reflect.DeepEqual(gotCalls, w.Calls) {
				t.Errorf("calls:\n got %+v\nwant %+v", gotCalls, w.Calls)
			}
		})
	}
}
