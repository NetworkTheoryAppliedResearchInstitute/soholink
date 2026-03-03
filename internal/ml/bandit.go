package ml

import (
	"fmt"
	"math"
	"sync"
)

// ---------------------------------------------------------------------------
// LinUCBBandit — Disjoint Linear Upper Confidence Bound
//
// Reference: Li et al., "A Contextual-Bandit Approach to Personalized News
// Article Recommendation", WWW 2010.
//
// For each arm a the bandit maintains:
//   - A_a  : d×d design matrix (initialised to I_d)
//   - b_a  : d-vector (initialised to 0)
//   - θ_a  : A_a⁻¹ · b_a  (the least-squares weight vector; lazily recomputed)
//
// At selection time, for context x (d-dimensional):
//   UCB_a = θ_a · x  +  α · sqrt(x^T · A_a⁻¹ · x)
//
// The arm with the highest UCB score is selected.
// After observing reward r for arm a with context x:
//   A_a ← A_a + x · x^T
//   b_a ← b_a + r · x
//
// Alpha (α) controls the exploration–exploitation tradeoff:
//   small α  → exploit (greedy); large α → explore.
//   A value of 0.1–1.0 is typical.  Default: 0.3.
// ---------------------------------------------------------------------------

// LinUCBBandit is a contextual bandit using the disjoint Linear UCB algorithm.
// It is safe for concurrent use.
type LinUCBBandit struct {
	mu    sync.RWMutex
	alpha float64
	dim   int // context vector dimension

	// Per-arm state.  Arms are identified by string keys (NodeDID).
	arms map[string]*armState

	// dirty tracks which arms need their theta recomputed.
	dirty map[string]bool
}

// armState holds the matrices for one arm.
type armState struct {
	// A is the d×d design matrix stored in row-major order (d*d floats).
	A []float64
	// b is the d-vector of accumulated reward signals.
	b []float64
	// theta is the cached least-squares coefficient vector (A⁻¹ · b).
	theta []float64
	// AInv is the cached inverse of A (lazily recomputed when dirty).
	AInv []float64
}

// SelectResult is returned by Select, giving the chosen arm and its UCB score.
type SelectResult struct {
	ArmKey   string
	UCBScore float64
	ArmIndex int // index in the arms slice passed to Select
}

// NewLinUCBBandit creates a new LinUCBBandit.
//
//   - dim   : context vector dimension (must equal ContextDim or caller's choice)
//   - alpha : exploration parameter (0.1–1.0; default 0.3 if ≤ 0)
func NewLinUCBBandit(dim int, alpha float64) *LinUCBBandit {
	if alpha <= 0 {
		alpha = 0.3
	}
	return &LinUCBBandit{
		alpha: alpha,
		dim:   dim,
		arms:  make(map[string]*armState),
		dirty: make(map[string]bool),
	}
}

// Select chooses the arm with the highest UCB score from the provided arm keys.
// If an arm key is not yet known, it is initialised on first encounter.
// Returns an error only if armKeys is empty.
func (b *LinUCBBandit) Select(context []float64, armKeys []string) (SelectResult, error) {
	if len(armKeys) == 0 {
		return SelectResult{}, fmt.Errorf("bandit: no arms to select from")
	}
	if len(context) != b.dim {
		return SelectResult{}, fmt.Errorf("bandit: context dim %d != expected %d", len(context), b.dim)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Ensure all arms exist (new arms get identity matrix → explore first).
	for _, key := range armKeys {
		if _, ok := b.arms[key]; !ok {
			b.initArm(key)
		}
	}

	// Recompute inverses for any dirty arms.
	for _, key := range armKeys {
		if b.dirty[key] {
			b.recomputeTheta(key)
			b.dirty[key] = false
		}
	}

	// Score every arm and pick the maximum.
	bestKey := armKeys[0]
	bestScore := math.Inf(-1)
	bestIdx := 0

	for i, key := range armKeys {
		arm := b.arms[key]
		score := b.ucbScore(arm, context)
		if score > bestScore {
			bestScore = score
			bestKey = key
			bestIdx = i
		}
	}

	return SelectResult{ArmKey: bestKey, UCBScore: bestScore, ArmIndex: bestIdx}, nil
}

// Update incorporates the observed reward for a given arm and context.
// This should be called once the scheduling outcome is known
// (e.g., on HTLC settle/cancel or on task completion).
func (b *LinUCBBandit) Update(armKey string, context []float64, reward float64) error {
	if len(context) != b.dim {
		return fmt.Errorf("bandit: context dim %d != expected %d", len(context), b.dim)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	arm, ok := b.arms[armKey]
	if !ok {
		b.initArm(armKey)
		arm = b.arms[armKey]
	}

	d := b.dim

	// A_a ← A_a + x·xᵀ
	for i := 0; i < d; i++ {
		for j := 0; j < d; j++ {
			arm.A[i*d+j] += context[i] * context[j]
		}
	}

	// b_a ← b_a + r·x
	for i := 0; i < d; i++ {
		arm.b[i] += reward * context[i]
	}

	b.dirty[armKey] = true
	return nil
}

// RemoveArm removes a disconnected node's arm state.
// Call this when a mobile node disconnects so stale state doesn't accumulate.
func (b *LinUCBBandit) RemoveArm(armKey string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.arms, armKey)
	delete(b.dirty, armKey)
}

// ArmCount returns the number of arms currently tracked.
func (b *LinUCBBandit) ArmCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.arms)
}

