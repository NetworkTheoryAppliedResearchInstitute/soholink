// Package ml provides machine-learning primitives for the SoHoLINK scheduler.
//
// All types and functions in this package are pure Go with no external ML
// framework dependencies.  Feature extraction lives in the orchestration
// package (mlfeatures.go) so that it can reference orchestration domain types
// without creating an import cycle.  The constants below define the vector
// layout that must be kept in sync between orchestration/mlfeatures.go and
// the bandit/telemetry consumers in this package.
package ml

// ---------------------------------------------------------------------------
// Dimension constants — document the layout of each feature vector.
// Any change here must be reflected in orchestration/mlfeatures.go and the
// bandit's arm matrix size.
// ---------------------------------------------------------------------------

// clamp returns v clamped to [lo, hi].
func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

const (
	// NodeFeatureDim is the number of float64 features extracted per node.
	NodeFeatureDim = 10

	// TaskFeatureDim is the number of float64 features extracted per task.
	TaskFeatureDim = 6

	// SystemFeatureDim is the number of system-level float64 features.
	SystemFeatureDim = 4

	// ContextDim is the total context vector length passed to the bandit.
	// context = node_features ++ task_features ++ system_features
	ContextDim = NodeFeatureDim + TaskFeatureDim + SystemFeatureDim
)
