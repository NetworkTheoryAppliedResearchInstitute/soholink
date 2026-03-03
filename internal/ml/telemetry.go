package ml

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ---------------------------------------------------------------------------
// SchedulerEvent — one JSONL record per dispatch decision
// ---------------------------------------------------------------------------

// Outcome classifies how a dispatched task segment resolved.
type Outcome string

const (
	OutcomeCompleted  Outcome = "completed"   // result returned without error
	OutcomePreempted  Outcome = "preempted"   // mobile node disconnected mid-task
	OutcomeHTLCSettle Outcome = "htlc_settle" // Lightning payment settled; shadow verified
	OutcomeHTLCCancel Outcome = "htlc_cancel" // Lightning payment cancelled; verification failed
	OutcomeError      Outcome = "error"       // node returned explicit error result
	OutcomePending    Outcome = "pending"     // not yet resolved (in-flight record)
)

// SchedulerEvent is one complete record written to the telemetry JSONL file.
// It captures the full context visible at dispatch time plus the eventual
// outcome so that offline ML training can use it as a labelled example.
type SchedulerEvent struct {
	// ---- Identity ----
	EventID    string    `json:"event_id"`
	RecordedAt time.Time `json:"recorded_at"`

	// ---- Dispatch context ----
	WorkloadID string  `json:"workload_id"`
	TaskID     string  `json:"task_id"`
	NodeDID    string  `json:"node_did"`
	NodeClass  string  `json:"node_class"`
	ArmIndex   int     `json:"arm_index"` // index into candidates slice at dispatch time

	// ---- Feature vectors (as recorded at dispatch time) ----
	NodeFeatures   []float64 `json:"node_features"`    // length NodeFeatureDim
	TaskFeatures   []float64 `json:"task_features"`    // length TaskFeatureDim
	SystemFeatures []float64 `json:"system_features"`  // length SystemFeatureDim

	// ---- Outcome (filled in on resolution) ----
	Outcome    Outcome `json:"outcome"`
	DurationMs int64   `json:"duration_ms,omitempty"`
	Reward     float64 `json:"reward,omitempty"` // scalar reward signal for bandit update
}

// RewardFor maps an Outcome to a scalar reward signal in [0, 1].
// The mapping is designed so that HTLC settlement is the strongest
// positive signal and HTLC cancellation the strongest negative signal.
//
// B4 — OutcomePending returns 0.0 intentionally, but callers MUST NOT feed
// this value to LinUCBBandit.Update.  A pending event is written to the
// telemetry log only to capture the dispatch-time feature vectors; the bandit
// is updated later, once the task resolves to a concrete outcome via
// RecordMobileOutcome (which is called with HTLCSettle, HTLCCancel, Error,
// etc.).  Calling bandit.Update with reward=0.0 from a pending event would
// incorrectly penalise the selected arm before the outcome is known.
func RewardFor(o Outcome, durationMs int64, maxDurationMs int64) float64 {
	switch o {
	case OutcomeHTLCSettle:
		// Maximum reward; bonus for fast completion.
		speedBonus := 0.0
		if maxDurationMs > 0 && durationMs > 0 && durationMs < maxDurationMs {
			speedBonus = 0.2 * (1.0 - float64(durationMs)/float64(maxDurationMs))
		}
		return clamp(0.8+speedBonus, 0, 1)
	case OutcomeCompleted:
		return 0.6
	case OutcomeError:
		return 0.1
	case OutcomePreempted:
		return 0.0
	case OutcomeHTLCCancel:
		return 0.0
	default:
		return 0.0
	}
}

// ---------------------------------------------------------------------------
// TelemetryRecorder — appends SchedulerEvents to a JSONL file
// ---------------------------------------------------------------------------

// TelemetryRecorder writes scheduling events to a JSONL file for offline
// analysis and ML training.  It is designed to be non-blocking: writes are
// queued in a buffered channel and flushed by a background goroutine.
//
// Each line in the output file is a valid JSON object (SchedulerEvent).
// The file can be consumed directly by Python pandas, DuckDB, or any tool
// that understands JSONL.
type TelemetryRecorder struct {
	path      string
	queue     chan SchedulerEvent
	closeOnce sync.Once  // ensures close(queue) is called exactly once (T1)
	mu        sync.Mutex // guards file handle and writer
	file      *os.File
	writer    *bufio.Writer
	done      chan struct{}
}

