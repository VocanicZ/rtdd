package main

import (
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
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
	got := metaAfterRun(meta{V: 1, Adapter: "python"}, legacyRepoAdapters(), selector.TierT0, "")
	if got.Adapter != "python" {
		t.Errorf("meta.adapter = %q, want the name already recorded", got.Adapter)
	}
}

// A meta written before `v` existed still gets one; the run must not write a versionless
// document back.
func TestRunStampsTheSchemaVersionOnAVersionlessMeta(t *testing.T) {
	if got := metaAfterRun(meta{}, legacyRepoAdapters(), selector.TierT0, "").V; got != 1 {
		t.Errorf("meta.v after run = %d, want 1", got)
	}
}

// meta.json's singular `adapter` is the first detected adapter when meta names none.
func TestRunNamesTheFirstDetectedAdapterWhenMetaNamesNone(t *testing.T) {
	if got := metaAfterRun(meta{V: 2}, legacyRepoAdapters(), selector.TierT0, "").Adapter; got != "maven" {
		t.Errorf("meta.adapter after run = %q, want maven", got)
	}
}
