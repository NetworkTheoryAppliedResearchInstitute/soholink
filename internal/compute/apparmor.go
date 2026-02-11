package compute

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// AppArmorProfile defines an AppArmor mandatory access control profile.
// AppArmor restricts programs' capabilities with per-program profiles.
type AppArmorProfile struct {
	// Name of the profile
	Name string

	// Flags for profile mode (complain or enforce)
	Flags []string

	// Capabilities allowed
	Capabilities []string

	// Network access rules
	Network []NetworkRule

	// File access rules
	Files []FileRule

	// Signal rules
	Signals []SignalRule

	// Ptrace rules
	Ptraces []PtraceRule

	// Mount rules
	Mounts []MountRule

	// Include other profiles
	Includes []string
}

// NetworkRule defines network access permissions.
type NetworkRule struct {
	// Family (inet, inet6, unix, etc.)
	Family string

	// Type (stream, dgram, etc.)
	Type string

	// Protocol (tcp, udp, etc.)
	Protocol string
}

// FileRule defines file access permissions.
type FileRule struct {
	// Path pattern (supports globs)
	Path string

	// Permissions (r, w, x, m, k, l, etc.)
	Permissions string
}

// SignalRule defines signal permissions.
type SignalRule struct {
	// Access (send, receive)
	Access string

	// Signal type (optional)
	Signal string

	// Peer profile (optional)
	Peer string
}

// PtraceRule defines ptrace permissions.
type PtraceRule struct {
	// Access (read, trace, tracedby)
	Access string

	// Peer profile (optional)
	Peer string
}

// MountRule defines mount permissions.
type MountRule struct {
	// Filesystem type
	FSType string

	// Source path
	Source string

	// Destination path
	Destination string

	// Options
	Options string
}

// DefaultAppArmorProfile returns a restrictive default profile.
func DefaultAppArmorProfile(name string) *AppArmorProfile {
	return &AppArmorProfile{
		Name:  name,
		Flags: []string{"complain"}, // Start in complain mode for testing
		Capabilities: []string{
			"net_bind_service",
			"setuid",
			"setgid",
		},
		Network: []NetworkRule{
			{Family: "inet", Type: "stream", Protocol: "tcp"},
			{Family: "inet", Type: "dgram", Protocol: "udp"},
			{Family: "inet6", Type: "stream", Protocol: "tcp"},
			{Family: "inet6", Type: "dgram", Protocol: "udp"},
			{Family: "unix", Type: "stream"},
		},
		Files: []FileRule{
			// Standard library paths
			{Path: "/lib/**", Permissions: "r"},
			{Path: "/usr/lib/**", Permissions: "r"},
			// Temporary files
			{Path: "/tmp/**", Permissions: "rw"},
			// Application data (adjust as needed)
			{Path: "/var/lib/**", Permissions: "rw"},
			// Proc filesystem (limited)
			{Path: "/proc/*/stat", Permissions: "r"},
			{Path: "/proc/*/status", Permissions: "r"},
			{Path: "/proc/sys/kernel/hostname", Permissions: "r"},
		},
		Includes: []string{
			"<abstractions/base>",
		},
	}
}

// WebServerAppArmorProfile returns a profile for web servers.
func WebServerAppArmorProfile(name string) *AppArmorProfile{
	profile := DefaultAppArmorProfile(name)

	// Add web server specific permissions
	profile.Capabilities = append(profile.Capabilities, "chown", "dac_override")

	profile.Files = append(profile.Files,
		FileRule{Path: "/var/www/**", Permissions: "r"},
		FileRule{Path: "/etc/nginx/**", Permissions: "r"},
		FileRule{Path: "/var/log/nginx/**", Permissions: "w"},
	)

	return profile
}

