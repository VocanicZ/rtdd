package main

import (
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/mapstore"
	"github.com/VocanicZ/rtdd/internal/selector"
)

// legacyRepoAdapters is a polyglot repository: a Java module beside a Python one.
func legacyRepoAdapters() []*adapter.Adapter {
	return []*adapter.Adapter{
		{
			Name: "maven", Detect: []string{"pom.xml"},
			TestGlobs: []string{"src/test/java/**/*.java"}, SourceGlobs: []string{"src/main/java/**/*.java"},
		},
		{
			Name: "python", Detect: []string{"pyproject.toml"},
			TestGlobs: []string{"tests/**/*.py"}, SourceGlobs: []string{"src/**/*.py"},
		},
	}
}

// An adapter meta ALREADY names is never rewritten: the field says whose the untagged rows
// of an existing map are, and a repository that gains a second toolchain must not have
// that answer changed underneath it.
func TestRunLeavesAnAlreadyNamedAdapterAlone(t *testing.T) {
	got := metaAfterRun(meta{V: 1, Adapter: "python"}, legacyRepoAdapters(), selector.TierT0, "", false)
	if got.Adapter != "python" {
		t.Errorf("meta.adapter = %q, want the name already recorded", got.Adapter)
	}
}

// A complete T2 run (no --fail-fast) recorded every unit, so the meta it writes names the
// current map version; the next run then selects from that map instead of escalating
// as "unseeded" forever.
func TestRunStampsTheMapVersionAfterACompleteT2Run(t *testing.T) {
	for _, in := range []meta{{}, {V: 1}} {
		if got := metaAfterRun(in, legacyRepoAdapters(), selector.TierT2, "", false).V; got != mapstore.MapVersion {
			t.Errorf("meta.v after a complete T2 run from v%d = %d, want %d", in.V, got, mapstore.MapVersion)
		}
	}
}

// A T2 run cut short by --fail-fast left units unrecorded: the map is not complete, so the
// version stays below MapVersion — but a versionless meta still gets a version.
func TestRunDoesNotStampTheMapVersionAfterAFailFastT2Run(t *testing.T) {
	if got := metaAfterRun(meta{}, legacyRepoAdapters(), selector.TierT2, "", true).V; got != 1 {
		t.Errorf("meta.v after a fail-fast T2 run = %d, want 1 (below MapVersion)", got)
	}
}

// meta.json's singular `adapter` is the first detected adapter when meta names none.
func TestRunNamesTheFirstDetectedAdapterWhenMetaNamesNone(t *testing.T) {
	if got := metaAfterRun(meta{V: 2}, legacyRepoAdapters(), selector.TierT0, "", false).Adapter; got != "maven" {
		t.Errorf("meta.adapter after run = %q, want maven", got)
	}
}
