//go:build !linux

package compute

import "os/exec"

// configureIsolation is a no-op on non-Linux platforms.
// On Linux, the real implementation in sandbox_linux.go applies
// namespace isolation, UID/GID mappings, and resource limits.
func configureIsolation(_ *exec.Cmd, _ ComputeJob) {}
