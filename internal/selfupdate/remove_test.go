package selfupdate

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestRemoveInstalledDeletesTheBinaryAndNamesIt - `rtdd uninstall --binary` has to say
// which file it removed, because the one it found may not be the one the user meant.
func TestRemoveInstalledDeletesTheBinaryAndNamesIt(t *testing.T) {
	target := installedBinary(t)

	got, err := RemoveInstalled(target)
	if err != nil {
		t.Fatalf("RemoveInstalled: %v", err)
	}
	if got != target {
		t.Errorf("RemoveInstalled returned %q, want %q", got, target)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Errorf("the binary survived removal (stat err: %v)", err)
	}
}

// TestRemoveInstalledReportsAnUnwritableDirectory is the /usr/local/bin case again: a user
// who cannot write there gets the same configuration error, not a panic or a silent pass.
func TestRemoveInstalledReportsAnUnwritableDirectory(t *testing.T) {
	if os.Geteuid() == 0 || runtime.GOOS == "windows" {
		t.Skip("root, and Windows, do not enforce this the same way")
	}
	target := installedBinary(t)
	dir := filepath.Dir(target)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("chmod %s: %v", dir, err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })

	_, err := RemoveInstalled(target)
	if err == nil {
		t.Fatal("RemoveInstalled claimed to delete from a read-only directory")
	}
	if !errors.Is(err, ErrNotWritable) {
		t.Errorf("error is not ErrNotWritable, so the command cannot pick an exit code: %v", err)
	}
	if _, statErr := os.Stat(target); statErr != nil {
		t.Errorf("the binary was removed anyway: %v", statErr)
	}
}

// TestRemoveInstalledOnAMissingBinaryIsNotAnError - uninstalling twice is not a failure.
func TestRemoveInstalledOnAMissingBinaryIsNotAnError(t *testing.T) {
	target := filepath.Join(t.TempDir(), "rtdd")
	if _, err := RemoveInstalled(target); err != nil {
		t.Errorf("RemoveInstalled on a binary that is already gone: %v", err)
	}
}
