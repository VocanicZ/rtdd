// readme_install_test.go guards the README's Install block against promising a route
// that 404s (#374). The repository is private, there are zero tags and zero releases, so
// an anonymous `curl -fsSL .../install.sh | sh` and the releases/latest API it calls both
// fail today. The Install block therefore has to document the build-from-source route
// beside the one-liner and say which route applies when — and the negative outcome README,
// whose whole premise is that no release is shipped, has to say that in its own prose
// instead of copying the positive text.
package installtest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The one-line installer the README has always offered. It stays — it is the shorter
// route the moment a release exists — but it is no longer offered alone.
const curlInstallLine = "curl -fsSL https://raw.githubusercontent.com/VocanicZ/rtdd/main/install.sh | sh"

// The root README and the positive outcome file are byte-identical by construction
// (outcomes_test.go); both are listed so a divergence names the file that broke.
var releaseShippingREADMEs = []string{
	"README.md",
	filepath.Join("docs", "outcomes", "README.positive.md"),
}

const negativeREADME = "docs/outcomes/README.negative.md"

func readREADME(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.FromSlash(name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}

// AC1: every README documents the build-from-source route, because it is the only route
// that works right now.
func TestEveryREADMEDocumentsTheBuildFromSourceRoute(t *testing.T) {
	names := append(append([]string{}, releaseShippingREADMEs...), negativeREADME)
	for _, name := range names {
		text := readREADME(t, name)
		for _, needle := range []string{"go build ./cmd/rtdd", "Go 1.24"} {
			if !strings.Contains(text, needle) {
				t.Errorf("%s does not document the build-from-source route: missing %q", name, needle)
			}
		}
	}
}

// AC2: the release-shipping READMEs keep the one-liner and state, in prose, that no
// release exists yet and that the one-liner starts working once one is published.
func TestReleaseShippingREADMEsSayWhichInstallRouteAppliesWhen(t *testing.T) {
	for _, name := range releaseShippingREADMEs {
		text := readREADME(t, name)
		if !strings.Contains(text, curlInstallLine) {
			t.Errorf("%s dropped the one-line installer; #374 asks for it to be kept, not replaced", name)
		}
		lower := strings.ToLower(text)
		if !strings.Contains(lower, "no release") {
			t.Errorf("%s does not say that no release exists yet, so a reader cannot tell why the one-liner fails", name)
		}
		if !strings.Contains(lower, "published") {
			t.Errorf("%s does not say the one-liner starts working once a release is published", name)
		}
	}
}

// AC4: the negative branch ships no release at all, so its Install section says so rather
// than offering a one-liner that can never work on that branch.
func TestNegativeREADMEOffersNoInstallerItWillNeverPublish(t *testing.T) {
	text := readREADME(t, negativeREADME)
	if strings.Contains(text, curlInstallLine) {
		t.Errorf("%s offers the one-line installer, but the negative branch publishes no release for it to fetch", negativeREADME)
	}
	if !strings.Contains(text, "## Install") {
		t.Errorf("%s has no Install section stating which route exists on this branch", negativeREADME)
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "not released") && !strings.Contains(lower, "no release") {
		t.Errorf("%s does not state that no release is shipped on this branch", negativeREADME)
	}
}

// AC5: scripts/release-preflight.sh greps README.md for these and must keep reporting
// `none`. The outcome files are checked too, since either can become README.md.
func TestREADMEsCarryNoPlaceholders(t *testing.T) {
	names := append(append([]string{}, releaseShippingREADMEs...), negativeREADME)
	for _, name := range names {
		text := readREADME(t, name)
		for _, placeholder := range []string{"TBD", "TO" + "DO", "FIX" + "ME", "REPLACE" + "_WITH"} {
			if strings.Contains(text, placeholder) {
				t.Errorf("%s contains the placeholder %q; scripts/release-preflight.sh would stop reporting `none`", name, placeholder)
			}
		}
	}
}

// AC6: the edit is confined to install routes. The claims that make this README a
// measurement rather than a pitch are spot-checked so an Install rewrite cannot quietly
// take one with it.
func TestInstallEditLeavesTheProductClaimsAlone(t *testing.T) {
	text := readREADME(t, "README.md")
	for _, claim := range []string{
		"It is a context provider, not a gate.",
		"does not carry its weight as a distinct tier",
		"It does not enforce anything.",
		"rtdd init      # front-ends, .gitattributes merge=union, config",
	} {
		if !strings.Contains(text, claim) {
			t.Errorf("README.md no longer states %q — the Install edit was not confined to install routes", claim)
		}
	}
}