// ---------------------------------------------------------------------------
// Internal helpers (called under b.mu lock)
// ---------------------------------------------------------------------------

// initArm creates a new arm with A = I_d, b = 0.
func (b *LinUCBBandit) initArm(key string) {
	d := b.dim
	arm := &armState{
		A:     make([]float64, d*d),
		b:     make([]float64, d),
		theta: make([]float64, d),
		AInv:  make([]float64, d*d),
	}
	// Initialise A to identity.
	for i := 0; i < d; i++ {
		arm.A[i*d+i] = 1.0
		arm.AInv[i*d+i] = 1.0 // AInv starts as I since A = I
	}
	b.arms[key] = arm
	b.dirty[key] = false // AInv already matches A = I
}

// recomputeTheta recomputes AInv and theta for an arm using Gaussian
// elimination (exact matrix inverse).  For the small context dimensions used
// here (≤ 20) this is fast enough; replace with Cholesky for larger dims.
func (b *LinUCBBandit) recomputeTheta(key string) {
	arm := b.arms[key]
	d := b.dim

	// Invert A using Gauss-Jordan elimination on [A | I].
	inv := gaussJordanInverse(arm.A, d)
	if inv == nil {
		// Singular or near-singular: reset to identity (conservative explore).
		for i := range arm.AInv {
			arm.AInv[i] = 0
		}
		for i := 0; i < d; i++ {
			arm.AInv[i*d+i] = 1.0
		}
	} else {
		copy(arm.AInv, inv)
	}

	// theta = AInv · b
	for i := 0; i < d; i++ {
		sum := 0.0
		for j := 0; j < d; j++ {
			sum += arm.AInv[i*d+j] * arm.b[j]
		}
		arm.theta[i] = sum
	}
}

// ucbScore computes UCB_a(x) = θ_a·x + α·sqrt(x^T·AInv·x).
func (b *LinUCBBandit) ucbScore(arm *armState, x []float64) float64 {
	d := b.dim

	// exploitation term: θ·x
	exploit := 0.0
	for i := 0; i < d; i++ {
		exploit += arm.theta[i] * x[i]
	}

	// exploration term: sqrt(x^T · AInv · x)
	// = sqrt( Σ_i Σ_j x_i · AInv[i,j] · x_j )
	quad := 0.0
	for i := 0; i < d; i++ {
		row := 0.0
		for j := 0; j < d; j++ {
			row += arm.AInv[i*d+j] * x[j]
		}
		quad += x[i] * row
	}
	if quad < 0 {
		quad = 0
	}
	explore := b.alpha * math.Sqrt(quad)

	return exploit + explore
}

// ---------------------------------------------------------------------------
// Gauss-Jordan matrix inverse
// ---------------------------------------------------------------------------

// gaussJordanInverse returns the inverse of the d×d row-major matrix m.
// Returns nil if m is singular (pivot < 1e-12).
func gaussJordanInverse(m []float64, d int) []float64 {
	// Work on a copy to avoid mutating the arm's A matrix.
	aug := make([]float64, d*2*d)
	for i := 0; i < d; i++ {
		for j := 0; j < d; j++ {
			aug[i*(2*d)+j] = m[i*d+j]
		}
		aug[i*(2*d)+d+i] = 1.0 // right half = identity
	}

	for col := 0; col < d; col++ {
		// Find pivot row.
		maxRow := col
		maxVal := math.Abs(aug[col*(2*d)+col])
		for row := col + 1; row < d; row++ {
			if v := math.Abs(aug[row*(2*d)+col]); v > maxVal {
				maxVal = v
				maxRow = row
			}
		}
		if maxVal < 1e-12 {
			return nil // singular
		}

		// Swap pivot row.
		if maxRow != col {
			for j := 0; j < 2*d; j++ {
				aug[col*(2*d)+j], aug[maxRow*(2*d)+j] =
					aug[maxRow*(2*d)+j], aug[col*(2*d)+j]
			}
		}

		// Scale pivot row.
		pivot := aug[col*(2*d)+col]
		for j := 0; j < 2*d; j++ {
			aug[col*(2*d)+j] /= pivot
		}

		// Eliminate column in all other rows.
		for row := 0; row < d; row++ {
			if row == col {
				continue
			}
			factor := aug[row*(2*d)+col]
			for j := 0; j < 2*d; j++ {
				aug[row*(2*d)+j] -= factor * aug[col*(2*d)+j]
			}
		}
	}

	// Extract right half.
	inv := make([]float64, d*d)
	for i := 0; i < d; i++ {
		for j := 0; j < d; j++ {
			inv[i*d+j] = aug[i*(2*d)+d+j]
		}
	}
	return inv
}
