package selfupdate

import (
	"fmt"
	"os"
)

// RemoveInstalled deletes the installed rtdd binary and returns the path it removed.
//
// target names the file; empty means the running one, resolved through any symlink the
// same way an update resolves it. A binary that is already gone is not an error —
// uninstalling twice is a thing people do.
//
// On Unix this unlinks a running executable, which is allowed: the process keeps running
// from the open inode and the path goes away. Windows refuses to unlink a running image
// and says so rather than being worked around, because the honest message is more useful
// than a file renamed to something the user did not ask for.
func RemoveInstalled(target string) (string, error) {
	path, err := resolveTarget(target)
	if err != nil {
		return "", err
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return path, nil
		}
		return path, fmt.Errorf("cannot remove %s (%v): %w", path, err, ErrNotWritable)
	}
	return path, nil
}
