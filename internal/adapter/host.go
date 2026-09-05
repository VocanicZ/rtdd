// Host-authored adapters. The shipped set is a convenience, not the boundary of support:
// a language RTDD has never heard of is supported by writing YAML into the host repo, with
// no engine change and no release (spec §4.5).

package adapter

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// HostAdapterDir is where a host repo declares its own adapters, relative to the repo root.
const HostAdapterDir = ".rtdd/adapters"

// Invalid is one host adapter file that failed to load. It is a value rather than a
// returned error because a half-written adapter must not take the working ones down with
// it: the file is reported by name, and the rest of the set still loads.
type Invalid struct {
	Path string
	Err  error
}

// hostDir is the on-disk adapter directory for repoRoot.
func hostDir(repoRoot string) string {
	return filepath.Join(repoRoot, filepath.FromSlash(HostAdapterDir))
}

// LoadHost reads every *.yaml directly under <repoRoot>/.rtdd/adapters, discarding the
// files that do not load. A missing directory returns no adapters and no error — that is
// every repo that exists today, and its absence is not a failure.
func LoadHost(repoRoot string) ([]*Adapter, error) {
	ok, _, err := LoadHostReport(repoRoot)
	return ok, err
}

// LoadHostReport is LoadHost with the discarded files kept: it returns the adapters that
// did load and a report of the ones that did not, so `rtdd doctor` can name the failing
// file and the failing field instead of dying on it.
//
// The returned error is reserved for a directory that exists but cannot be read. A file
// that fails to parse or validate is an Invalid, never an error: partial failure is the
// point.
func LoadHostReport(repoRoot string) ([]*Adapter, []Invalid, error) {
	dir := hostDir(repoRoot)
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil, nil
	} else if err != nil {
		return nil, nil, fmt.Errorf("adapter: read dir %s: %w", dir, err)
	}

	var ok []*Adapter
	var bad []Invalid
	// os.ReadDir sorts by file name, so both slices are already deterministic; ok is
	// re-sorted by adapter name below to match what LoadFS returns.
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		p := filepath.Join(dir, e.Name())
		// Load is the same reader the embedded set goes through, so a host adapter is
		// held to exactly the contract a built-in is, contract v2 included.
		a, err := Load(p)
		if err != nil {
			bad = append(bad, Invalid{Path: p, Err: err})
			continue
		}
		ok = append(ok, a)
	}
	sort.Slice(ok, func(i, j int) bool { return ok[i].Name < ok[j].Name })
	return ok, bad, nil
}

// Available returns the built-in adapters overlaid with the host's, sorted by name.
func Available(repoRoot string) ([]*Adapter, error) {
	all, _, err := AvailableReport(repoRoot)
	return all, err
}

// AvailableReport is Available for callers that must be able to say WHY a host adapter is
// missing from the resolved set.
//
// Precedence: the HOST wins. §4.5 makes host YAML the real boundary of support, so a user
// whose shipped adapter is wrong for their monorepo must be able to correct it without an
// RTDD release. "Built-in wins" would make the shipped set unfixable, and "a collision is
// an error" would break working repos on upgrade. The override is never silent — rtdd
// doctor names the adapter and says it came from the host repo.
//
// Two HOST files claiming one name is an error: there is no principled winner, and
// picking one by file name would make the resolved set depend on a rename.
func AvailableReport(repoRoot string) ([]*Adapter, []Invalid, error) {
	builtin, err := Builtin()
	if err != nil {
		return nil, nil, err
	}
	host, bad, err := LoadHostReport(repoRoot)
	if err != nil {
		return nil, nil, err
	}

	byName := map[string]*Adapter{}
	for _, a := range builtin {
		byName[a.Name] = a
	}
	seen := map[string]string{}
	for _, a := range host {
		if prev, dup := seen[a.Name]; dup {
			return nil, nil, fmt.Errorf("adapter: two host adapters are both named %q: %s and %s", a.Name, prev, a.Src)
		}
		seen[a.Name] = a.Src
		byName[a.Name] = a
	}

	out := make([]*Adapter, 0, len(byName))
	for _, a := range byName {
		out = append(out, a)
	}
	// Map iteration is random; the resolved set is sorted so repeated runs on one repo
	// produce the same adapters in the same order.
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, bad, nil
}

// IsHostAuthored reports whether a is one of the host repo's own adapters rather than one
// embedded in the binary.
func IsHostAuthored(repoRoot string, a *Adapter) bool {
	if a == nil || a.Src == "" {
		return false
	}
	rel, err := filepath.Rel(hostDir(repoRoot), a.Src)
	if err != nil {
		return false
	}
	return rel == filepath.Base(a.Src)
}