// DatabaseAppArmorProfile returns a profile for database servers.
func DatabaseAppArmorProfile(name string) *AppArmorProfile {
	profile := DefaultAppArmorProfile(name)

	// Add database specific permissions
	profile.Capabilities = append(profile.Capabilities, "ipc_lock", "sys_resource")

	profile.Files = append(profile.Files,
		FileRule{Path: "/var/lib/postgresql/**", Permissions: "rwk"},
		FileRule{Path: "/var/lib/mysql/**", Permissions: "rwk"},
		FileRule{Path: "/var/lib/mongodb/**", Permissions: "rwk"},
		FileRule{Path: "/var/lib/redis/**", Permissions: "rwk"},
		FileRule{Path: "/run/postgresql/**", Permissions: "rwk"},
		FileRule{Path: "/run/mysqld/**", Permissions: "rwk"},
	)

	return profile
}

// DockerContainerAppArmorProfile returns a profile for Docker containers.
func DockerContainerAppArmorProfile(name string) *AppArmorProfile {
	profile := &AppArmorProfile{
		Name:  name,
		Flags: []string{},
		Capabilities: []string{
			"chown",
			"dac_override",
			"fowner",
			"fsetid",
			"kill",
			"setgid",
			"setuid",
			"setpcap",
			"net_bind_service",
			"net_raw",
			"sys_chroot",
			"mknod",
			"audit_write",
			"setfcap",
		},
		Network: []NetworkRule{
			{Family: "inet"},
			{Family: "inet6"},
			{Family: "unix"},
		},
		Files: []FileRule{
			// Container root filesystem
			{Path: "/**", Permissions: "rwl"},
			// Deny access to host-specific paths
			{Path: "/proc/sys/**", Permissions: ""},  // Deny
			{Path: "/proc/sysrq-trigger", Permissions: ""},
			{Path: "/proc/kcore", Permissions: ""},
			{Path: "/sys/**", Permissions: ""},
		},
		Signals: []SignalRule{
			{Access: "send"},
			{Access: "receive"},
		},
		Ptraces: []PtraceRule{
			{Access: "read"},
		},
		Mounts: []MountRule{},
		Includes: []string{
			"<abstractions/base>",
		},
	}

	return profile
}

// GenerateProfile generates the AppArmor profile text.
func (p *AppArmorProfile) GenerateProfile() string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("#include <tunables/global>\n\n"))
	sb.WriteString(fmt.Sprintf("profile %s ", p.Name))

	if len(p.Flags) > 0 {
		sb.WriteString(fmt.Sprintf("flags=(%s) ", strings.Join(p.Flags, ",")))
	}

	sb.WriteString("{\n")

	// Includes
	for _, inc := range p.Includes {
		sb.WriteString(fmt.Sprintf("  #include %s\n", inc))
	}

	if len(p.Includes) > 0 {
		sb.WriteString("\n")
	}

	// Capabilities
	if len(p.Capabilities) > 0 {
		sb.WriteString("  # Capabilities\n")
		for _, cap := range p.Capabilities {
			sb.WriteString(fmt.Sprintf("  capability %s,\n", cap))
		}
		sb.WriteString("\n")
	}

	// Network rules
	if len(p.Network) > 0 {
		sb.WriteString("  # Network access\n")
		for _, net := range p.Network {
			rule := "  network"
			if net.Family != "" {
				rule += " " + net.Family
			}
			if net.Type != "" {
				rule += " " + net.Type
			}
			if net.Protocol != "" {
				rule += " " + net.Protocol
			}
			sb.WriteString(rule + ",\n")
		}
		sb.WriteString("\n")
	}

	// File rules
	if len(p.Files) > 0 {
		sb.WriteString("  # File access\n")
		for _, file := range p.Files {
			if file.Permissions == "" {
				// Deny rule
				sb.WriteString(fmt.Sprintf("  deny %s,\n", file.Path))
			} else {
				sb.WriteString(fmt.Sprintf("  %s %s,\n", file.Path, file.Permissions))
			}
		}
		sb.WriteString("\n")
	}

	// Signal rules
	if len(p.Signals) > 0 {
		sb.WriteString("  # Signal access\n")
		for _, sig := range p.Signals {
			rule := fmt.Sprintf("  signal (%s)", sig.Access)
			if sig.Signal != "" {
				rule += fmt.Sprintf(" set=(%s)", sig.Signal)
			}
			if sig.Peer != "" {
				rule += fmt.Sprintf(" peer=%s", sig.Peer)
			}
			sb.WriteString(rule + ",\n")
		}
		sb.WriteString("\n")
	}

	// Ptrace rules
	if len(p.Ptraces) > 0 {
		sb.WriteString("  # Ptrace access\n")
		for _, ptr := range p.Ptraces {
			rule := fmt.Sprintf("  ptrace (%s)", ptr.Access)
			if ptr.Peer != "" {
				rule += fmt.Sprintf(" peer=%s", ptr.Peer)
			}
			sb.WriteString(rule + ",\n")
		}
		sb.WriteString("\n")
	}

	// Mount rules
	if len(p.Mounts) > 0 {
		sb.WriteString("  # Mount access\n")
		for _, mnt := range p.Mounts {
			rule := "  mount"
			if mnt.FSType != "" {
				rule += fmt.Sprintf(" fstype=%s", mnt.FSType)
			}
			if mnt.Source != "" {
				rule += fmt.Sprintf(" %s ->", mnt.Source)
			}
			if mnt.Destination != "" {
				rule += fmt.Sprintf(" %s", mnt.Destination)
			}
			if mnt.Options != "" {
				rule += fmt.Sprintf(" options=(%s)", mnt.Options)
			}
			sb.WriteString(rule + ",\n")
		}
		sb.WriteString("\n")
	}

	sb.WriteString("}\n")

	return sb.String()
}

