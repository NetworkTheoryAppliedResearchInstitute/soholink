// Package wasm provides a portable WebAssembly task executor for SoHoLINK.
//
// The Executor interface abstracts over the underlying Wasm runtime so that
// the scheduler and mobile-node code are decoupled from the concrete engine.
// The default implementation is a stub that returns ErrNotImplemented; replace
// it with a wazero-backed executor once github.com/tetratelabs/wazero is added
// as a dependency (planned for v0.3.0).
//
// Usage:
//
//	exec := wasm.NewStubExecutor()  // or wasm.NewWazeroExecutor() in v0.3
//	out, err := exec.Execute(ctx, moduleBytes, inputBytes)
package wasm

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrTimeout is wrapping-sentinel returned by WithTimeout when the execution
// deadline fires.  Callers can detect this with errors.Is(err, ErrTimeout).
var ErrTimeout = errors.New("wasm: execution timed out")

// ErrNotImplemented is returned by executor stubs that have not yet been wired
// to a real Wasm runtime.
var ErrNotImplemented = errors.New("wasm: executor not yet implemented (planned v0.3)")

// ExecuteResult holds the output of a successful Wasm task execution.
type ExecuteResult struct {
	// Output is the raw bytes written to the executor's output buffer.
	Output []byte

	// ResultHash is the hex-encoded SHA-256 of Output.  The coordinator uses
	// this to verify shadow-replica results before settling the HTLC payment.
	ResultHash string

	// DurationMs is the wall-clock execution time in milliseconds.
	DurationMs int64
}

// Executor is the interface implemented by all Wasm task runners.
// A single Executor instance may be called concurrently from multiple
// goroutines, but each call receives its own sandboxed module instance.
type Executor interface {
	// Execute instantiates the given Wasm module bytes, calls its "_start"
	// (WASI) entry-point with the provided input bytes available on stdin,
	// and returns the collected stdout as ExecuteResult.
	//
	// ctx cancellation must abort the running module within a reasonable
	// grace period (implementation-defined, typically ≤100 ms).
	Execute(ctx context.Context, module []byte, input []byte) (*ExecuteResult, error)

	// Close releases all resources held by the executor (thread pools,
	// compiled module caches, etc.).  After Close, Execute must return an error.
	Close(ctx context.Context) error
}

// ---------------------------------------------------------------------------
// Stub executor — compiles and runs, but always returns ErrNotImplemented.
// ---------------------------------------------------------------------------

// StubExecutor satisfies the Executor interface.  It is the default until
// wazero is vendored in v0.3.
type StubExecutor struct{}

// NewStubExecutor returns a StubExecutor.
func NewStubExecutor() Executor { return &StubExecutor{} }

// Execute always returns ErrNotImplemented.
func (s *StubExecutor) Execute(_ context.Context, _ []byte, _ []byte) (*ExecuteResult, error) {
	return nil, ErrNotImplemented
}

// Close is a no-op.
func (s *StubExecutor) Close(_ context.Context) error { return nil }

// ---------------------------------------------------------------------------
// Task manifest — standard format for packaging Wasm tasks.
// ---------------------------------------------------------------------------

// TaskManifest describes a Wasm task package uploaded to IPFS.
// It is stored as task.json alongside task.wasm and inputs/ in the same
// IPFS directory.
type TaskManifest struct {
	// Name is a human-readable label for the task.
	Name string `json:"name"`

	// Version is the semantic version of the task binary.
	Version string `json:"version"`

	// WasmFilename is the filename of the Wasm module within the task package.
	// Defaults to "task.wasm" if empty.
	WasmFilename string `json:"wasm_filename,omitempty"`

	// InputDir is the path within the IPFS directory that contains task input
	// files.  Defaults to "inputs/" if empty.
	InputDir string `json:"input_dir,omitempty"`

	// MaxDurationSeconds is the maximum expected execution time.
	// The executor cancels execution if this is exceeded.
	MaxDurationSeconds int `json:"max_duration_seconds"`

	// MemoryLimitMB is the maximum Wasm linear memory in megabytes.
	MemoryLimitMB int `json:"memory_limit_mb,omitempty"`

	// Arch lists the CPU architectures that can run this module.
	// An empty slice means "any" (pure Wasm, architecture-neutral).
	Arch []string `json:"arch,omitempty"`
}

// ---------------------------------------------------------------------------
// Timeout helper used by real executor implementations.
// ---------------------------------------------------------------------------

// WithTimeout wraps Execute with a hard timeout derived from the manifest.
// If maxSeconds ≤ 0 the call passes through without a timeout override.
func WithTimeout(exec Executor, maxSeconds int) Executor {
	if maxSeconds <= 0 {
		return exec
	}
	return &timedExecutor{inner: exec, timeout: time.Duration(maxSeconds) * time.Second}
}

type timedExecutor struct {
	inner   Executor
	timeout time.Duration
}

func (t *timedExecutor) Execute(ctx context.Context, module []byte, input []byte) (*ExecuteResult, error) {
	// WA1 fix: use a named tctx (not the incoming ctx) for the deadline check.
	// Checking the parent ctx could give a false "timed out" result when the
	// parent was cancelled for an unrelated reason, or miss our own deadline
	// when the inner executor returns a non-context error while our deadline
	// also fired.  Wrapping with %w lets callers use errors.Is(err, ErrTimeout).
	tctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()
	result, err := t.inner.Execute(tctx, module, input)
	if err != nil && errors.Is(tctx.Err(), context.DeadlineExceeded) {
		return nil, fmt.Errorf("%w: %s limit exceeded: %v", ErrTimeout, t.timeout, err)
	}
	return result, err
}

func (t *timedExecutor) Close(ctx context.Context) error {
	return t.inner.Close(ctx)
}
