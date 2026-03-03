package orchestration

// NodeClass identifies the class of a compute node in the federation.
// The class drives scheduling constraints, replication policy, and payment flow.
type NodeClass string

const (
	// NodeClassDesktop represents a full SOHO desktop, workstation, or mini-PC node.
	// Always-on, AC-powered, no task-duration limits, maximum capability.
	NodeClassDesktop NodeClass = "desktop"

	// NodeClassMobileAndroid represents an Android smartphone node.
	// Participates only when plugged in to AC power, connected to WiFi,
	// and the user has explicitly opted in.  Tasks must be ≤ 120 s per segment.
	NodeClassMobileAndroid NodeClass = "mobile-android"

	// NodeClassMobileIOS represents an iPhone or iPad.
	// Management and monitoring only — iOS platform restrictions prohibit
	// sustained background compute, so this class carries no compute capability.
	NodeClassMobileIOS NodeClass = "mobile-ios"

	// NodeClassAndroidTV represents an Android TV box or Amazon Fire TV device.
	// Always-on, always-plugged-in, no battery or thermal constraints.
	// The best mobile-ecosystem option for steady compute work.
	NodeClassAndroidTV NodeClass = "android-tv"
)

// NodeConstraints describes scheduling constraints that are specific to a
// node class.  The FedScheduler consults these when selecting candidates and
// when deciding whether to shadow-replicate a task.
type NodeConstraints struct {
	// MaxTaskDurationSeconds is the maximum wall-clock duration (in seconds) a
	// single task segment may run on this node.  0 means unconstrained.
	// Mobile nodes require ≤ 120 s to survive OS interruption.
	MaxTaskDurationSeconds int `json:"max-task-duration-seconds"`

	// RequiresPluggedIn indicates the node participates only when connected
	// to AC power.
	RequiresPluggedIn bool `json:"requires-plugged-in"`

	// WifiOnly indicates the node only accepts tasks when connected to WiFi
	// (not cellular), to avoid incurring mobile data charges.
	WifiOnly bool `json:"wifi-only"`

	// Arch is the CPU instruction-set architecture: "amd64", "arm64", etc.
	// The scheduler uses this to route Wasm tasks compiled for a specific ISA.
	Arch string `json:"arch"`

	// Mobile indicates the node is battery-powered.  The scheduler applies
	// shadow replication (OPA: task_replication_factor) for battery nodes.
	Mobile bool `json:"mobile"`
}

// DefaultConstraints returns the canonical NodeConstraints for a NodeClass.
// Mobile apps MAY override individual fields after calling this.
func DefaultConstraints(class NodeClass) NodeConstraints {
	switch class {
	case NodeClassAndroidTV:
		return NodeConstraints{
			MaxTaskDurationSeconds: 0,    // unconstrained — always-on device
			RequiresPluggedIn:      true,
			WifiOnly:               true,
			Arch:                   "arm64",
			Mobile:                 false,
		}
	case NodeClassMobileAndroid:
		return NodeConstraints{
			MaxTaskDurationSeconds: 120,
			RequiresPluggedIn:      true,
			WifiOnly:               true,
			Arch:                   "arm64",
			Mobile:                 true,
		}
	case NodeClassMobileIOS:
		return NodeConstraints{
			MaxTaskDurationSeconds: 0,    // monitoring only; no compute dispatched
			RequiresPluggedIn:      false,
			WifiOnly:               false,
			Arch:                   "arm64",
			Mobile:                 true,
		}
	default: // NodeClassDesktop
		return NodeConstraints{
			MaxTaskDurationSeconds: 0,
			RequiresPluggedIn:      false,
			WifiOnly:               false,
			Arch:                   "amd64",
			Mobile:                 false,
		}
	}
}

// ---------------------------------------------------------------------------
// Wire protocol types shared between the coordinator and mobile clients.
// ---------------------------------------------------------------------------

// MobileNodeInfo is the registration payload sent by a mobile node when it
// first connects (or reconnects) to the coordinator WebSocket hub.
type MobileNodeInfo struct {
	NodeDID    string    `json:"node_did"`
	NodeClass  NodeClass `json:"node_class"`
	Arch       string    `json:"arch"`
	MemoryMB   int64     `json:"memory_mb"`
	CPUCores   float64   `json:"cpu_cores"`
	BatteryPct int       `json:"battery_pct"` // 0–100; -1 if not applicable
	Plugged    bool      `json:"plugged"`
	WiFi       bool      `json:"wifi"`
	AppVersion string    `json:"app_version"`
}

// MobileTaskDescriptor is sent from the coordinator to a mobile node via the
// WebSocket hub.  It describes a single task segment for the node to execute.
type MobileTaskDescriptor struct {
	TaskID            string `json:"task_id"`
	WorkloadID        string `json:"workload_id"`
	WasmCID           string `json:"wasm_cid"`         // IPFS CID of task.wasm
	InputCID          string `json:"input_cid"`        // IPFS CID of inputs/ directory
	MaxDurationSeconds int   `json:"max_duration_s"`
	SegmentIndex      int    `json:"segment_index"`
	SegmentCount      int    `json:"segment_count"`
	PaymentHashHex    string `json:"payment_hash_hex"` // HTLC payment hash
}

// MobileTaskResult is sent from the mobile node to the coordinator after a
// task segment completes (or fails).
type MobileTaskResult struct {
	TaskID     string `json:"task_id"`
	WorkloadID string `json:"workload_id"`
	ResultCID  string `json:"result_cid"`  // IPFS CID of the result bytes
	ResultHash string `json:"result_hash"` // SHA-256 hex of result bytes
	DurationMs int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

// MobileHeartbeat is sent periodically by the mobile node to prove liveness.
type MobileHeartbeat struct {
	NodeDID    string `json:"node_did"`
	BatteryPct int    `json:"battery_pct"`
	Plugged    bool   `json:"plugged"`
	WiFi       bool   `json:"wifi"`
}
