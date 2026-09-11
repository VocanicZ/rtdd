package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/VocanicZ/rtdd/internal/selfupdate"
)

// parseUpdateFlags turns `rtdd update`'s flags into the options the mechanism takes.
//
// --version is an override rather than a hint, in the same sense as --adapter: it installs
// the tag it names even when that tag is older than what is running, which is how a source
// build installs a release and how a bad release is rolled back.
func parseUpdateFlags(args []string) (selfupdate.Options, error) {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	check := fs.Bool("check", false, "report whether a newer release exists and write nothing")
	tag := fs.String("version", "", "install this release tag instead of the latest")
	if err := fs.Parse(args); err != nil {
		return selfupdate.Options{}, err
	}
	if fs.NArg() > 0 {
		return selfupdate.Options{}, fmt.Errorf("unexpected argument %q", fs.Arg(0))
	}
	return selfupdate.Options{
		CheckOnly: *check,
		Tag:       *tag,
		// RTDD_API_URL and RTDD_BASE_URL are install.sh's overrides, carried here so the
		// two install paths can be pointed at the same fixture server. Undocumented for
		// end users, exactly as in install.sh.
		APIURL:  os.Getenv("RTDD_API_URL"),
		BaseURL: os.Getenv("RTDD_BASE_URL"),
	}, nil
}

// renderUpdate states which of the three outcomes happened. A replacement names the path
// it wrote, because the binary that answers `rtdd` next may not be the one the user
// expected to change.
func renderUpdate(res selfupdate.Result) string {
	switch {
	case res.Updated:
		return fmt.Sprintf("updated rtdd %s -> %s at %s\n", res.Current, res.Latest, res.Target)
	case res.Outdated:
		return fmt.Sprintf("rtdd %s is installed; %s is available. Run `rtdd update` to install it.\n", res.Current, res.Latest)
	default:
		return fmt.Sprintf("rtdd %s is the latest release.\n", res.Current)
	}
}

// cmdUpdate replaces the running binary with a published release.
//
// Exit codes follow the usage block: 0 for every outcome that is merely a fact - already
// current, an update found by --check, an update applied - 2 for a configuration problem
// the caller can fix, and 3 for an environment that failed underneath it.
func cmdUpdate(args []string, stdout, stderr io.Writer) int {
	opts, err := parseUpdateFlags(args)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd update: %v\n\n%s", err, usage)
		return 2
	}
	res, err := selfupdate.Update(version, opts)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd update: %v\n", err)
		if errors.Is(err, selfupdate.ErrNotComparable) || errors.Is(err, selfupdate.ErrNotWritable) {
			return 2
		}
		return 3
	}
	fmt.Fprint(stdout, renderUpdate(res))
	// The front-ends are embedded in the binary, so a replaced binary makes whatever is in
	// the home directory stale. Only on an actual replacement: --check writes nothing, and
	// an already-current install has nothing to refresh.
	if res.Updated {
		refreshGlobalSkill(stdout, stderr)
	}
	return 0
}
