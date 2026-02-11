// internal/validation/paths.go
// NEWLY CREATED FILE - Add to project

package validation

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidatePath ensures a path is safe and within the allowed base directory.
// Prevents path traversal attacks like "../../../etc/shadow"
func ValidatePath(basePath, userPath string) (string, error) {
	// Clean the path to remove . and .. components
	cleanPath := filepath.Clean(userPath)

	// Check for obvious traversal attempts
	if strings.Contains(cleanPath, "..") {
		return "", fmt.Errorf("path traversal detected: %s", userPath)
	}

	// If it's an absolute path, reject it (unless it starts with basePath)
	if filepath.IsAbs(cleanPath) {
		if !strings.HasPrefix(cleanPath, basePath) {
			return "", fmt.Errorf("absolute path outside base directory: %s", userPath)
		}
		return cleanPath, nil
	}

	// Join with base path
	fullPath := filepath.Join(basePath, cleanPath)

	// Ensure the resulting path is still within base directory
	// This handles tricks like "foo/../../etc/shadow"
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return "", fmt.Errorf("invalid base path: %w", err)
	}

	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	// Verify the absolute path starts with the base path
	if !strings.HasPrefix(absPath, absBase) {
		return "", fmt.Errorf("path escapes base directory: %s", userPath)
	}

	return absPath, nil
}

// ValidateFilename ensures a filename is safe (no path components).
// Use this for cases where only a filename is expected, not a path.
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
func SecureJoin(base string, parts ...string) (string, error) {
	result := base
	for _, part := range parts {
		validated, err := ValidatePath(result, part)
		if err != nil {
			return "", err
		}
		result = validated
	}
	return result, nil
}
