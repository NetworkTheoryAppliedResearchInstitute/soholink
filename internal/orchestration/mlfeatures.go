package orchestration

// mlfeatures.go — Feature extraction for the ML scheduling layer.
//
// This file lives in the orchestration package (not internal/ml) so that it
// can reference orchestration domain types (MobileNodeInfo, MobileTaskDescriptor)
// without creating an import cycle.  The dimension constants it relies on are
// defined in internal/ml/features.go and replicated as local constants below
// to keep this file import-free with respect to the ml package.
//
// Layout contract (must stay in sync with internal/ml/features.go):
//
//	NodeFeatures    [10]float64  — see nodeFeatureDim
//	TaskFeatures    [ 6]float64  — see taskFeatureDim
//	SystemFeatures  [ 4]float64  — see systemFeatureDim
//	BuildContext    [20]float64  — concatenation of the above

// Local copies of the ml dimension constants — avoids an import cycle while
// keeping a single source of truth via the build-time assertion in assert_test.go.
const (
	nodeFeatureDim   = 10
	taskFeatureDim   = 6
	systemFeatureDim = 4
	contextDim       = nodeFeatureDim + taskFeatureDim + systemFeatureDim // 20
)

// ---------------------------------------------------------------------------
// System state
// ---------------------------------------------------------------------------

// SystemState captures scheduler-level metrics at dispatch time.
// These are converted to a fixed-length float64 vector by SystemFeatures.
type SystemState struct {
	// PendingCount is the number of tasks currently in the pending queue.
	PendingCount int

	// MobileNodeCount is the number of connected mobile nodes.
	MobileNodeCount int

	// DesktopNodeCount is the number of registered desktop/TV nodes.
	DesktopNodeCount int

	// HTLCCancelRateRecent is the HTLC cancel rate over the last 5 minutes (0–1).
	HTLCCancelRateRecent float64
}

// ---------------------------------------------------------------------------
// Node feature extraction
// ---------------------------------------------------------------------------

// NodeFeatures extracts a normalised float64 vector from a MobileNodeInfo.
//
// Layout (nodeFeatureDim = 10):
//
//	[0]  is_android_tv      (0 or 1)
//	[1]  is_mobile_android  (0 or 1)
//	[2]  is_desktop         (0 or 1)
//	[3]  memory_mb / 8192   (normalised; capped at 1)
//	[4]  cpu_cores / 8      (normalised; capped at 1)
//	[5]  battery_pct / 100  (normalised; 1.0 if not applicable)
//	[6]  plugged            (0 or 1)
//	[7]  wifi               (0 or 1)
//	[8]  arch_is_arm64      (0 or 1; 0 = amd64 or unknown)
//	[9]  battery_trend      (1=charging, 0.5=n/a, 0=draining)
func NodeFeatures(n MobileNodeInfo) [nodeFeatureDim]float64 {
	var v [nodeFeatureDim]float64

	// Class one-hot (3 bits; iOS never dispatched so no bit assigned)
	switch n.NodeClass {
	case NodeClassAndroidTV:
		v[0] = 1
	case NodeClassMobileAndroid:
		v[1] = 1
	case NodeClassDesktop:
		v[2] = 1
	}

	// Resource capacity
	if n.MemoryMB > 0 {
		v[3] = clampF(float64(n.MemoryMB)/8192.0, 0, 1)
	}
	if n.CPUCores > 0 {
		v[4] = clampF(n.CPUCores/8.0, 0, 1)
	}

	// Battery and power state
	if n.BatteryPct < 0 {
		// -1 = not applicable (always-on device); treat as full
		v[5] = 1.0
	} else {
		v[5] = clampF(float64(n.BatteryPct)/100.0, 0, 1)
	}
	if n.Plugged {
		v[6] = 1
	}
	if n.WiFi {
		v[7] = 1
	}

	// Architecture
	if n.Arch == "arm64" {
		v[8] = 1
	}

	// Battery trend: plugged in = charging (1.0), not plugged = draining (0.0).
	// Always-on (BatteryPct == -1) → 0.5 (neutral / not applicable).
	//
	// F2 fix: the previous condition "n.Plugged && n.BatteryPct > 50" was wrong —
	// a device at 20 % that is plugged in is CHARGING, not draining.
	// Plugged status alone is the correct indicator of charging direction.
	if n.BatteryPct < 0 {
		v[9] = 0.5 // always-on / no battery
	} else if n.Plugged {
		v[9] = 1.0 // plugged in → charging
	}
	// else 0.0 (on battery / draining)

	return v
}

