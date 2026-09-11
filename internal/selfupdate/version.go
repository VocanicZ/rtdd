// Package selfupdate replaces the running rtdd binary with a published release.
//
// It is the only package in this module that talks to the network, and it does so only
// when a person types `rtdd update`. Nothing here runs on any other code path.
package selfupdate

import (
	"fmt"
	"strconv"
	"strings"
)

// parsed is a release version: three numbers and an optional pre-release suffix.
type parsed struct {
	nums [3]int
	pre  string
}

// parseVersion reads the forms this project actually produces: a tag (v0.1.1), the numeric
// form GoReleaser puts in archive names (0.1.1), and its snapshot template's pre-release
// (0.1.2-next). Anything else is refused rather than coerced — "dev" is what an
// ldflags-less `go build` leaves behind, and treating it as a number would order it.
func parseVersion(s string) (parsed, error) {
	raw := strings.TrimPrefix(strings.TrimSpace(s), "v")
	if raw == "" {
		return parsed{}, fmt.Errorf("%q is not a release version", s)
	}
	core, pre, _ := strings.Cut(raw, "-")
	fields := strings.Split(core, ".")
	if len(fields) != 3 {
		return parsed{}, fmt.Errorf("%q is not a release version: want major.minor.patch", s)
	}
	var p parsed
	for i, f := range fields {
		if f == "" || strings.IndexFunc(f, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
			return parsed{}, fmt.Errorf("%q is not a release version: %q is not a number", s, f)
		}
		n, err := strconv.Atoi(f)
		if err != nil {
			return parsed{}, fmt.Errorf("%q is not a release version: %w", s, err)
		}
		p.nums[i] = n
	}
	p.pre = pre
	return p, nil
}

// compare returns -1, 0 or 1 as a is older than, the same as, or newer than b. A
// pre-release sorts below the release it counts towards, so 0.1.2-next < 0.1.2.
func compare(a, b string) (int, error) {
	pa, err := parseVersion(a)
	if err != nil {
		return 0, err
	}
	pb, err := parseVersion(b)
	if err != nil {
		return 0, err
	}
	for i := range pa.nums {
		switch {
		case pa.nums[i] > pb.nums[i]:
			return 1, nil
		case pa.nums[i] < pb.nums[i]:
			return -1, nil
		}
	}
	switch {
	case pa.pre == pb.pre:
		return 0, nil
	case pa.pre == "":
		return 1, nil
	case pb.pre == "":
		return -1, nil
	}
	return strings.Compare(pa.pre, pb.pre), nil
}
