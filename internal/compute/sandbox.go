package compute

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

// ComputeJob describes a job to be executed in a sandbox.
type ComputeJob struct {
	JobID         string
	TransactionID string
	UserDID       string
	ProviderDID   string
	Executable    string
	Args          []string
	CPUCores      int
	MemoryMB      int
	DiskMB        int
	CPUSeconds    int
	Timeout       time.Duration
	WorkDir       string
}

// ComputeResult holds the output of a completed compute job.
type ComputeResult struct {
	ExitCode    int
	Stdout      []byte
	Stderr      []byte
	CPUUsed     int64 // CPU seconds consumed
	MemoryPeak  int64 // Peak memory in MB
	ResultsPath string
}

// Sandbox executes compute jobs in an isolated environment.
type Sandbox struct {
	workDir string
}

// NewSandbox creates a new compute sandbox with the given work directory.
func NewSandbox(workDir string) *Sandbox {
	return &Sandbox{workDir: workDir}
}

// Execute runs a compute job in an isolated sandbox.
// On Linux, this uses namespaces, seccomp, and resource limits.
// On other platforms, it runs with basic process isolation.
func (s *Sandbox) Execute(ctx context.Context, job ComputeJob) (*ComputeResult, error) {
	if runtime.GOOS == "linux" {
		return s.executeLinux(ctx, job)
	}
	return s.executeFallback(ctx, job)
}

// executeLinux runs the job with Linux namespace isolation and rlimits.
// On Linux builds, configureIsolation (from sandbox_linux.go) applies
// CLONE_NEW* namespaces, UID/GID mappings, and per-process resource limits.
// On non-Linux builds, configureIsolation (from sandbox_other.go) is a no-op.
func (s *Sandbox) executeLinux(ctx context.Context, job ComputeJob) (*ComputeResult, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, job.Timeout)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, job.Executable, job.Args...)
	cmd.Dir = s.workDir
	if job.WorkDir != "" {
		cmd.Dir = job.WorkDir
	}

	// Apply platform-specific isolation (real on Linux, no-op elsewhere).
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

// executeFallback runs the job with basic process isolation (non-Linux platforms).
func (s *Sandbox) executeFallback(ctx context.Context, job ComputeJob) (*ComputeResult, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, job.Timeout)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, job.Executable, job.Args...)
	cmd.Dir = s.workDir

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
