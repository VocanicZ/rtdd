package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/VocanicZ/rtdd/internal/install"
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

// selfUpdate is selfupdate.Update, a variable so a test can stand in a release without a
// download.
var selfUpdate = selfupdate.Update

// refreshWith runs `<bin> skill install --refresh`: the freshly installed binary refreshes
// the machine-wide skill from its own agent table and renders. Its output is the command's.
var refreshWith = func(bin string, stdout, stderr io.Writer) error {
	cmd := exec.Command(bin, "skill", "install", "--refresh")
	cmd.Stdout, cmd.Stderr = stdout, stderr
	return cmd.Run()
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
	res, err := selfUpdate(version, opts)
	if err != nil {
		fmt.Fprintf(stderr, "rtdd update: %v\n", err)
		if errors.Is(err, selfupdate.ErrNotComparable) || errors.Is(err, selfupdate.ErrNotWritable) {
			return 2
		}
		return 3
	}
	fmt.Fprint(stdout, renderUpdate(res))
	// The front-ends are embedded in the binary, so a replaced binary makes whatever is in
	// the home directory stale. This process is the OLD release: its agent table and its
	// renders are the ones being replaced, so the refresh is the new binary's to do (#566).
	// An already-current install runs the same re-check itself — it is the newest release —
	// for an agent installed since, or a skill deleted. --check writes nothing.
	switch {
	case res.Updated:
		if err := refreshWith(res.Target, stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "note: the new rtdd could not refresh the agent skill (%v); run `rtdd skill install`\n", err)
		}
	case !opts.CheckOnly:
		refreshGlobalSkill(stdout, stderr, false)
	}
	// What an older rtdd wrote outside its own files is reported on every outcome: it is a
	// read, and an upgrade is when a user meets the new, self-contained footprint (#509).
	var legacy []install.Step
	if root, err := findRepoRoot("."); err == nil {
		legacy = repoLegacy(root)
	}
	if legacy = append(legacy, globalLegacy()...); len(legacy) > 0 {
		fmt.Fprint(stdout, RenderInit(legacy))
	}
	return 0
}