// NewTelemetryRecorder creates a recorder that writes to path.
// If path is empty, a file named "scheduler_telemetry.jsonl" is created in
// the current working directory.
// bufSize controls the in-memory queue depth; 1024 is adequate for most loads.
func NewTelemetryRecorder(path string, bufSize int) (*TelemetryRecorder, error) {
	if path == "" {
		path = "scheduler_telemetry.jsonl"
	}
	// T3: Reject paths with path-traversal sequences to prevent writing outside
	// the intended directory (e.g. "../../../etc/cron.d/evil").
	if strings.Contains(path, "..") {
		return nil, fmt.Errorf("telemetry: path must not contain '..': %q", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	if bufSize <= 0 {
		bufSize = 1024
	}

	r := &TelemetryRecorder{
		path:   path,
		queue:  make(chan SchedulerEvent, bufSize),
		file:   f,
		writer: bufio.NewWriterSize(f, 64*1024),
		done:   make(chan struct{}),
	}
	go r.writeLoop()
	return r, nil
}

// Record enqueues a SchedulerEvent for writing.  If the queue is full the
// event is dropped with a log warning rather than blocking the scheduler.
func (r *TelemetryRecorder) Record(ev SchedulerEvent) {
	select {
	case r.queue <- ev:
	default:
		log.Printf("[telemetry] queue full — dropping event for task %s", ev.TaskID)
	}
}

// Close signals the write loop to stop, waits for the final flush, then
// closes the underlying file.  Safe to call multiple times (T1).
func (r *TelemetryRecorder) Close() error {
	// closeOnce prevents a double-close panic if Close is called concurrently
	// or more than once (T1).
	r.closeOnce.Do(func() { close(r.queue) })
	<-r.done
	// writeLoop already performed a final flush before signalling done;
	// no second flush needed here (T2).
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.file.Close()
}

// writeLoop drains the event queue and writes JSONL records.
func (r *TelemetryRecorder) writeLoop() {
	defer close(r.done)
	for ev := range r.queue {
		r.writeOne(ev)
	}
	// Final flush after channel close.
	r.mu.Lock()
	_ = r.writer.Flush()
	r.mu.Unlock()
}

// writeOne serialises ev and appends it as a JSONL record.
func (r *TelemetryRecorder) writeOne(ev SchedulerEvent) {
	b, err := json.Marshal(ev)
	if err != nil {
		log.Printf("[telemetry] marshal error for task %s: %v", ev.TaskID, err)
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// T4: Surface write errors rather than silently discarding records.
	if _, werr := r.writer.Write(b); werr != nil {
		log.Printf("[telemetry] write error for task %s: %v", ev.TaskID, werr)
		return
	}
	if werr := r.writer.WriteByte('\n'); werr != nil {
		log.Printf("[telemetry] write newline error for task %s: %v", ev.TaskID, werr)
		return
	}

	// Flush when the buffer is at least half full to bound data loss on crash.
	if r.writer.Buffered() >= 32*1024 {
		if ferr := r.writer.Flush(); ferr != nil {
			log.Printf("[telemetry] flush error: %v", ferr)
		}
	}
}

// ---------------------------------------------------------------------------
// EventBuilder — fluent helper for constructing SchedulerEvents
// ---------------------------------------------------------------------------

// EventBuilder builds a SchedulerEvent incrementally.  Create one at dispatch
// time, fill in the context fields, then call Outcome() when the task resolves.
type EventBuilder struct {
	ev SchedulerEvent
}

// NewEventBuilder starts a new event for the given task + node.
func NewEventBuilder(workloadID, taskID, nodeDID, nodeClass string, armIndex int) *EventBuilder {
	return &EventBuilder{ev: SchedulerEvent{
		EventID:    taskID + "_" + nodeDID,
		RecordedAt: time.Now(),
		WorkloadID: workloadID,
		TaskID:     taskID,
		NodeDID:    nodeDID,
		NodeClass:  nodeClass,
		ArmIndex:   armIndex,
		Outcome:    OutcomePending,
	}}
}

// WithNodeFeatures attaches the node feature vector to the event.
func (b *EventBuilder) WithNodeFeatures(f []float64) *EventBuilder {
	b.ev.NodeFeatures = f
	return b
}

// WithTaskFeatures attaches the task feature vector.
func (b *EventBuilder) WithTaskFeatures(f []float64) *EventBuilder {
	b.ev.TaskFeatures = f
	return b
}

// WithSystemFeatures attaches the system feature vector.
func (b *EventBuilder) WithSystemFeatures(f []float64) *EventBuilder {
	b.ev.SystemFeatures = f
	return b
}

// Resolve sets the outcome and duration, computes the scalar reward, and
// returns the final SchedulerEvent ready to be passed to Record().
func (b *EventBuilder) Resolve(outcome Outcome, durationMs int64, maxDurationMs int64) SchedulerEvent {
	b.ev.Outcome = outcome
	b.ev.DurationMs = durationMs
	b.ev.Reward = RewardFor(outcome, durationMs, maxDurationMs)
	return b.ev
}
