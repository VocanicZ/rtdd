package protocol

import "testing"

const sample = `# title

<!-- rtdd:meta version=1 -->

<!-- rtdd:section id=b title="Second" targets=skill,agents order=20 -->
long body for b
<!-- rtdd:variant target=agents -->
short b
<!-- rtdd:endvariant -->
<!-- rtdd:endsection -->

<!-- rtdd:section id=a title="First" targets=skill order=10 -->
body for a
<!-- rtdd:endsection -->
`

func TestParseReadsMetaAndSections(t *testing.T) {
	d, err := Parse(sample)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if d.Version != 1 {
		t.Fatalf("Version = %d, want 1", d.Version)
	}
	if len(d.Sections) != 2 {
		t.Fatalf("len(Sections) = %d, want 2", len(d.Sections))
	}
}

func TestForOrdersByOrderNotFilePosition(t *testing.T) {
	d, _ := Parse(sample)
	got := d.For("skill")
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("For(skill) = %v, want [a b]", ids(got))
	}
}

func TestForFiltersByTarget(t *testing.T) {
	d, _ := Parse(sample)
	if got := ids(d.For("agents")); len(got) != 1 || got[0] != "b" {
		t.Fatalf("For(agents) = %v, want [b]", got)
	}
	if got := ids(d.For("mdc")); len(got) != 0 {
		t.Fatalf("For(mdc) = %v, want []", got)
	}
}

func TestBodyForPrefersTheVariant(t *testing.T) {
	d, _ := Parse(sample)
	b := d.For("skill")[1]
	if b.BodyFor("skill") != "long body for b" {
		t.Fatalf("skill body = %q", b.BodyFor("skill"))
	}
	if b.BodyFor("agents") != "short b" {
		t.Fatalf("agents body = %q", b.BodyFor("agents"))
	}
}

func TestTitleIsRequired(t *testing.T) {
	_, err := Parse("<!-- rtdd:meta version=1 -->\n<!-- rtdd:section id=x targets=skill order=1 -->\nb\n<!-- rtdd:endsection -->\n")
	if err == nil {
		t.Fatal("want error for a section with no title")
	}
}

func TestMissingMetaIsAnError(t *testing.T) {
	_, err := Parse("<!-- rtdd:section id=x title=\"X\" targets=skill order=1 -->\nb\n<!-- rtdd:endsection -->\n")
	if err == nil {
		t.Fatal("want error for a doc with no rtdd:meta")
	}
}

func TestUnterminatedSectionIsAnError(t *testing.T) {
	_, err := Parse("<!-- rtdd:meta version=1 -->\n<!-- rtdd:section id=x title=\"X\" targets=skill order=1 -->\nbody\n")
	if err == nil {
		t.Fatal("want error for an unterminated section")
	}
}

func TestUnterminatedVariantIsAnError(t *testing.T) {
	src := "<!-- rtdd:meta version=1 -->\n" +
		"<!-- rtdd:section id=x title=\"X\" targets=skill,agents order=1 -->\n" +
		"body\n<!-- rtdd:variant target=agents -->\nshort\n<!-- rtdd:endsection -->\n"
	_, err := Parse(src)
	if err == nil {
		t.Fatal("want error for an unterminated variant")
	}
}

func TestDuplicateIDIsAnError(t *testing.T) {
	src := "<!-- rtdd:meta version=1 -->\n" +
		"<!-- rtdd:section id=x title=\"X\" targets=skill order=1 -->\na\n<!-- rtdd:endsection -->\n" +
		"<!-- rtdd:section id=x title=\"X\" targets=skill order=2 -->\nb\n<!-- rtdd:endsection -->\n"
	if _, err := Parse(src); err == nil {
		t.Fatal("want error for a duplicate section id")
	}
}

func TestMalformedOrderIsAnError(t *testing.T) {
	src := "<!-- rtdd:meta version=1 -->\n<!-- rtdd:section id=x title=\"X\" targets=skill order=notanumber -->\nb\n<!-- rtdd:endsection -->\n"
	if _, err := Parse(src); err == nil {
		t.Fatal("want error for a malformed order")
	}
}

func TestUnknownTargetIsAnError(t *testing.T) {
	src := "<!-- rtdd:meta version=1 -->\n<!-- rtdd:section id=x title=\"X\" targets=vscode order=1 -->\nb\n<!-- rtdd:endsection -->\n"
	if _, err := Parse(src); err == nil {
		t.Fatal("want error for an unknown target")
	}
}

func ids(secs []Section) []string {
	out := make([]string, 0, len(secs))
	for _, s := range secs {
		out = append(out, s.ID)
	}
	return out
}
