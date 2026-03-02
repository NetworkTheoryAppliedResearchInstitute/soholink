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
	ProposalTypeParameter     ProposalType = "parameter"
	ProposalTypeFeatureToggle ProposalType = "feature_toggle"
	ProposalTypeNodeAdmission ProposalType = "node_admission"
	ProposalTypeNodeRemoval   ProposalType = "node_removal"
	ProposalTypePolicyChange  ProposalType = "policy_change"
	ProposalTypeTreasurySpend ProposalType = "treasury_spend"
)

// ProposalState defines the current state of a proposal.
type ProposalState string

const (
	ProposalStateDraft    ProposalState = "draft"
	ProposalStateActive   ProposalState = "active"
	ProposalStatePassed   ProposalState = "passed"
	ProposalStateRejected ProposalState = "rejected"
	ProposalStateExecuted ProposalState = "executed"
	ProposalStateExpired  ProposalState = "expired"
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
	QuorumPct    int           `json:"quorum_pct"`
	PassPct      int           `json:"pass_pct"`
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
	Signature  string     `json:"signature"`
	CreatedAt  time.Time  `json:"created_at"`
}

// Manager handles governance proposal and voting operations.
type Manager struct {
	store *store.Store
}

// NewManager creates a new governance manager.
func NewManager(s *store.Store) *Manager {
	return &Manager{store: s}
}

// CreateProposal creates a new governance proposal.
func (m *Manager) CreateProposal(ctx context.Context, p *Proposal) error {
	if p.ProposalID == "" {
		p.ProposalID = generateProposalID(p.ProposerDID, p.Title, time.Now())
	}

	if p.QuorumPct == 0 {
		p.QuorumPct = 51
	}
	if p.PassPct == 0 {
		p.PassPct = 66
	}

	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now
	p.State = ProposalStateDraft

	if p.VotingStart.IsZero() {
		p.VotingStart = now
	}
	if p.VotingEnd.IsZero() {
		p.VotingEnd = p.VotingStart.Add(7 * 24 * time.Hour)
	}

	if p.VotingEnd.Before(p.VotingStart) {
		return fmt.Errorf("voting_end must be after voting_start")
	}
	if p.QuorumPct < 1 || p.QuorumPct > 100 {
		return fmt.Errorf("quorum_pct must be between 1 and 100")
	}
	if p.PassPct < 1 || p.PassPct > 100 {
		return fmt.Errorf("pass_pct must be between 1 and 100")
	}

	return m.store.CreateGovernanceProposal(ctx, toStoreProposal(p))
}

// ActivateProposal moves a proposal from draft to active state.
func (m *Manager) ActivateProposal(ctx context.Context, proposalID string) error {
	sp, err := m.store.GetGovernanceProposal(ctx, proposalID)
	if err != nil {
		return err
	}
	if sp == nil {
		return fmt.Errorf("proposal %s not found", proposalID)
	}

	p := fromStoreProposal(sp)
	if p.State != ProposalStateDraft {
		return fmt.Errorf("proposal %s is not in draft state", proposalID)
	}

	p.State = ProposalStateActive
	p.UpdatedAt = time.Now()

	return m.store.UpdateGovernanceProposal(ctx, toStoreProposal(p))
}

