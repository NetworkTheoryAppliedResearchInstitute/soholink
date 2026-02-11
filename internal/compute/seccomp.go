package compute

import (
	"encoding/json"
	"fmt"
	"os"
)

// SeccompProfile defines a seccomp (Secure Computing Mode) filter.
// Seccomp restricts the system calls a process can make, providing
// defense-in-depth against kernel vulnerabilities.
type SeccompProfile struct {
	// DefaultAction is the action to take for syscalls not in the rules
	DefaultAction SeccompAction `json:"defaultAction"`

	// Architectures specifies which architectures this profile applies to
	Architectures []string `json:"architectures"`

	// Syscalls defines the syscall filtering rules
	Syscalls []SeccompSyscall `json:"syscalls"`
}

// SeccompAction defines what to do with a syscall.
type SeccompAction string

const (
	// SeccompActionAllow permits the syscall
	SeccompActionAllow SeccompAction = "SCMP_ACT_ALLOW"

	// SeccompActionErrno returns an error to the caller
	SeccompActionErrno SeccompAction = "SCMP_ACT_ERRNO"

	// SeccompActionKill kills the process
	SeccompActionKill SeccompAction = "SCMP_ACT_KILL"

	// SeccompActionLog logs the syscall but allows it
	SeccompActionLog SeccompAction = "SCMP_ACT_LOG"

	// SeccompActionTrace notifies a tracer
	SeccompActionTrace SeccompAction = "SCMP_ACT_TRACE"
)

// SeccompSyscall defines a rule for specific syscalls.
type SeccompSyscall struct {
	// Names of syscalls this rule applies to
	Names []string `json:"names"`

	// Action to take for these syscalls
	Action SeccompAction `json:"action"`

	// Args defines argument-based filtering (optional)
	Args []SeccompArg `json:"args,omitempty"`
}

// SeccompArg defines filtering based on syscall arguments.
type SeccompArg struct {
	// Index of the argument (0-5)
	Index uint `json:"index"`

	// Value to compare against
	Value uint64 `json:"value"`

	// Operator for comparison
	Op SeccompOperator `json:"op"`
}

// SeccompOperator defines how to compare argument values.
type SeccompOperator string

const (
	SeccompOpEQ SeccompOperator = "SCMP_CMP_EQ" // Equal
	SeccompOpNE SeccompOperator = "SCMP_CMP_NE" // Not equal
	SeccompOpLT SeccompOperator = "SCMP_CMP_LT" // Less than
	SeccompOpLE SeccompOperator = "SCMP_CMP_LE" // Less than or equal
	SeccompOpGT SeccompOperator = "SCMP_CMP_GT" // Greater than
	SeccompOpGE SeccompOperator = "SCMP_CMP_GE" // Greater than or equal
)

// DefaultSeccompProfile returns a restrictive default profile.
func DefaultSeccompProfile() *SeccompProfile {
	return &SeccompProfile{
		DefaultAction: SeccompActionErrno,
		Architectures: []string{
			"SCMP_ARCH_X86_64",
			"SCMP_ARCH_X86",
			"SCMP_ARCH_AARCH64",
		},
		Syscalls: []SeccompSyscall{
			// Allow basic I/O
			{
				Names:  []string{"read", "write", "readv", "writev", "pread64", "pwrite64"},
				Action: SeccompActionAllow,
			},
			// Allow file operations
			{
				Names:  []string{"open", "openat", "close", "stat", "fstat", "lstat", "access", "faccessat"},
				Action: SeccompActionAllow,
			},
			// Allow memory management
			{
				Names:  []string{"mmap", "munmap", "mprotect", "brk", "mremap", "madvise"},
				Action: SeccompActionAllow,
			},
			// Allow process management
			{
				Names:  []string{"clone", "fork", "vfork", "execve", "exit", "exit_group", "wait4", "waitpid"},
				Action: SeccompActionAllow,
			},
			// Allow networking
			{
				Names:  []string{"socket", "bind", "connect", "listen", "accept", "accept4", "sendto", "recvfrom", "sendmsg", "recvmsg"},
				Action: SeccompActionAllow,
			},
			// Allow time operations
			{
				Names:  []string{"gettimeofday", "clock_gettime", "time", "nanosleep"},
				Action: SeccompActionAllow,
			},
			// Allow signal handling
			{
				Names:  []string{"rt_sigaction", "rt_sigprocmask", "rt_sigreturn", "sigaltstack"},
				Action: SeccompActionAllow,
			},
			// Allow thread operations
			{
				Names:  []string{"futex", "set_robust_list", "get_robust_list"},
				Action: SeccompActionAllow,
			},
		},
	}
}

