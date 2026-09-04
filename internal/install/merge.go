// Package install writes rtdd's generated front-ends into a host repository.
//
// AGENTS.md and CLAUDE.md belong to the host project, so rtdd owns only what is
// between its markers and never rewrites a byte outside them. Anything it cannot
// interpret unambiguously — a half-written block, two blocks — is an error, not
// a guess, because guessing here destroys someone else's file.
package install

import (
	"fmt"
	"strings"

	"github.com/VocanicZ/rtdd/internal/protocol"
)

type Action int

const (
	Create Action = iota
	ReplaceBlock
	AppendBlock
	Skip
	Conflict
)

func (a Action) String() string {
	switch a {
	case Create:
		return "create"
	case ReplaceBlock:
		return "replace-block"
	case AppendBlock:
		return "append-block"
	case Skip:
		return "skip"
	case Conflict:
		return "conflict"
	}
	return "unknown"
}

// MergeBlock inserts or replaces the marker-delimited rtdd block in existing.
// Content outside the markers is never modified.
func MergeBlock(existing, block string) (string, Action, error) {
	begins := strings.Count(existing, protocol.BeginMarker)
	ends := strings.Count(existing, protocol.EndMarker)

	switch {
	case begins == 0 && ends == 0:
		if strings.TrimSpace(existing) == "" {
			return block, Create, nil
		}
		trimmed := strings.TrimRight(existing, "\n")
		return trimmed + "\n\n" + block, AppendBlock, nil

	case begins == 1 && ends == 1:
		start := strings.Index(existing, protocol.BeginMarker)
		endAt := strings.Index(existing, protocol.EndMarker)
		if endAt < start {
			return "", Conflict, fmt.Errorf("rtdd END marker appears before BEGIN — fix the file by hand")
		}
		endAt += len(protocol.EndMarker)
		if endAt < len(existing) && existing[endAt] == '\n' {
			endAt++
		}
		current := existing[start:endAt]
		if current == block {
			return existing, Skip, nil
		}
		return existing[:start] + block + existing[endAt:], ReplaceBlock, nil

	default:
		return "", Conflict, fmt.Errorf(
			"found %d BEGIN and %d END rtdd markers — expected exactly one of each; fix the file by hand",
			begins, ends)
	}
}