// CastVote records a vote on a proposal.
func (m *Manager) CastVote(ctx context.Context, v *Vote) error {
	sp, err := m.store.GetGovernanceProposal(ctx, v.ProposalID)
	if err != nil {
		return err
	}
	if sp == nil {
		return fmt.Errorf("proposal %s not found", v.ProposalID)
	}

	p := fromStoreProposal(sp)

	if p.State != ProposalStateActive {
		return fmt.Errorf("proposal %s is not active for voting", v.ProposalID)
	}

	now := time.Now()
	if now.Before(p.VotingStart) {
		return fmt.Errorf("voting has not started yet")
	}
	if now.After(p.VotingEnd) {
		return fmt.Errorf("voting period has ended")
	}

	if v.Choice != VoteYes && v.Choice != VoteNo && v.Choice != VoteAbstain {
		return fmt.Errorf("invalid vote choice: %s", v.Choice)
	}

	existingVote, err := m.store.GetGovernanceVote(ctx, v.ProposalID, v.VoterDID)
	if err != nil {
		return err
	}
	if existingVote != nil {
		return fmt.Errorf("voter %s has already voted on proposal %s", v.VoterDID, v.ProposalID)
	}

	if v.VoteID == "" {
		v.VoteID = generateVoteID(v.ProposalID, v.VoterDID, now)
	}
	v.CreatedAt = now

	if err := m.store.CreateGovernanceVote(ctx, toStoreVote(v)); err != nil {
		return err
	}

	switch v.Choice {
	case VoteYes:
		p.YesVotes++
	case VoteNo:
		p.NoVotes++
	case VoteAbstain:
		p.AbstainVotes++
	}
	p.UpdatedAt = now

	return m.store.UpdateGovernanceProposal(ctx, toStoreProposal(p))
}

// TallyProposal calculates the final result and updates proposal state.
func (m *Manager) TallyProposal(ctx context.Context, proposalID string) error {
	sp, err := m.store.GetGovernanceProposal(ctx, proposalID)
	if err != nil {
		return err
	}
	if sp == nil {
		return fmt.Errorf("proposal %s not found", proposalID)
	}

	p := fromStoreProposal(sp)

	if p.State != ProposalStateActive {
		return fmt.Errorf("proposal %s is not active", proposalID)
	}
	if time.Now().Before(p.VotingEnd) {
		return fmt.Errorf("voting period has not ended yet")
	}

	eligibleVoters, err := m.store.CountEligibleVoters(ctx)
	if err != nil {
		return err
	}
	if eligibleVoters == 0 {
		return fmt.Errorf("no eligible voters found")
	}

	totalVotes := p.YesVotes + p.NoVotes + p.AbstainVotes
	participationPct := (totalVotes * 100) / eligibleVoters

	if participationPct < p.QuorumPct {
		p.State = ProposalStateRejected
		p.UpdatedAt = time.Now()
		return m.store.UpdateGovernanceProposal(ctx, toStoreProposal(p))
	}

	votesForDecision := p.YesVotes + p.NoVotes
	if votesForDecision == 0 {
		p.State = ProposalStateRejected
		p.UpdatedAt = time.Now()
		return m.store.UpdateGovernanceProposal(ctx, toStoreProposal(p))
	}

	yesPct := (p.YesVotes * 100) / votesForDecision

	if yesPct >= p.PassPct {
		p.State = ProposalStatePassed
	} else {
		p.State = ProposalStateRejected
	}
	p.UpdatedAt = time.Now()

	return m.store.UpdateGovernanceProposal(ctx, toStoreProposal(p))
}

// ExecuteProposal marks a passed proposal as executed.
func (m *Manager) ExecuteProposal(ctx context.Context, proposalID string) error {
	sp, err := m.store.GetGovernanceProposal(ctx, proposalID)
	if err != nil {
		return err
	}
	if sp == nil {
		return fmt.Errorf("proposal %s not found", proposalID)
	}

	p := fromStoreProposal(sp)
	if p.State != ProposalStatePassed {
		return fmt.Errorf("proposal %s has not passed", proposalID)
	}

	now := time.Now()
	p.State = ProposalStateExecuted
	p.UpdatedAt = now
	p.ExecutedAt = &now

	return m.store.UpdateGovernanceProposal(ctx, toStoreProposal(p))
}

// GetProposal retrieves a proposal by ID.
func (m *Manager) GetProposal(ctx context.Context, proposalID string) (*Proposal, error) {
	sp, err := m.store.GetGovernanceProposal(ctx, proposalID)
	if err != nil {
		return nil, err
	}
	return fromStoreProposal(sp), nil
}

