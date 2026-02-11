// internal/did/keygen_windows.go
// NEWLY CREATED FILE - Windows-specific key file security

//go:build windows

package did

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// writeSecureFile writes a file with Windows ACLs restricting access to current user only.
func writeSecureFile(path string, data []byte) error {
	// First write the file with default permissions
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	// Then set strict ACLs
	if err := setOwnerOnlyACL(path); err != nil {
		// Log warning but don't fail - file is still written
		fmt.Fprintf(os.Stderr, "Warning: Could not set strict ACL on %s: %v\n", path, err)
		fmt.Fprintf(os.Stderr, "File may be accessible to other users. Consider manual ACL configuration.\n")
		// Return nil to not block functionality, but log the security issue
		return nil
	}

	return nil
}

// setOwnerOnlyACL sets Windows ACL to allow access only to the current user.
func setOwnerOnlyACL(path string) error {
	// Get current user SID
	token, err := syscall.OpenCurrentProcessToken()
	if err != nil {
		return fmt.Errorf("failed to open process token: %w", err)
	}
	defer token.Close()

	tokenUser, err := token.GetTokenUser()
	if err != nil {
		return fmt.Errorf("failed to get token user: %w", err)
	}

	userSID := tokenUser.User.Sid

	// Get file security descriptor
	pathUTF16, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("failed to convert path to UTF16: %w", err)
	}

	// Create new DACL with only owner access
	// ACE: Access Control Entry
	// DACL: Discretionary Access Control List

	const (
		// Access rights
		FILE_ALL_ACCESS = 0x1F01FF

		// ACE types
		ACCESS_ALLOWED_ACE_TYPE = 0x0

		// ACE flags
		CONTAINER_INHERIT_ACE = 0x2
		OBJECT_INHERIT_ACE    = 0x1
	)

	// Build ACE for owner with full access
	aceSize := uint16(unsafe.Sizeof(windows.ACCESS_ALLOWED_ACE{})) + uint16(windows.GetLengthSid(userSID)) - 4
	ace := &windows.ACCESS_ALLOWED_ACE{
		Header: windows.ACE_HEADER{
			AceType:  ACCESS_ALLOWED_ACE_TYPE,
			AceFlags: CONTAINER_INHERIT_ACE | OBJECT_INHERIT_ACE,
			AceSize:  aceSize,
		},
		Mask: FILE_ALL_ACCESS,
	}

	// Create new DACL
	var dacl *windows.ACL
	daclSize := uint32(unsafe.Sizeof(windows.ACL{})) + uint32(aceSize)
	daclBuffer := make([]byte, daclSize)
	dacl = (*windows.ACL)(unsafe.Pointer(&daclBuffer[0]))

	if err := windows.InitializeAcl(dacl, daclSize, windows.ACL_REVISION); err != nil {
		return fmt.Errorf("failed to initialize ACL: %w", err)
	}

	// Add ACE to DACL
	if err := windows.AddAccessAllowedAce(dacl, windows.ACL_REVISION, FILE_ALL_ACCESS, userSID); err != nil {
		return fmt.Errorf("failed to add ACE: %w", err)
	}

	// Create security descriptor
	var sd *windows.SECURITY_DESCRIPTOR
	sdBuffer := make([]byte, unsafe.Sizeof(windows.SECURITY_DESCRIPTOR{}))
	sd = (*windows.SECURITY_DESCRIPTOR)(unsafe.Pointer(&sdBuffer[0]))

	if err := windows.InitializeSecurityDescriptor(sd, windows.SECURITY_DESCRIPTOR_REVISION); err != nil {
		return fmt.Errorf("failed to initialize security descriptor: %w", err)
	}

	// Set DACL in security descriptor
	if err := windows.SetSecurityDescriptorDacl(sd, true, dacl, false); err != nil {
		return fmt.Errorf("failed to set DACL: %w", err)
	}

	// Apply security descriptor to file
	if err := windows.SetFileSecurity(
		pathUTF16,
		windows.DACL_SECURITY_INFORMATION,
		sd,
	); err != nil {
		return fmt.Errorf("failed to set file security: %w", err)
	}

	return nil
}

// readSecureFile reads a file (Windows version - same as Unix).
func readSecureFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
