package selfupdate

import (
	"fmt"
	"strings"
)

// Repo is the only repository this command will install from. It is a constant rather
// than an option so that a hostile RTDD_BASE_URL cannot redirect the binary's identity —
// the override in Options changes where the bytes come from, which the tests need, not
// which project they claim to be.
const Repo = "VocanicZ/rtdd"

// defaultAPIURL and defaultBaseURL are install.sh's two URLs, verbatim.
const (
	defaultAPIURL  = "https://api.github.com/repos/" + Repo + "/releases/latest"
	defaultBaseURL = "https://github.com/" + Repo + "/releases/download"
)

// archiveName mirrors .goreleaser.yaml's archive name_template, including the
// format_overrides entry that makes Windows a zip. The version in the name carries no
// leading v; the tag in the URL does.
func archiveName(tag, goos, goarch string) string {
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("rtdd_%s_%s_%s%s", strings.TrimPrefix(tag, "v"), goos, goarch, ext)
}

// binaryName is the entry to pull out of the archive: GoReleaser appends .exe on Windows
// and nothing anywhere else.
func binaryName(goos string) string {
	if goos == "windows" {
		return "rtdd.exe"
	}
	return "rtdd"
}
