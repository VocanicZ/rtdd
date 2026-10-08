package contract

import (
	"regexp"
	"strings"
	"testing"
)

// v030Docs are the pages that describe rtdd as it is now (PRD #411 AC7). README.md and
// README.positive.md are one file twice (internal/installtest/outcomes_test.go).
var v030Docs = []string{"README.md", "docs/outcomes/README.positive.md", "DEVELOPMENT.md", "docs/LIMITATIONS.md",
	"docs/PRIOR-ART.md"}

// v030Guides are the v030Docs that teach the commands and the process, so they must name them.
var v030Guides = map[string]bool{"README.md": true, "docs/outcomes/README.positive.md": true, "DEVELOPMENT.md": true}

// A v0.2 measurement or changelog kept on a v0.3.0 page sits between these markers, and
// the v0.2 vocabulary guard does not read it: it is a dated record, not a description.
const (
	v02RecordOpen  = "<!-- rtdd:v0.2-record -->"
	v02RecordClose = "<!-- /rtdd:v0.2-record -->"
)

// outsideV02Records drops every marked record from src; an unbalanced marker is an error.
func outsideV02Records(t *testing.T, rel, src string) string {
	t.Helper()
	var b strings.Builder
	for {
		i := strings.Index(src, v02RecordOpen)
		if i < 0 {
			break
		}
		j := strings.Index(src[i:], v02RecordClose)
		if j < 0 {
			t.Errorf("%s opens a v0.2 record it never closes", rel)
			return src
		}
		b.WriteString(src[:i])
		src = src[i+j+len(v02RecordClose):]
	}
	b.WriteString(src)
	if strings.Contains(b.String(), v02RecordClose) {
		t.Errorf("%s closes a v0.2 record it never opened", rel)
	}
	return b.String()
}

// v02AsCurrent is v0.2 behaviour a v0.3.0 page may not present as current. "no coverage"
// and "no adapters" say what v0.3.0 does not do, and are removed before matching.
var v02AsCurrent = []struct {
	what string
	re   *regexp.Regexp
}{
	{"a removed command", regexp.MustCompile(`\brtdd\s+(seed|run|verify|status|map)\b`)},
	{"the v0.2 map file", regexp.MustCompile(`map\.jsonl`)},
	{"adapters", regexp.MustCompile(`(?i)\badapters?\b`)},
	{"coverage", regexp.MustCompile(`(?i)\bcoverage\b`)},
	{"the T0/T1/T2 tiers", regexp.MustCompile(`\bT[0-2]\b`)},
	{"selection_fidelity", regexp.MustCompile(`selection_fidelity`)},
}

var saysWhatV030DoesNot = regexp.MustCompile(`(?i)\bno\s+(coverage|adapters?)\b`)

// PRD #411 AC7: README, DEVELOPMENT.md, LIMITATIONS.md and PRIOR-ART.md present no v0.2
// behaviour as current, and README and DEVELOPMENT.md describe the v0.3.0 commands and process.
func TestUserDocsDescribeV030NotV02(t *testing.T) {
	for _, rel := range v030Docs {
		body := saysWhatV030DoesNot.ReplaceAllString(outsideV02Records(t, rel, readRepoFile(t, rel)), "")
		for _, v := range v02AsCurrent {
			for _, m := range v.re.FindAllString(body, 3) {
				t.Errorf("%s presents %s as current (%q) outside a v0.2 record", rel, v.what, m)
			}
		}
		if !v030Guides[rel] {
			continue
		}
		for _, want := range []string{"rtdd init", "rtdd which", "rtdd graph", "rtdd explain", "rtdd doctor",
			"Round 1", "Round 2", "full suite once", "graphify"} {
			if !strings.Contains(body, want) {
				t.Errorf("%s does not mention %q", rel, want)
			}
		}
	}
}

// supersededSpecs are the designs docs/specs/2026-10-07-node-graph.md replaces. They stay
// as the record of what was built and why; each opens with supersededBanner.
var supersededSpecs = []string{
	"docs/specs/2026-08-26-rtdd-design.md",
	"docs/specs/2026-09-05-multi-language.md",
	"docs/specs/2026-09-29-one-pipeline.md",
}

const supersededBanner = "> **Superseded** by [`2026-10-07-node-graph.md`](2026-10-07-node-graph.md) (v0.3.0). " +
	"Kept as the record of the design it describes; do not implement from it."

// PRD #411 AC7: every superseded spec still exists and opens with the banner; the current
// spec does not carry it.
func TestSupersededSpecsAreMarkedNotDeleted(t *testing.T) {
	for _, rel := range supersededSpecs {
		if src := readRepoFile(t, rel); !strings.HasPrefix(src, supersededBanner+"\n") {
			t.Errorf("%s does not open with the banner %q", rel, supersededBanner)
		}
	}
	if strings.Contains(readRepoFile(t, "docs/specs/2026-10-07-node-graph.md"), "**Superseded**") {
		t.Error("the current spec carries a superseded banner")
	}
}
