//go:build linux

package compute

import (
	"context"
	"fmt"
	"os/exec"
	"syscall"
)

// configureIsolation applies Linux namespace isolation, UID/GID mappings,
// and resource limits to the given command. This is called by executeLinux
// in sandbox.go to enforce production-grade sandboxing on Linux hosts.
func configureIsolation(cmd *exec.Cmd, job ComputeJob) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUSER |
			syscall.CLONE_NEWNS |
			syscall.CLONE_NEWPID |
			syscall.CLONE_NEWNET |
			syscall.CLONE_NEWUTS |
			syscall.CLONE_NEWIPC,

		// Map container root (0) to host nobody (65534) so the
		// sandboxed process runs fully unprivileged on the host.
		UidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: 65534, Size: 1},
		},
		GidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: 65534, Size: 1},
		},

		// Ensure GID mapping is written without the setgroups helper
		// (required for unprivileged user namespaces).
		GidMappingsEnableSetgroups: false,
	}

	// --- resource limits ------------------------------------------------

	cpuSeconds := uint64(job.CPUSeconds)
	if cpuSeconds == 0 {
		cpuSeconds = 3600 // 1 hour default
	}

	memBytes := uint64(job.MemoryMB) * 1024 * 1024
	if memBytes == 0 {
		memBytes = 512 * 1024 * 1024 // 512 MB default
	}

	diskBytes := uint64(job.DiskMB) * 1024 * 1024
	if diskBytes == 0 {
		diskBytes = 1024 * 1024 * 1024 // 1 GB default
	}

	cmd.SysProcAttr.Rlimits = []syscall.Rlimit{
		// CPU time in seconds.
		{Type: syscall.RLIMIT_CPU, Cur: cpuSeconds, Max: cpuSeconds},
		// Virtual address space.
		{Type: syscall.RLIMIT_AS, Cur: memBytes, Max: memBytes},
		// Maximum file size the process may create.
		{Type: syscall.RLIMIT_FSIZE, Cur: diskBytes, Max: diskBytes},
		// Open file descriptors.
		{Type: syscall.RLIMIT_NOFILE, Cur: 64, Max: 64},
		// Maximum number of processes / threads.
		{Type: syscall.RLIMIT_NPROC, Cur: 16, Max: 16},
	}
}

// mountPrivate ensures the sandbox mount namespace does not propagate
// mount events back to the host.  Call this early inside the child
// (e.g. via a pre-exec callback) after CLONE_NEWNS has taken effect.
func mountPrivate() error {
	// MS_REC|MS_PRIVATE on "/" makes every mount in the namespace private.
	if err := syscall.Mount("none", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("mount private propagation: %w", err)
	}
	return nil
}

// executeLinux runs the job with full Linux namespace isolation,
// unprivileged user mapping, and per-process resource limits.
// This definition is only compiled on Linux (//go:build linux) and
// shadows the stub in sandbox.go once the build-tag split is applied.
//
// NOTE: To activate this method, the stub executeLinux in sandbox.go
// must be removed or gated behind //go:build !linux.  Until that
// refactor is done, sandbox.go can call configureIsolation(cmd, job)
// directly inside its own executeLinux body.
func (s *Sandbox) executeLinuxIsolated(ctx context.Context, job ComputeJob) (*ComputeResult, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, job.Timeout)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, job.Executable, job.Args...)
	cmd.Dir = s.workDir
	if job.WorkDir != "" {
		cmd.Dir = job.WorkDir
	}

	// Apply namespace isolation and resource limits.
	configureIsolation(cmd, job)

	stdout, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return &ComputeResult{
				ExitCode: exitErr.ExitCode(),
				Stdout:   stdout,
				Stderr:   exitErr.Stderr,
			}, nil
		}
		return nil, fmt.Errorf("sandbox execution failed: %w", err)
	}

	return &ComputeResult{
		ExitCode: 0,
		Stdout:   stdout,
	}, nil
}
