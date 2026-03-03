//go:build windows

package did

import (
	"fmt"
	"os"
	"os/exec"
)

// writeSecureFile writes a file with Windows ACLs restricting access to current user only.
// On Windows, Unix permissions (0600) are ignored, so we must use ACLs via icacls.
func writeSecureFile(path string, data []byte) error {
	// First write the file with default permissions
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Use icacls to set owner-only permissions
	// This is simpler and more reliable than using Windows API directly
	if err := setOwnerOnlyACL(path); err != nil {
		// Log warning but don't fail - file is still written
		fmt.Fprintf(os.Stderr, "Warning: Could not set strict ACL on %s: %v\n", path, err)
		fmt.Fprintf(os.Stderr, "File may be accessible to other users. Consider manual ACL configuration with: icacls \"%s\" /inheritance:r /grant:r \"%%USERNAME%%\":F\n", path)
		// Return nil to not block functionality, but user is warned
		return nil
	}

	return nil
}

// setOwnerOnlyACL uses icacls to set file permissions to owner-only.
//
// The command we run is:
//   icacls "filepath" /inheritance:r /grant:r "%USERNAME%":F
//
// This:
//   /inheritance:r  - Removes inherited permissions
//   /grant:r        - Replaces permissions (not adds)
//   "%USERNAME%":F  - Grants Full control to current user only
func setOwnerOnlyACL(path string) error {
	// Build icacls command
	// We need to grant full control to current user and remove all others
	cmd := exec.Command("icacls", path, "/inheritance:r", "/grant:r", fmt.Sprintf("%s:F", os.Getenv("USERNAME"))) // #nosec G702 -- path is our keygen file path; icacls args are static flags; USERNAME is Windows system env var

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("icacls failed: %w (output: %s)", err, string(output))
	}

	return nil
}

// readSecureFile reads a file (Windows version - same as Unix).
func readSecureFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
