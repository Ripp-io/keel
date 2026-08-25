package globaldeps

import (
	"os"
	"path/filepath"
)

// BinDir returns the path to ~/.keel/bin/.
func BinDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".keel", "bin")
	}
	return filepath.Join(home, ".keel", "bin")
}
