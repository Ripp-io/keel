//go:build !unix

package remote

import "os"

// AcquireLock is a no-op on non-Unix platforms.
func AcquireLock(path string) (*os.File, error) {
	return nil, nil
}

// ReleaseLock is a no-op on non-Unix platforms.
func ReleaseLock(f *os.File) {
}
