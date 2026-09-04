package install

import (
	"strings"
	"testing"

	"github.com/VocanicZ/rtdd/internal/protocol"
)

func block(body string) string {
	return protocol.BeginMarker + "\n" + body + "\n" + protocol.EndMarker + "\n"
}

func TestMergeIntoAnEmptyFileIsTheBlockAlone(t *testing.T) {
	got, action, err := MergeBlock("", block("v1"))
	if err != nil {
		t.Fatal(err)
	}
	if action != Create {
		t.Fatalf("action = %v, want Create", action)
	}
	if got != block("v1") {
		t.Fatalf("got %q", got)
	}
}

func TestMergeAppendsToAFileWithNoMarkersAndKeepsEverything(t *testing.T) {
	existing := "# Our conventions\n\nUse tabs. Ship on Fridays.\n"
	got, action, err := MergeBlock(existing, block("v1"))
	if err != nil {
		t.Fatal(err)
	}
	if action != AppendBlock {
		t.Fatalf("action = %v, want AppendBlock", action)
	}
	if !strings.HasPrefix(got, existing) {
		t.Fatal("existing content was modified")
	}
	if !strings.Contains(got, "Ship on Fridays.") {
		t.Fatal("existing content was lost")
	}
	if !strings.Contains(got, protocol.BeginMarker) {
		t.Fatal("block was not appended")
	}
}

func TestMergeReplacesOnlyBetweenTheMarkers(t *testing.T) {
	existing := "# Ours\n\nbefore\n\n" + block("v1") + "\nafter\n"
	got, action, err := MergeBlock(existing, block("v2"))
	if err != nil {
		t.Fatal(err)
	}
	if action != ReplaceBlock {
		t.Fatalf("action = %v, want ReplaceBlock", action)
	}
	if !strings.Contains(got, "before") || !strings.Contains(got, "after") {
		t.Fatal("content outside the markers was modified")
	}
	if strings.Contains(got, "v1") {
		t.Fatal("old block survived")
	}
	if !strings.Contains(got, "v2") {
		t.Fatal("new block missing")
	}
}

func TestMergeIsIdempotent(t *testing.T) {
	existing := "# Ours\n\nbefore\n\n" + block("v1") + "\nafter\n"
	once, _, _ := MergeBlock(existing, block("v1"))
	twice, action, _ := MergeBlock(once, block("v1"))
	if once != twice {
		t.Fatal("merge is not idempotent")
	}
	if action != Skip {
		t.Fatalf("action = %v, want Skip on an unchanged block", action)
	}
}

func TestUnterminatedMarkerIsAnErrorNotAGuess(t *testing.T) {
	existing := "# Ours\n\n" + protocol.BeginMarker + "\nhalf a block\n"
	if _, _, err := MergeBlock(existing, block("v1")); err == nil {
		t.Fatal("want an error for an unterminated rtdd block")
	}
}

func TestDoubledBeginMarkerIsAnError(t *testing.T) {
	existing := block("v1") + block("v1")
	if _, _, err := MergeBlock(existing, block("v2")); err == nil {
		t.Fatal("want an error when two rtdd blocks are present")
	}
}
