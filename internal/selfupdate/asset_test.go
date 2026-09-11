package selfupdate

import "testing"

// TestArchiveNameMatchesTheGoreleaserTemplate pins the name half of the protocol
// install.sh implements: .goreleaser.yaml's name_template is
// "rtdd_{{ .Version }}_{{ .Os }}_{{ .Arch }}", the version in it carries no leading v, and
// format_overrides makes Windows a zip. Getting any of that wrong produces a 404 rather
// than a wrong binary, but it produces it every time.
func TestArchiveNameMatchesTheGoreleaserTemplate(t *testing.T) {
	for _, tc := range []struct{ tag, goos, goarch, want string }{
		{"v0.1.1", "linux", "amd64", "rtdd_0.1.1_linux_amd64.tar.gz"},
		{"0.1.1", "linux", "amd64", "rtdd_0.1.1_linux_amd64.tar.gz"},
		{"v0.1.1", "linux", "arm64", "rtdd_0.1.1_linux_arm64.tar.gz"},
		{"v0.1.1", "darwin", "arm64", "rtdd_0.1.1_darwin_arm64.tar.gz"},
		{"v9.9.9", "windows", "amd64", "rtdd_9.9.9_windows_amd64.zip"},
	} {
		if got := archiveName(tc.tag, tc.goos, tc.goarch); got != tc.want {
			t.Errorf("archiveName(%q, %q, %q) = %q, want %q", tc.tag, tc.goos, tc.goarch, got, tc.want)
		}
	}
}

// TestBinaryNameCarriesTheWindowsSuffix is the entry to pull out of the archive.
func TestBinaryNameCarriesTheWindowsSuffix(t *testing.T) {
	if got := binaryName("linux"); got != "rtdd" {
		t.Errorf("binaryName(linux) = %q, want rtdd", got)
	}
	if got := binaryName("windows"); got != "rtdd.exe" {
		t.Errorf("binaryName(windows) = %q, want rtdd.exe", got)
	}
}
