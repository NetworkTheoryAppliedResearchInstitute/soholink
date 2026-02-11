//go:build !windows

package did

import (
	"os"
)

// writeSecureFile writes a file with Unix permissions (0600 = owner read/write only).
// This prevents other users from reading private keys.
func writeSecureFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0600)
}

// readSecureFile reads a file.
func readSecureFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
