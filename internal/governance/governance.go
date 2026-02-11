package governance

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// ProposalType defines the type of governance proposal.
type ProposalType string

const (
	ProposalTypeParameter      ProposalType = "parameter"       // Change system parameter
	ProposalTypeFeatureToggle  ProposalType = "feature_toggle"  // Enable/disable feature
	ProposalTypeNodeAdmission  ProposalType = "node_admission"  // Admit new node to federation
	ProposalTypeNodeRemoval    ProposalType = "node_removal"    // Remove node from federation
	ProposalTypePolicyChange   ProposalType = "policy_change"   // Change federation policy
	ProposalTypeTreasurySpend  ProposalType = "treasury_spend"  // Spend from federation treasury
)

// ProposalState defines the current state of a proposal.
type ProposalState string

const (
	ProposalStateDraft    ProposalState = "draft"    // Being prepared
	ProposalStateActive   ProposalState = "active"   // Open for voting
	ProposalStatePassed   ProposalState = "passed"   // Passed, awaiting execution
	ProposalStateRejected ProposalState = "rejected" // Rejected by vote
	ProposalStateExecuted ProposalState = "executed" // Executed successfully
	ProposalStateExpired  ProposalState = "expired"  // Voting period expired
)

// VoteChoice defines a vote cast on a proposal.
type VoteChoice string

const (
	VoteYes     VoteChoice = "yes"
	VoteNo      VoteChoice = "no"
	VoteAbstain VoteChoice = "abstain"
)

// Proposal represents a governance proposal.
type Proposal struct {
	ProposalID   string        `json:"proposal_id"`
	ProposerDID  string        `json:"proposer_did"`
	Title        string        `json:"title"`
	Description  string        `json:"description"`
	ProposalType ProposalType  `json:"proposal_type"`
	State        ProposalState `json:"state"`
	VotingStart  time.Time     `json:"voting_start"`
	VotingEnd    time.Time     `json:"voting_end"`
	QuorumPct    int           `json:"quorum_pct"`     // Percentage of eligible voters required
	PassPct      int           `json:"pass_pct"`       // Percentage of votes needed to pass
	YesVotes     int           `json:"yes_votes"`
	NoVotes      int           `json:"no_votes"`
	AbstainVotes int           `json:"abstain_votes"`
	CreatedAt    time.Time     `json:"created_at"`
	UpdatedAt    time.Time     `json:"updated_at"`
	ExecutedAt   *time.Time    `json:"executed_at,omitempty"`
}

// Vote represents a vote cast on a proposal.
type Vote struct {
	VoteID     string     `json:"vote_id"`
	ProposalID string     `json:"proposal_id"`
	VoterDID   string     `json:"voter_did"`
	Choice     VoteChoice `json:"choice"`
	Signature  string     `json:"signature"` // Ed25519 signature over vote data
	CreatedAt  time.Time  `json:"created_at"`
}

// Manager handles governance proposal and voting operations.
type Manager struct {
	store *store.Store
}

// NewManager creates a new governance manager.
func NewManager(s *store.Store) *Manager {
	return &Manager{
		store: s,
	}
}

// CreateProposal creates a new governance proposal.
func (m *Manager) CreateProposal(ctx context.Context, p *Proposal) error {
	if p.ProposalID == "" {
		p.ProposalID = generateProposalID(p.ProposerDID, p.Title, time.Now())
	}

	// Set default quorum and pass thresholds if not specified
	if p.QuorumPct == 0 {
		p.QuorumPct = 51 // Default: 51% quorum
	}
	if p.PassPct == 0 {
		p.PassPct = 66 // Default: 66% supermajority
	}

	// Set timestamps
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now
	p.State = ProposalStateDraft

	// If voting times not specified, default to 7 days from now
	if p.VotingStart.IsZero() {
		p.VotingStart = now
	}
	if p.VotingEnd.IsZero() {
		p.VotingEnd = p.VotingStart.Add(7 * 24 * time.Hour)
	}

	// Validate
	if p.VotingEnd.Before(p.VotingStart) {
		return fmt.Errorf("voting_end must be after voting_start")
	}
	if p.QuorumPct < 1 || p.QuorumPct > 100 {
		return fmt.Errorf("quorum_pct must be between 1 and 100")
	}
	if p.PassPct < 1 || p.PassPct > 100 {
		return fmt.Errorf("pass_pct must be between 1 and 100")
	}

	return m.store.CreateGovernanceProposal(ctx, p)
}

// ActivateProposal moves a proposal from draft to active state.
func (m *Manager) ActivateProposal(ctx context.Context, proposalID string) error {
	p, err := m.store.GetGovernanceProposal(ctx, proposalID)
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("proposal %s not found", proposalID)
	}
	if p.State != ProposalStateDraft {
		return fmt.Errorf("proposal %s is not in draft state", proposalID)
	}

	p.State = ProposalStateActive
	p.UpdatedAt = time.Now()

	return m.store.UpdateGovernanceProposal(ctx, p)
}