// SaveProfile writes the profile to the AppArmor profiles directory.
func (p *AppArmorProfile) SaveProfile() error {
	profilePath := filepath.Join("/etc/apparmor.d", p.Name)

	profileText := p.GenerateProfile()

	if err := os.WriteFile(profilePath, []byte(profileText), 0644); err != nil {
		return fmt.Errorf("failed to write profile: %w", err)
	}

	return nil
}

// LoadProfile loads the profile into the kernel.
func (p *AppArmorProfile) LoadProfile() error {
	// First save the profile
	if err := p.SaveProfile(); err != nil {
		return err
	}

	// Load into kernel using apparmor_parser
	cmd := exec.Command("apparmor_parser", "-r", filepath.Join("/etc/apparmor.d", p.Name))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to load profile: %w", err)
	}

	return nil
}

// UnloadProfile removes the profile from the kernel.
func (p *AppArmorProfile) UnloadProfile() error {
	cmd := exec.Command("apparmor_parser", "-R", filepath.Join("/etc/apparmor.d", p.Name))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to unload profile: %w", err)
	}

	return nil
}

// SetEnforceMode puts the profile in enforce mode.
func (p *AppArmorProfile) SetEnforceMode() error {
	cmd := exec.Command("aa-enforce", p.Name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set enforce mode: %w", err)
	}

	return nil
}

// SetComplainMode puts the profile in complain mode.
func (p *AppArmorProfile) SetComplainMode() error {
	cmd := exec.Command("aa-complain", p.Name)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set complain mode: %w", err)
	}

	return nil
}

// GetStatus returns the current status of the profile.
func (p *AppArmorProfile) GetStatus() (string, error) {
	cmd := exec.Command("aa-status", "--json")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get status: %w", err)
	}

	// In production, parse JSON output
	// For now, return raw output
	return string(output), nil
}

// IsLoaded checks if the profile is currently loaded.
func (p *AppArmorProfile) IsLoaded() (bool, error) {
	// First check if AppArmor is enabled
	cmd := exec.Command("aa-status", "--enabled")
	if err := cmd.Run(); err != nil {
		return false, nil
	}

	// Get full status output WITHOUT using shell
	// This prevents command injection via profile names
	cmd = exec.Command("aa-status")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to get aa-status output: %w", err)
	}

	// Search for profile name in output using Go strings (NOT grep!)
	// This is safe because we're not executing shell commands with user input
	return strings.Contains(string(output), p.Name), nil
}
