package scan

import (
	"reflect"
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

func TestInnermostNodeOwnsALineAndAClassHasMethodEdges(t *testing.T) {
	r := ScanFile("pkg/shapes.py", []byte("class Shape:\n    def area(self):\n        return measure(self)\n\n    def name(self):\n        return label()\n"))
	if got, want := r.Calls["pkg/shapes.py::Shape::area"], []string{"measure"}; !reflect.DeepEqual(got, want) {
		t.Errorf("area calls %v, want %v", got, want)
	}
	if got := r.Calls["pkg/shapes.py::Shape"]; got != nil {
		t.Errorf("the class owns no line its methods own, but calls %v", got)
	}
	want := []graph.Edge{
		{From: "pkg/shapes.py::Shape", To: "pkg/shapes.py::Shape::area", Relation: graph.RelMethod},
		{From: "pkg/shapes.py::Shape", To: "pkg/shapes.py::Shape::name", Relation: graph.RelMethod},
	}
	if !reflect.DeepEqual(r.Edges, want) {
		t.Errorf("edges = %v\nwant %v", r.Edges, want)
	}
}

func TestEverySameNamedDefinitionIsCalledButNotFromTheDefinitionLine(t *testing.T) {
	var nodes []graph.Node
	calls := map[string][]string{}
	for _, f := range []struct{ path, src string }{
		{"a.py", "def load():\n    return 1\n"},
		{"b.py", "def load():\n    return 2\n"},
		{"c.py", "def start():\n    return load()\n\n\ndef run(cb=load()):\n    return 0\n"},
	} {
		r := ScanFile(f.path, []byte(f.src))
		nodes = append(nodes, r.Nodes...)
		for k, v := range r.Calls {
			calls[k] = v
		}
	}
	want := []graph.Edge{
		{From: "c.py::start", To: "a.py::load", Relation: graph.RelCalls},
		{From: "c.py::start", To: "b.py::load", Relation: graph.RelCalls},
	}
	if got := Link(nodes, calls); !reflect.DeepEqual(got, want) {
		t.Errorf("Link = %v\nwant %v", got, want)
	}
}

// A keyword is a call statement's opener only when it stands alone: after `.` or `::` it is
// a member or path segment, so `Calc::new()` and `p.delete()` are calls that must not be
// missed (spec §4.3), while a bare `new(` or `return(` is still no call.
func TestAQualifiedNameIsACallEvenWhenItIsAKeyword(t *testing.T) {
	r := ScanFile("lib.rs", []byte("fn make() {\n    let c = Calc::new();\n    store.delete(c);\n    return(c);\n}\n"))
	if got, want := r.Calls["lib.rs::make"], []string{"delete", "new"}; !reflect.DeepEqual(got, want) {
		t.Errorf("make calls %v, want %v", got, want)
	}
}
