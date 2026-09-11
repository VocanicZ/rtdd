package selfupdate

import "testing"

// TestCompareOrdersReleaseVersions pins the only question the default `rtdd update` asks:
// is the published release newer than the one running? Versions arrive with and without
// the leading v (the tag carries it, the archive name does not), and GoReleaser's snapshot
// template produces pre-release forms like 0.1.2-next, which are older than the release
// they are counting towards.
func TestCompareOrdersReleaseVersions(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{
		{"0.1.1", "0.1.1", 0},
		{"v0.1.1", "0.1.1", 0},
		{"0.1.1", "v0.1.1", 0},
		{"0.1.2", "0.1.1", 1},
		{"0.1.1", "0.1.2", -1},
		{"0.2.0", "0.1.9", 1},
		{"1.0.0", "0.9.9", 1},
		{"0.1.10", "0.1.9", 1},
		{"0.1.2-next", "0.1.2", -1},
		{"0.1.2", "0.1.2-next", 1},
		{"0.1.2-next", "0.1.1", 1},
	} {
		got, err := compare(tc.a, tc.b)
		if err != nil {
			t.Errorf("compare(%q, %q): unexpected error: %v", tc.a, tc.b, err)
			continue
		}
		if got != tc.want {
			t.Errorf("compare(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

// TestCompareRefusesVersionsItCannotOrder covers the source build: `go build` with no
// ldflags leaves version at "dev" (cmd/rtdd/version.go), and "dev" against a release is
// not a comparison with an answer. Guessing here would silently replace someone's own
// build with a published binary.
func TestCompareRefusesVersionsItCannotOrder(t *testing.T) {
	for _, v := range []string{"dev", "", "none", "0.1", "0.1.x", "v"} {
		if _, err := compare(v, "0.1.1"); err == nil {
			t.Errorf("compare(%q, \"0.1.1\") returned no error; an unorderable version must refuse rather than guess", v)
		}
	}
}
