//go:build !unix

package globaldeps

import "os"

// lockManifest is a no-op on non-Unix platforms.
func lockManifest(manifestPath string) (*os.File, error) {
	return nil, nil
}

// unlockManifest is a no-op on non-Unix platforms.
func unlockManifest(f *os.File) {
}
