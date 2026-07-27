//go:build unix

package remote

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// AcquireLock acquires an advisory exclusive lock on a .lock file in the given
// directory. Returns the open lock file handle. The caller must pass it to
// ReleaseLock when done.
//
// This uses syscall.Flock with LOCK_EX. It is blocking -- if another process
// holds the lock, this call will wait.
func AcquireLock(dir string) (*os.File, error) {
	lockPath := filepath.Join(dir, ".lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("opening lock file %s: %w", lockPath, err)
	}

	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("acquiring lock on %s: %w", lockPath, err)
	}

	return f, nil
}

// ReleaseLock releases an advisory lock and closes the file.
// It is safe to call with a nil argument.
func ReleaseLock(f *os.File) {
	if f == nil {
		return
	}
	// Unlock, then close. Errors are intentionally ignored since this is
	// cleanup code -- the lock is released when the file is closed anyway.
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	_ = f.Close()
}