// WebServerSeccompProfile returns a profile for web servers.
func WebServerSeccompProfile() *SeccompProfile {
	profile := DefaultSeccompProfile()

	// Add additional syscalls needed for web servers
	profile.Syscalls = append(profile.Syscalls,
		SeccompSyscall{
			Names:  []string{"epoll_create", "epoll_create1", "epoll_ctl", "epoll_wait", "epoll_pwait"},
			Action: SeccompActionAllow,
		},
		SeccompSyscall{
			Names:  []string{"poll", "ppoll", "select", "pselect6"},
			Action: SeccompActionAllow,
		},
		SeccompSyscall{
			Names:  []string{"setsockopt", "getsockopt", "shutdown"},
			Action: SeccompActionAllow,
		},
	)

	return profile
}

// DatabaseSeccompProfile returns a profile for database servers.
func DatabaseSeccompProfile() *SeccompProfile {
	profile := DefaultSeccompProfile()

	// Add syscalls for databases
	profile.Syscalls = append(profile.Syscalls,
		SeccompSyscall{
			Names:  []string{"fdatasync", "fsync", "sync", "syncfs"},
			Action: SeccompActionAllow,
		},
		SeccompSyscall{
			Names:  []string{"flock", "fcntl"},
			Action: SeccompActionAllow,
		},
		SeccompSyscall{
			Names:  []string{"fallocate", "ftruncate", "truncate"},
			Action: SeccompActionAllow,
		},
	)

	return profile
}

// RestrictiveSeccompProfile returns a highly restrictive profile.
// This denies most dangerous syscalls including kernel modules, debugging, etc.
func RestrictiveSeccompProfile() *SeccompProfile {
	profile := DefaultSeccompProfile()

	// Explicitly block dangerous syscalls
	profile.Syscalls = append(profile.Syscalls,
		SeccompSyscall{
			Names: []string{
				// Kernel module operations
				"init_module", "finit_module", "delete_module",
				// Raw I/O
				"ioperm", "iopl",
				// System configuration
				"syslog", "kexec_load", "kexec_file_load",
				// Debugging/tracing
				"ptrace", "process_vm_readv", "process_vm_writev",
				// Privileged operations
				"reboot", "swapon", "swapoff", "mount", "umount", "umount2",
				// Time manipulation
				"settimeofday", "clock_settime", "adjtimex",
				// User/group changes
				"setuid", "setgid", "setreuid", "setregid", "setresuid", "setresgid",
				// Capabilities
				"capset",
				// Kernel keyring
				"add_key", "request_key", "keyctl",
				// BPF
				"bpf",
				// Performance counters
				"perf_event_open",
			},
			Action: SeccompActionKill,
		},
	)

	return profile
}

// SaveSeccompProfile writes a seccomp profile to a JSON file.
func SaveSeccompProfile(profile *SeccompProfile, path string) error {
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal profile: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write profile: %w", err)
	}

	return nil
}

// LoadSeccompProfile reads a seccomp profile from a JSON file.
func LoadSeccompProfile(path string) (*SeccompProfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read profile: %w", err)
	}

	var profile SeccompProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("failed to unmarshal profile: %w", err)
	}

	return &profile, nil
}

// ApplySeccompProfile applies a seccomp profile to a container.
// This generates the appropriate Docker seccomp configuration.
func ApplySeccompProfile(profile *SeccompProfile) (map[string]interface{}, error) {
	// Convert to Docker seccomp format
	dockerProfile := map[string]interface{}{
		"defaultAction": string(profile.DefaultAction),
		"architectures": profile.Architectures,
		"syscalls":      make([]map[string]interface{}, 0),
	}

	for _, syscall := range profile.Syscalls {
		sc := map[string]interface{}{
			"names":  syscall.Names,
			"action": string(syscall.Action),
		}

		if len(syscall.Args) > 0 {
			args := make([]map[string]interface{}, 0)
			for _, arg := range syscall.Args {
				args = append(args, map[string]interface{}{
					"index": arg.Index,
					"value": arg.Value,
					"op":    string(arg.Op),
				})
			}
			sc["args"] = args
		}

		dockerProfile["syscalls"] = append(dockerProfile["syscalls"].([]map[string]interface{}), sc)
	}

	return dockerProfile, nil
}
