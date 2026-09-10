package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/VocanicZ/rtdd/internal/adapter"
	"github.com/VocanicZ/rtdd/internal/gitctx"
	"github.com/VocanicZ/rtdd/internal/selector"
)

func escalateAdapters() []*adapter.Adapter {
	return []*adapter.Adapter{{
		Name:         "python",
		FullEscalate: []string{"pyproject.toml", "**/conftest.py"},
		TestGlobs:    []string{"tests/**/*.py"},
	}}
}

func writeAt(t *testing.T, root, rel, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, filepath.Dir(rel)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEscalateDigestIsEmptyWhenNothingFullEscalateChanged(t *testing.T) {
	root := t.TempDir()
	writeAt(t, root, "src/auth.py", "x = 1")
	changes := []gitctx.Change{{Path: "src/auth.py", Status: gitctx.Modified}}

	if got := escalateDigest(root, escalateAdapters(), changes); got != "" {
		t.Fatalf("digest = %q, want empty: an ordinary source edit is not a config change", got)
	}
}

func TestEscalateDigestIsStableWhileTheEditSitsInTheDiff(t *testing.T) {
	// The whole point: an uncommitted conftest edit keeps the SAME name across
	// cycles, so the full run that covered it retires it.
	root := t.TempDir()
	writeAt(t, root, "tests/conftest.py", "import pytest")
	changes := []gitctx.Change{{Path: "tests/conftest.py", Status: gitctx.Modified}}

	first := escalateDigest(root, escalateAdapters(), changes)
	second := escalateDigest(root, escalateAdapters(), changes)
	if first == "" {
		t.Fatal("digest is empty; a changed conftest.py must name a config state")
	}
	if first != second {
		t.Fatalf("digest moved without an edit: %q then %q", first, second)
	}
}

func TestEditingTheSameFileAgainMovesTheDigest(t *testing.T) {
	root := t.TempDir()
	writeAt(t, root, "tests/conftest.py", "import pytest")
	changes := []gitctx.Change{{Path: "tests/conftest.py", Status: gitctx.Modified}}
	before := escalateDigest(root, escalateAdapters(), changes)

	writeAt(t, root, "tests/conftest.py", "import pytest\n\nfixture = 2")
	if after := escalateDigest(root, escalateAdapters(), changes); after == before {
		t.Fatal("digest did not move after a second edit; the next cycle would not re-escalate")
	}
}

func TestADeletedConfigFileIsNotTheSameStateAsAnEmptyOne(t *testing.T) {
	root := t.TempDir()
	changes := []gitctx.Change{{Path: "pyproject.toml", Status: gitctx.Deleted}}
	absent := escalateDigest(root, escalateAdapters(), changes)

	writeAt(t, root, "pyproject.toml", "")
	if empty := escalateDigest(root, escalateAdapters(), changes); empty == absent {
		t.Fatal("a deleted file and an empty file share a digest")
	}
}

func TestOnlyAFullRunRecordsTheConfigStateItCovered(t *testing.T) {
	ads := escalateAdapters()

	if got := metaAfterRun(meta{V: 1}, ads, selector.TierT0, "sha256:cfg").EscalateDigest; got != "" {
		t.Fatalf("EscalateDigest = %q after a T0 run; only a full suite retires the escalation", got)
	}
	if got := metaAfterRun(meta{V: 1}, ads, selector.TierT2, "sha256:cfg").EscalateDigest; got != "sha256:cfg" {
		t.Fatalf("EscalateDigest = %q after a T2 run, want the state it covered", got)
	}
}
