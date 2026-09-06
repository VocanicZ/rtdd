package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/mapstore"
)

// twoAdapters is the polyglot repository of spec §4.4: a Python tooling directory beside
// a TypeScript service. Detection returns both since #309, so every command below has to
// answer twice.
func twoAdapters() []*adapter.Adapter {
	return []*adapter.Adapter{
		{
			Name: "python", Detect: []string{"pyproject.toml"},
			Selection: adapter.SelectionCoverage, Coverage: "sqlite",
			TestGlobs: []string{"tests/**/*.py"}, SourceGlobs: []string{"src/**/*.py"},
		},
		{
			Name: "vitest", Detect: []string{"vitest.config.ts"},
			Selection: adapter.SelectionStatic, Coverage: adapter.CoverageNone,
			TestGlobs: []string{"**/*.test.ts"}, SourceGlobs: []string{"src/**/*.ts"},
			TestFor: []string{"{dir}/{name}.test.ts"},
		},
	}
}

func polyglotMap() *mapstore.Map {
	m := mapstore.New()
	m.Replace(mapstore.Row{T: "tests/test_calc.py::test_add", F: []string{"src/calc.py"}, A: "python", S: "pass"})
	m.Replace(mapstore.Row{T: "src/calc.test.ts", F: []string{"src/calc.ts"}, A: "vitest", S: "pass"})
	return m
}

// staticRepo is a repository the correspondence resolver can answer over: a static
// adapter selects a test file that EXISTS, and repoExists reads the real filesystem.
func staticRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "calc.test.ts"), []byte("test('x', () => {})\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func bothFilesChanged() []gitctx.Change {
	return []gitctx.Change{
		{Path: "src/calc.py", Status: gitctx.Modified},
		{Path: "src/calc.ts", Status: gitctx.Modified},
	}
}

// Spec §4.4: each adapter answers for itself. Merging the two lists would hand one
// runner the other's selectors, which is PRD #232 AC6's forbidden case with extra steps.
func TestSelectPerAdapterKeepsEachAdaptersIDsInItsOwnBlock(t *testing.T) {
	root := staticRepo(t)
	got, err := selectPerAdapter(root, twoAdapters(), polyglotMap(),
		mapstore.Meta{V: 1, Adapter: "python", Adapters: []string{"python", "vitest"}},
		selectionContext{Changes: bothFilesChanged()})
	if err != nil {
		t.Fatalf("selectPerAdapter: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d blocks, want one per detected adapter", len(got))
	}
	for _, blk := range got {
		if len(blk.Selection.Tests) == 0 {
			t.Errorf("%s selected nothing; its own changed file is in the changed set", blk.Adapter)
		}
		for _, id := range blk.Selection.Tests {
			if blk.Adapter == "python" && id == "src/calc.test.ts" {
				t.Errorf("python's block contains vitest's id %q", id)
			}
			if blk.Adapter == "vitest" && id == "tests/test_calc.py::test_add" {
				t.Errorf("vitest's block contains python's id %q", id)
			}
		}
	}
}

// Decision 3: an untagged row is the adapter meta.json names, and nobody else's. A
// polyglot repository that seeded before the tag existed must not have its pytest ids
// handed to the vitest runner.
func TestSelectPerAdapterServesAnUntaggedRowOnlyToTheAdapterMetaNames(t *testing.T) {
	m := mapstore.New()
	m.Replace(mapstore.Row{T: "tests/test_calc.py::test_add", F: []string{"src/calc.py", "src/calc.ts"}, S: "pass"})

	got, err := selectPerAdapter(staticRepo(t), twoAdapters(), m,
		mapstore.Meta{V: 1, Adapter: "python"},
		selectionContext{Changes: bothFilesChanged()})
	if err != nil {
		t.Fatalf("selectPerAdapter: %v", err)
	}
	for _, blk := range got {
		if blk.Adapter != "vitest" {
			continue
		}
		if len(blk.Selection.Tests) == 0 {
			t.Fatal("vitest selected nothing at all; the assertion below would then be vacuous")
		}
		for _, id := range blk.Selection.Tests {
			if id == "tests/test_calc.py::test_add" {
				t.Errorf("vitest's block contains the untagged python row %q", id)
			}
		}
	}
}

// A one-adapter repository is the overwhelmingly common one, and its selection is what
// spec §4.1 promises to hold byte-identical.
func TestSelectPerAdapterOnOneAdapterIsOneBlockNamingIt(t *testing.T) {
	ads := twoAdapters()[:1]
	got, err := selectPerAdapter(t.TempDir(), ads, polyglotMap(),
		mapstore.Meta{V: 1, Adapter: "python"},
		selectionContext{Changes: bothFilesChanged()})
	if err != nil {
		t.Fatalf("selectPerAdapter: %v", err)
	}
	if len(got) != 1 || got[0].Adapter != "python" {
		t.Fatalf("got %+v, want a single python block", got)
	}
}

// A repository with no adapter at all still gets an answer: file classification is
// disabled and `which` says so, which is the behaviour that predates detection returning
// a set. One block with no adapter is how that survives the loop.
func TestSelectPerAdapterWithNoAdapterStillAnswersOnce(t *testing.T) {
	got, err := selectPerAdapter(t.TempDir(), nil, polyglotMap(), mapstore.Meta{V: 1},
		selectionContext{Changes: bothFilesChanged()})
	if err != nil {
		t.Fatalf("selectPerAdapter: %v", err)
	}
	if len(got) != 1 || got[0].Adapter != "" {
		t.Fatalf("got %+v, want one unnamed block", got)
	}
}

// PRD #232 AC6, on the human surface: the reader must be able to tell which toolchain
// produced which ids, or the two lists are one list with a blank line in it.
func TestRenderSelectionsNamesEachAdapterInAPolyglotRepository(t *testing.T) {
	blocks, err := selectPerAdapter(staticRepo(t), twoAdapters(), polyglotMap(),
		mapstore.Meta{V: 1, Adapter: "python", Adapters: []string{"python", "vitest"}},
		selectionContext{Changes: bothFilesChanged()})
	if err != nil {
		t.Fatalf("selectPerAdapter: %v", err)
	}
	out := RenderSelections(blocks)
	for _, want := range []string{"adapter: python", "adapter: vitest",
		"tests/test_calc.py::test_add", "src/calc.test.ts"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered selection %q does not contain %q", out, want)
		}
	}
	if strings.Index(out, "src/calc.test.ts") < strings.Index(out, "adapter: vitest") {
		t.Error("vitest's id is rendered above its own heading; each block must sit under the adapter that produced it")
	}
}

// A one-adapter repository prints EXACTLY what it printed before the loop existed. The
// heading is per-adapter disambiguation, and there is nothing to disambiguate.
func TestRenderSelectionsAddsNoHeadingForASingleAdapter(t *testing.T) {
	blocks, err := selectPerAdapter(t.TempDir(), twoAdapters()[:1], polyglotMap(),
		mapstore.Meta{V: 1, Adapter: "python"},
		selectionContext{Changes: bothFilesChanged()})
	if err != nil {
		t.Fatalf("selectPerAdapter: %v", err)
	}
	got := RenderSelections(blocks)
	want := RenderWhich(blocks[0].Selection, blocks[0].Signal.UnmappedFiles, blocks[0].Ad)
	if got != want {
		t.Errorf("single-adapter rendering changed:\ngot  %q\nwant %q", got, want)
	}
}
