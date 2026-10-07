package scan

import (
	"reflect"
	"testing"

	"github.com/VocanicZ/rtdd/internal/graph"
)

// span is a node as the shape tests pin it.
type span struct {
	ID         string
	Kind       graph.Kind
	Start, End int
}

func spans(rel, src string) []span {
	var out []span
	for _, n := range ScanFile(rel, []byte(src)).Nodes {
		out = append(out, span{n.ID, n.Kind, n.Start, n.End})
	}
	return out
}

func checkSpans(t *testing.T, rel, src string, want []span) {
	t.Helper()
	if got := spans(rel, src); !reflect.DeepEqual(got, want) {
		t.Errorf("%s nodes:\n got %+v\nwant %+v", rel, got, want)
	}
}
