package graph

import (
	"reflect"
	"testing"
)

// PRD #409 AC1: the model is spec §3 field for field, and the relation set is closed.
func TestNodeAndEdgeAreSpecSection3FieldForField(t *testing.T) {
	fields := func(v any) []string {
		var out []string
		rt := reflect.TypeOf(v)
		for i := 0; i < rt.NumField(); i++ {
			out = append(out, rt.Field(i).Name)
		}
		return out
	}
	if got, want := fields(Node{}), []string{"ID", "File", "Name", "Kind", "Start", "End", "IsTest"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Node fields = %v, want %v", got, want)
	}
	if got, want := fields(Edge{}), []string{"From", "To", "Relation"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Edge fields = %v, want %v", got, want)
	}
	if got, want := []Kind{KindFunc, KindMethod, KindClass, KindTest}, []Kind{"func", "method", "class", "test"}; !reflect.DeepEqual(got, want) {
		t.Errorf("kinds = %v, want %v", got, want)
	}
}

func TestRelationSetIsClosed(t *testing.T) {
	for _, s := range []string{"calls", "method", "inherits", "implements", "references"} {
		if r, ok := ParseRelation(s); !ok || string(r) != s {
			t.Errorf("ParseRelation(%q) = %q, %v; want it kept", s, r, ok)
		}
	}
	for _, s := range []string{"contains", "conceptually_related_to", "semantically_similar_to",
		"shares_data_with", "cites", "imports", "defines", "", "Calls"} {
		if r, ok := ParseRelation(s); ok {
			t.Errorf("ParseRelation(%q) = %q, true; the relation set is closed", s, r)
		}
	}
	if len(Relations) != 5 {
		t.Errorf("Relations has %d members, want the 5 of spec §3", len(Relations))
	}
}
