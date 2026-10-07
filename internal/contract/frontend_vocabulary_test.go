package contract

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// v02Vocabulary is what no generated front-end may say (PRD #411 AC2): the v0.2 commands,
// state files and concepts, every language and every test framework. Each entry is shaped
// so ordinary English does not trip it — `run` is forbidden only as an rtdd command or a
// code span, never as the verb the five steps need; language names are matched in their
// proper capitalisation, so "go back" and "rust" are prose while "Go" and "Rust" are names.
var v02Vocabulary = []struct {
	what string
	re   *regexp.Regexp
}{
	{"the seed command", regexp.MustCompile(`(?i)\bseed(s|ed|ing)?\b`)},
	{"the run command", regexp.MustCompile("(?i)\\brtdd\\s+run\\b|`run`")},
	{"the verify command", regexp.MustCompile(`(?i)\bverif(y|ies|ied|ying)\b`)},
	{"the status command", regexp.MustCompile(`(?i)\brtdd\s+status\b`)},
	{"the map command", regexp.MustCompile(`(?i)\brtdd\s+map\b`)},
	{"the v0.2 map file", regexp.MustCompile(`map\.jsonl`)},
	{"the v0.2 meta file", regexp.MustCompile(`meta\.json`)},
	{"adapters", regexp.MustCompile(`(?i)\badapters?\b`)},
	{"coverage", regexp.MustCompile(`(?i)\b(coverage|uncovered)\b`)},
	{"the T0/T1/T2 tiers", regexp.MustCompile(`\bT[0-2]\b`)},
	{"selection_fidelity", regexp.MustCompile(`selection_fidelity`)},
	{"a v0.2 run flag", regexp.MustCompile(`--fail-fast`)},
	{"a language", regexp.MustCompile(`\b(Python|Go|JavaScript|TypeScript|Java|Rust|Ruby|Lua|Luau|Bash|Kotlin|PHP|Swift|Dart|Perl|Nim|Scala|Elixir|Node\.js)\b|C#|C\+\+`)},
	{"a test framework or toolchain", regexp.MustCompile(`(?i)\b(pytest|unittest|jest|vitest|mocha|jasmine|junit|testng|nunit|xunit|mstest|rspec|minitest|phpunit|cargo|maven|mvn|gradle|dotnet|npm|npx|yarn|pnpm|go test)\b`)},
}

// vocabularyAllowed are exact phrases under dist/ that may contain a forbidden match, each
// with the reason. It starts empty — the entries above are shaped so the protocol's own
// prose needs none — and an entry whose phrase no file under dist/ still holds is an error:
// the list only shrinks.
var vocabularyAllowed = map[string]string{}

// vocabularyHits returns what of v02Vocabulary text says, after removing allowed phrases.
func vocabularyHits(text string) []string {
	for phrase := range vocabularyAllowed {
		text = strings.ReplaceAll(text, phrase, "")
	}
	var hits []string
	for _, v := range v02Vocabulary {
		if m := v.re.FindString(text); m != "" {
			hits = append(hits, v.what+" ("+m+")")
		}
	}
	return hits
}

// PRD #411 AC2: no file under dist/ says any of v02Vocabulary.
func TestFrontEndsUseNoV02Vocabulary(t *testing.T) {
	root := repoRoot(t)
	var all strings.Builder
	files := 0
	err := filepath.WalkDir(filepath.Join(root, "dist"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		files++
		all.Write(b)
		rel, _ := filepath.Rel(root, p)
		for _, hit := range vocabularyHits(string(b)) {
			t.Errorf("%s says %s", filepath.ToSlash(rel), hit)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files < 5 {
		t.Fatalf("read %d files under dist/, want the 5 generated front-ends", files)
	}
	for phrase, why := range vocabularyAllowed {
		if !strings.Contains(all.String(), phrase) {
			t.Errorf("vocabularyAllowed holds %q (%s) but no front-end says it: remove it", phrase, why)
		}
	}
}

// The guard is only as good as its patterns: each forbidden form trips it, and the prose
// the five steps need does not.
func TestV02VocabularyCatchesTheFormsAndSparesOrdinaryEnglish(t *testing.T) {
	for _, bad := range []string{"run `rtdd seed` once", "rtdd run --fail-fast", "rtdd verify", "rtdd status",
		"rtdd map compact", "the map.jsonl file", "meta.json", "the python adapter", "recorded coverage",
		"tier T2", "selection_fidelity", "a Go repository", "TypeScript", "C#", "run pytest", "Jest", "go test ./..."} {
		if len(vocabularyHits(bad)) == 0 {
			t.Errorf("vocabularyHits(%q) found nothing", bad)
		}
	}
	for _, ok := range []string{"Run **Round 1** with the project's own test command.", "rtdd runs no tests",
		"return to step 2", "go back to step 2", "never a pass", "the full suite once", "rtdd which --json"} {
		if hits := vocabularyHits(ok); len(hits) != 0 {
			t.Errorf("vocabularyHits(%q) = %v, want nothing: ordinary prose", ok, hits)
		}
	}
}
