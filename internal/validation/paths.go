// Package validation provides input validation and sanitization functions.
package validation

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidatePath ensures a path is safe and within the allowed base directory.
// Prevents path traversal attacks like "../../../etc/shadow"
//
// Parameters:
//   basePath: The allowed base directory (e.g., "/var/lib/soholink")
//   userPath: The user-provided path (e.g., "keys/alice.key" or "../../../etc/passwd")
//
// Returns:
//   The absolute safe path within basePath, or an error if path traversal is detected.
//
// Example:
//   safePath, err := ValidatePath("/var/lib/soholink", "keys/alice.key")
//   // Returns: "/var/lib/soholink/keys/alice.key", nil
//
//   safePath, err := ValidatePath("/var/lib/soholink", "../../../etc/passwd")
//   // Returns: "", error("path traversal detected")
func ValidatePath(basePath, userPath string) (string, error) {
	// Reject empty paths
	if userPath == "" {
		return "", fmt.Errorf("empty path not allowed")
	}

	// Clean the path to remove . and .. components
	cleanPath := filepath.Clean(userPath)

	// Check for obvious traversal attempts in the cleaned path
	if strings.Contains(cleanPath, "..") {
		return "", fmt.Errorf("path traversal detected: %s", userPath)
	}

	// Get absolute version of base path for comparison
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return "", fmt.Errorf("invalid base path: %w", err)
	}

	// If it's an absolute path, verify it starts with basePath
	if filepath.IsAbs(cleanPath) {
		// Convert to absolute to handle cross-platform issues
		// On Windows, Unix paths like "/etc/passwd" become "C:\etc\passwd"
		absClean, err := filepath.Abs(cleanPath)
		if err != nil {
			return "", fmt.Errorf("invalid path: %w", err)
		}

		// Ensure the absolute path is within the base directory
		if !strings.HasPrefix(absClean, absBase+string(filepath.Separator)) &&
			absClean != absBase {
			return "", fmt.Errorf("absolute path outside base directory: %s", userPath)
		}
		return absClean, nil
	}

	// Join with base path (userPath is relative)
	fullPath := filepath.Join(basePath, cleanPath)

	// Ensure the resulting path is still within base directory
	// This handles tricks like "foo/../../etc/shadow"
	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Verify the absolute path starts with the base path
	// Add separator to prevent "/var/lib/soholink-evil" matching "/var/lib/soholink"
	if !strings.HasPrefix(absPath, absBase+string(filepath.Separator)) &&
		absPath != absBase {
		return "", fmt.Errorf("path escapes base directory: %s", userPath)
	}

	return absPath, nil
}

// ValidateFilename ensures a filename is safe (no path components).
// Use this for cases where only a filename is expected, not a path.
//
// Example:
//   err := ValidateFilename("alice.key")  // OK
//   err := ValidateFilename("../passwd")  // Error: traversal
//   err := ValidateFilename("foo/bar")    // Error: contains separator
func ValidateFilename(filename string) error {
	// Clean the filename
	clean := filepath.Clean(filename)

	// Ensure it doesn't contain directory separators
	if strings.Contains(clean, string(filepath.Separator)) {
		return fmt.Errorf("filename contains path separator: %s", filename)
	}

	// Ensure it doesn't contain traversal
	if strings.Contains(clean, "..") {
		return fmt.Errorf("filename contains traversal: %s", filename)
	}

	// Ensure it's not empty
	if clean == "" || clean == "." {
		return fmt.Errorf("filename is empty or invalid: %s", filename)
	}

	return nil
}

// SecureJoin safely joins path components, preventing traversal.
// Similar to filepath.Join but with safety checks.
//
// Example:
//   path, err := SecureJoin("/var/lib/soholink", "keys", "alice.key")
//   // Returns: "/var/lib/soholink/keys/alice.key", nil
//
//   path, err := SecureJoin("/var/lib/soholink", "..", "etc", "passwd")
//   // Returns: "", error("path traversal detected")
func SecureJoin(base string, parts ...string) (string, error) {
	result := base
	for i, part := range parts {
		validated, err := ValidatePath(result, part)
		if err != nil {
			return "", fmt.Errorf("invalid path component %d (%q): %w", i, part, err)
		}
		result = validated
	}
	return result, nil
}

// ValidateUsername ensures a username follows safe conventions.
// Usernames must be 3-32 characters, lowercase alphanumeric with underscores/hyphens.
func ValidateUsername(username string) error {
	if len(username) < 3 || len(username) > 32 {
		return fmt.Errorf("username must be 3-32 characters: %s", username)
	}

	for _, c := range username {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '-') {
			return fmt.Errorf("username contains invalid character %q: %s", c, username)
		}
	}

	return nil
}

// ValidateDID ensures a DID follows the did:soho: format.
// Expected format: did:soho:z6Mkp... (43 characters after z6)
func ValidateDID(did string) error {
	if !strings.HasPrefix(did, "did:soho:") {
		return fmt.Errorf("DID must start with 'did:soho:': %s", did)
	}

	parts := strings.Split(did, ":")
	if len(parts) != 3 {
		return fmt.Errorf("invalid DID format (expected 3 parts): %s", did)
	}

	identifier := parts[2]
	if len(identifier) < 10 {
		return fmt.Errorf("DID identifier too short: %s", did)
	}

	return nil
}