// ---------------------------------------------------------------------------
// Task feature extraction
// ---------------------------------------------------------------------------

// TaskFeatures extracts a normalised float64 vector from a MobileTaskDescriptor.
//
// Layout (taskFeatureDim = 6):
//
//	[0]  segment_index / max(segment_count,1)  — progress through multi-segment task
//	[1]  max_duration_seconds / 120            — normalised duration budget
//	[2]  has_payment_hash                      (0 or 1)
//	[3]  segment_count / 10                    (capped at 1)
//	[4]  is_first_segment                      (0 or 1)
//	[5]  is_last_segment                       (0 or 1)
func TaskFeatures(t MobileTaskDescriptor) [taskFeatureDim]float64 {
	var v [taskFeatureDim]float64

	sc := t.SegmentCount
	if sc < 1 {
		sc = 1
	}

	v[0] = clampF(float64(t.SegmentIndex)/float64(sc), 0, 1)
	v[1] = clampF(float64(t.MaxDurationSeconds)/120.0, 0, 1)
	if t.PaymentHashHex != "" {
		v[2] = 1
	}
	v[3] = clampF(float64(sc)/10.0, 0, 1)
	if t.SegmentIndex == 0 {
		v[4] = 1
	}
	if t.SegmentIndex == sc-1 {
		v[5] = 1
	}

	return v
}

// ---------------------------------------------------------------------------
// System feature extraction
// ---------------------------------------------------------------------------

// SystemFeatures extracts a normalised float64 vector from a SystemState.
//
// Layout (systemFeatureDim = 4):
//
//	[0]  pending_count / 100           (capped at 1)
//	[1]  mobile_node_count / 50        (capped at 1)
//	[2]  desktop_node_count / 200      (capped at 1)
//	[3]  htlc_cancel_rate_recent       (0–1, already normalised)
func SystemFeatures(s SystemState) [systemFeatureDim]float64 {
	var v [systemFeatureDim]float64
	v[0] = clampF(float64(s.PendingCount)/100.0, 0, 1)
	v[1] = clampF(float64(s.MobileNodeCount)/50.0, 0, 1)
	v[2] = clampF(float64(s.DesktopNodeCount)/200.0, 0, 1)
	v[3] = clampF(s.HTLCCancelRateRecent, 0, 1)
	return v
}

// ---------------------------------------------------------------------------
// Context vector assembly
// ---------------------------------------------------------------------------

// BuildContext assembles the full context vector from node, task, and system
// features.  The returned slice has length contextDim (= ml.ContextDim = 20).
//
// Pass a zero-value MobileNodeInfo{} to produce a shared context that omits
// per-node hardware features — recommended for the disjoint LinUCB variant
// where each arm (NodeDID) learns its own weight vector.
func BuildContext(node MobileNodeInfo, task MobileTaskDescriptor, sys SystemState) []float64 {
	ctx := make([]float64, contextDim)
	nf := NodeFeatures(node)
	tf := TaskFeatures(task)
	sf := SystemFeatures(sys)

	copy(ctx[:nodeFeatureDim], nf[:])
	copy(ctx[nodeFeatureDim:nodeFeatureDim+taskFeatureDim], tf[:])
	copy(ctx[nodeFeatureDim+taskFeatureDim:], sf[:])
	return ctx
}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

// clampF returns v clamped to [lo, hi].
func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