// CastVote records a vote on a proposal.
func (m *Manager) CastVote(ctx context.Context, v *Vote) error {
	// Get proposal
	p, err := m.store.GetGovernanceProposal(ctx, v.ProposalID)
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("proposal %s not found", v.ProposalID)
	}

	// Validate proposal state
	if p.State != ProposalStateActive {
		return fmt.Errorf("proposal %s is not active for voting", v.ProposalID)
	}

	// Validate voting period
	now := time.Now()
	if now.Before(p.VotingStart) {
		return fmt.Errorf("voting has not started yet")
	}
	if now.After(p.VotingEnd) {
		return fmt.Errorf("voting period has ended")
	}

	// Validate choice
	if v.Choice != VoteYes && v.Choice != VoteNo && v.Choice != VoteAbstain {
		return fmt.Errorf("invalid vote choice: %s", v.Choice)
	}

	// Check if voter already voted
	existingVote, err := m.store.GetGovernanceVote(ctx, v.ProposalID, v.VoterDID)
	if err != nil {
		return err
	}
	if existingVote != nil {
		return fmt.Errorf("voter %s has already voted on proposal %s", v.VoterDID, v.ProposalID)
	}

	// Generate vote ID
	if v.VoteID == "" {
		v.VoteID = generateVoteID(v.ProposalID, v.VoterDID, now)
	}
	v.CreatedAt = now

	// Record vote
	if err := m.store.CreateGovernanceVote(ctx, v); err != nil {
		return err
	}

	// Update proposal vote counts
	switch v.Choice {
	case VoteYes:
		p.YesVotes++
	case VoteNo:
		p.NoVotes++
	case VoteAbstain:
		p.AbstainVotes++
	}
	p.UpdatedAt = now

	return m.store.UpdateGovernanceProposal(ctx, p)
}

// TallyProposal calculates the final result and updates proposal state.
func (m *Manager) TallyProposal(ctx context.Context, proposalID string) error {
	p, err := m.store.GetGovernanceProposal(ctx, proposalID)
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("proposal %s not found", proposalID)
	}

	// Can only tally active proposals after voting ends
	if p.State != ProposalStateActive {
		return fmt.Errorf("proposal %s is not active", proposalID)
	}
	if time.Now().Before(p.VotingEnd) {
		return fmt.Errorf("voting period has not ended yet")
	}

	// Get total eligible voters (nodes in federation)
	eligibleVoters, err := m.store.CountEligibleVoters(ctx)
	if err != nil {
		return err
	}
	if eligibleVoters == 0 {
		return fmt.Errorf("no eligible voters found")
	}

	// Calculate totals
	totalVotes := p.YesVotes + p.NoVotes + p.AbstainVotes
	participationPct := (totalVotes * 100) / eligibleVoters

	// Check quorum
	if participationPct < p.QuorumPct {
		p.State = ProposalStateRejected
		p.UpdatedAt = time.Now()
		return m.store.UpdateGovernanceProposal(ctx, p)
	}

	// Calculate pass percentage (excluding abstentions)
	votesForDecision := p.YesVotes + p.NoVotes
	if votesForDecision == 0 {
		// All abstentions, reject
		p.State = ProposalStateRejected
		p.UpdatedAt = time.Now()
		return m.store.UpdateGovernanceProposal(ctx, p)
	}

	yesPct := (p.YesVotes * 100) / votesForDecision

	// Determine outcome
	if yesPct >= p.PassPct {
		p.State = ProposalStatePassed
	} else {
		p.State = ProposalStateRejected
	}
	p.UpdatedAt = time.Now()

	return m.store.UpdateGovernanceProposal(ctx, p)
}

// ExecuteProposal marks a passed proposal as executed.
func (m *Manager) ExecuteProposal(ctx context.Context, proposalID string) error {
	p, err := m.store.GetGovernanceProposal(ctx, proposalID)
	if err != nil {
		return err
	}
	if p == nil {
		return fmt.Errorf("proposal %s not found", proposalID)
	}
	if p.State != ProposalStatePassed {
		return fmt.Errorf("proposal %s has not passed", proposalID)
	}

	now := time.Now()
	p.State = ProposalStateExecuted
	p.UpdatedAt = now
	p.ExecutedAt = &now

	return m.store.UpdateGovernanceProposal(ctx, p)
}

// GetProposal retrieves a proposal by ID.
func (m *Manager) GetProposal(ctx context.Context, proposalID string) (*Proposal, error) {
	return m.store.GetGovernanceProposal(ctx, proposalID)
}

// ListProposals returns all proposals matching the given state filter.
func (m *Manager) ListProposals(ctx context.Context, state ProposalState, limit, offset int) ([]*Proposal, error) {
	return m.store.ListGovernanceProposals(ctx, string(state), limit, offset)
}

// GetVotesForProposal returns all votes cast on a proposal.
func (m *Manager) GetVotesForProposal(ctx context.Context, proposalID string) ([]*Vote, error) {
	return m.store.ListGovernanceVotes(ctx, proposalID)
}

// generateProposalID creates a unique proposal ID.
func generateProposalID(proposerDID, title string, t time.Time) string {
	data := fmt.Sprintf("%s:%s:%d", proposerDID, title, t.UnixNano())
	hash := sha256.Sum256([]byte(data))
	return "prop_" + hex.EncodeToString(hash[:8])
}

// generateVoteID creates a unique vote ID.
func generateVoteID(proposalID, voterDID string, t time.Time) string {
	data := fmt.Sprintf("%s:%s:%d", proposalID, voterDID, t.UnixNano())
	hash := sha256.Sum256([]byte(data))
	return "vote_" + hex.EncodeToString(hash[:8])
}
