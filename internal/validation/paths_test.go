package validation

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatePath_Traversal(t *testing.T) {
	base := "/var/lib/soholink"

	tests := []struct {
		name      string
		userPath  string
		shouldErr bool
	}{
		// Valid paths
		{"valid relative", "keys/alice.key", false},
		{"valid subdirectory", "users/bob/data", false},
		{"valid single file", "config.yaml", false},
		{"valid deep path", "a/b/c/d/e/file.txt", false},

		// Invalid - traversal attempts
		{"traversal parent", "../../../etc/shadow", true},
		{"traversal mixed", "foo/../../etc/passwd", true},
		{"traversal relative", "..", true},
		{"traversal multiple", "../../..", true},
		{"traversal windows style", "..\\..\\Windows\\System32", true},

		// Invalid - absolute paths outside base
		{"absolute outside", "/etc/shadow", true},
		{"absolute outside 2", "/root/.ssh/id_rsa", true},

		// Valid - absolute paths inside base
		{"absolute inside", "/var/lib/soholink/keys/test", false},

		// Edge cases
		{"empty", "", true},
		{"dot", ".", false}, // Current dir is OK
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidatePath(base, tt.userPath)

			if tt.shouldErr && err == nil {
				t.Errorf("Expected error for path: %s, but got none (result: %s)", tt.userPath, result)
			}

			if !tt.shouldErr && err != nil {
				t.Errorf("Unexpected error for valid path %s: %v", tt.userPath, err)
			}

			if !tt.shouldErr && err == nil {
				// Valid paths should be within base
				absBase, _ := filepath.Abs(base)
				if !strings.HasPrefix(result, absBase) {
					t.Errorf("Valid path %s resulted in %s outside base %s", tt.userPath, result, base)
				}
			}
		})
	}
}

func TestValidatePath_WindowsTraversal(t *testing.T) {
	if filepath.Separator != '\\' {
		t.Skip("Windows-specific test")
	}

	base := `C:\ProgramData\SoHoLINK`

	tests := []struct {
		name      string
		userPath  string
		shouldErr bool
	}{
		{"valid windows path", `keys\alice.key`, false},
		{"traversal windows", `..\..\..\Windows\System32`, true},
		{"absolute outside", `C:\Windows\System32`, true},
		{"absolute inside", `C:\ProgramData\SoHoLINK\keys\test`, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ValidatePath(base, tt.userPath)

			if tt.shouldErr && err == nil {
				t.Errorf("Expected error for path: %s, but got none (result: %s)", tt.userPath, result)
			}

			if !tt.shouldErr && err != nil {
				t.Errorf("Unexpected error for valid path %s: %v", tt.userPath, err)
			}
		})
	}
}

func TestValidateFilename(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		shouldErr bool
	}{
		// Valid filenames
		{"simple file", "alice.key", false},
		{"with extension", "config.yaml", false},
		{"with underscore", "my_file.txt", false},
		{"with dash", "my-file.txt", false},
		{"with numbers", "file123.dat", false},

		// Invalid - path components
		{"with slash", "foo/bar.txt", true},
		{"with backslash", "foo\\bar.txt", true},
		{"traversal", "../passwd", true},
		{"parent dir", "..", true},

		// Edge cases
		{"empty", "", true},
		{"dot only", ".", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilename(tt.filename)

			if tt.shouldErr && err == nil {
				t.Errorf("Expected error for filename: %s, but got none", tt.filename)
			}

			if !tt.shouldErr && err != nil {
				t.Errorf("Unexpected error for valid filename %s: %v", tt.filename, err)
			}
		})
	}
}

func TestSecureJoin(t *testing.T) {
	base := "/var/lib/soholink"

	tests := []struct {
		name      string
		parts     []string
		shouldErr bool
		contains  string // substring that should be in result
	}{
		{
			name:     "valid join",
			parts:    []string{"keys", "alice.key"},
			contains: "keys/alice.key",
		},
		{
			name:     "valid deep join",
			parts:    []string{"users", "bob", "data", "file.txt"},
			contains: "users/bob/data/file.txt",
		},
		{
			name:      "invalid traversal",
			parts:     []string{"..", "etc", "passwd"},
			shouldErr: true,
		},
		{
			name:      "invalid traversal middle",
			parts:     []string{"keys", "..", "..", "etc", "passwd"},
			shouldErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := SecureJoin(base, tt.parts...)

			if tt.shouldErr && err == nil {
				t.Errorf("Expected error for parts: %v, but got none (result: %s)", tt.parts, result)
			}

			if !tt.shouldErr && err != nil {
				t.Errorf("Unexpected error for valid parts %v: %v", tt.parts, err)
			}

			if !tt.shouldErr && err == nil && tt.contains != "" {
				if !strings.Contains(result, filepath.FromSlash(tt.contains)) {
					t.Errorf("Result %s does not contain expected substring %s", result, tt.contains)
				}
			}
		})
	}
}

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name      string
		username  string
		shouldErr bool
	}{
		// Valid usernames
		{"simple", "alice", false},
		{"with number", "alice123", false},
		{"with underscore", "alice_bob", false},
		{"with dash", "alice-bob", false},
		{"min length", "abc", false},
		{"max length", "abcdefghijklmnopqrstuvwxyz123456", false}, // 32 chars

		// Invalid
		{"too short", "ab", true},
		{"too long", "abcdefghijklmnopqrstuvwxyz1234567", true}, // 33 chars
		{"uppercase", "Alice", true},
		{"special char", "alice!", true},
		{"space", "alice bob", true},
		{"slash", "alice/bob", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateUsername(tt.username)

			if tt.shouldErr && err == nil {
				t.Errorf("Expected error for username: %s, but got none", tt.username)
			}

			if !tt.shouldErr && err != nil {
				t.Errorf("Unexpected error for valid username %s: %v", tt.username, err)
			}
		})
	}
}

func TestValidateDID(t *testing.T) {
	tests := []struct {
		name      string
		did       string
		shouldErr bool
	}{
		// Valid DIDs
		{"valid short", "did:soho:z6Mkp123456789", false},
		{"valid long", "did:soho:z6MkpTHR2PyqcL1UvJMXPWGJ3R8EpLqCa9oJrKLEZ", false},

		// Invalid
		{"wrong prefix", "did:key:z6Mkp123", true},
		{"no prefix", "z6Mkp123", true},
		{"too short", "did:soho:abc", true},
		{"wrong separator", "did-soho-z6Mkp123", true},
		{"extra parts", "did:soho:z6Mkp123:extra", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDID(tt.did)

			if tt.shouldErr && err == nil {
				t.Errorf("Expected error for DID: %s, but got none", tt.did)
			}

			if !tt.shouldErr && err != nil {
				t.Errorf("Unexpected error for valid DID %s: %v", tt.did, err)
			}
		})
	}
}

// Benchmark path validation performance
func BenchmarkValidatePath(b *testing.B) {
	base := "/var/lib/soholink"
	path := "keys/users/alice/data.key"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ValidatePath(base, path)
	}
}

func BenchmarkValidateFilename(b *testing.B) {
	filename := "alice.key"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateFilename(filename)
	}
}