// ListProposals returns all proposals matching the given state filter.
func (m *Manager) ListProposals(ctx context.Context, state ProposalState, limit, offset int) ([]*Proposal, error) {
	sps, err := m.store.ListGovernanceProposals(ctx, string(state), limit, offset)
	if err != nil {
		return nil, err
	}
	result := make([]*Proposal, len(sps))
	for i, sp := range sps {
		result[i] = fromStoreProposal(sp)
	}
	return result, nil
}

// GetVotesForProposal returns all votes cast on a proposal.
func (m *Manager) GetVotesForProposal(ctx context.Context, proposalID string) ([]*Vote, error) {
	svs, err := m.store.ListGovernanceVotes(ctx, proposalID)
	if err != nil {
		return nil, err
	}
	result := make([]*Vote, len(svs))
	for i, sv := range svs {
		result[i] = fromStoreVote(sv)
	}
	return result, nil
}

// --- ID generators ---

func generateProposalID(proposerDID, title string, t time.Time) string {
	data := fmt.Sprintf("%s:%s:%d", proposerDID, title, t.UnixNano())
	hash := sha256.Sum256([]byte(data))
	return "prop_" + hex.EncodeToString(hash[:8])
}

func generateVoteID(proposalID, voterDID string, t time.Time) string {
	data := fmt.Sprintf("%s:%s:%d", proposalID, voterDID, t.UnixNano())
	hash := sha256.Sum256([]byte(data))
	return "vote_" + hex.EncodeToString(hash[:8])
}

// --- Store ↔ domain converters ---

func toStoreProposal(p *Proposal) *store.GovernanceProposal {
	return &store.GovernanceProposal{
		ProposalID:   p.ProposalID,
		ProposerDID:  p.ProposerDID,
		Title:        p.Title,
		Description:  p.Description,
		ProposalType: string(p.ProposalType),
		State:        string(p.State),
		VotingStart:  p.VotingStart,
		VotingEnd:    p.VotingEnd,
		QuorumPct:    p.QuorumPct,
		PassPct:      p.PassPct,
		YesVotes:     p.YesVotes,
		NoVotes:      p.NoVotes,
		AbstainVotes: p.AbstainVotes,
		CreatedAt:    p.CreatedAt,
		UpdatedAt:    p.UpdatedAt,
		ExecutedAt:   p.ExecutedAt,
	}
}

func fromStoreProposal(sp *store.GovernanceProposal) *Proposal {
	if sp == nil {
		return nil
	}
	return &Proposal{
		ProposalID:   sp.ProposalID,
		ProposerDID:  sp.ProposerDID,
		Title:        sp.Title,
		Description:  sp.Description,
		ProposalType: ProposalType(sp.ProposalType),
		State:        ProposalState(sp.State),
		VotingStart:  sp.VotingStart,
		VotingEnd:    sp.VotingEnd,
		QuorumPct:    sp.QuorumPct,
		PassPct:      sp.PassPct,
		YesVotes:     sp.YesVotes,
		NoVotes:      sp.NoVotes,
		AbstainVotes: sp.AbstainVotes,
		CreatedAt:    sp.CreatedAt,
		UpdatedAt:    sp.UpdatedAt,
		ExecutedAt:   sp.ExecutedAt,
	}
}

func toStoreVote(v *Vote) *store.GovernanceVote {
	return &store.GovernanceVote{
		VoteID:     v.VoteID,
		ProposalID: v.ProposalID,
		VoterDID:   v.VoterDID,
		Choice:     string(v.Choice),
		Signature:  v.Signature,
		CreatedAt:  v.CreatedAt,
	}
}

func fromStoreVote(sv *store.GovernanceVote) *Vote {
	if sv == nil {
		return nil
	}
	return &Vote{
		VoteID:     sv.VoteID,
		ProposalID: sv.ProposalID,
		VoterDID:   sv.VoterDID,
		Choice:     VoteChoice(sv.Choice),
		Signature:  sv.Signature,
		CreatedAt:  sv.CreatedAt,
	}
}
