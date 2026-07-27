//go:build unix

package globaldeps

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// lockManifest acquires an exclusive advisory lock on the manifest lockfile
// adjacent to the given manifest path. The caller must call unlockManifest
// when done.
func lockManifest(manifestPath string) (*os.File, error) {
	lockPath := manifestPath + ".lock"
	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		return nil, fmt.Errorf("creating manifest dir: %w", err)
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("opening manifest lock %s: %w", lockPath, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("acquiring manifest lock on %s: %w", lockPath, err)
	}
	return f, nil
}

// unlockManifest releases the advisory lock and closes the file.
// Safe to call with a nil argument.
func unlockManifest(f *os.File) {
	if f == nil {
		return
	}
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	_ = f.Close()
}
