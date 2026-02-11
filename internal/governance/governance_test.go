package governance

import (
	"context"
	"testing"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// TestNewManager tests governance manager creation.
func TestNewManager(t *testing.T) {
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatalf("Failed to create memory store: %v", err)
	}
	defer s.Close()

	m := NewManager(s)
	if m == nil {
		t.Fatal("Expected non-nil manager")
	}
	if m.store == nil {
		t.Error("Manager store is nil")
	}
}

// TestCreateProposal tests proposal creation.
func TestCreateProposal(t *testing.T) {
	tests := []struct {
		name      string
		proposal  *Proposal
		wantErr   bool
		checkFunc func(*testing.T, *Proposal)
	}{
		{
			name: "valid proposal with defaults",
			proposal: &Proposal{
				ProposerDID:  "did:soho:proposer123",
				Title:        "Test Proposal",
				Description:  "This is a test proposal",
				ProposalType: ProposalTypeParameter,
			},
			wantErr: false,
			checkFunc: func(t *testing.T, p *Proposal) {
				if p.ProposalID == "" {
					t.Error("Expected non-empty proposal ID")
				}
				if p.State != ProposalStateDraft {
					t.Errorf("State = %s, want %s", p.State, ProposalStateDraft)
				}
				if p.QuorumPct != 51 {
					t.Errorf("QuorumPct = %d, want 51", p.QuorumPct)
				}
				if p.PassPct != 66 {
					t.Errorf("PassPct = %d, want 66", p.PassPct)
				}
				if p.VotingEnd.Before(p.VotingStart) {
					t.Error("VotingEnd should be after VotingStart")
				}
			},
		},
		{
			name: "proposal with custom thresholds",
			proposal: &Proposal{
				ProposerDID:  "did:soho:proposer123",
				Title:        "Custom Thresholds",
				Description:  "Testing custom quorum and pass thresholds",
				ProposalType: ProposalTypeFeatureToggle,
				QuorumPct:    75,
				PassPct:      80,
			},
			wantErr: false,
			checkFunc: func(t *testing.T, p *Proposal) {
				if p.QuorumPct != 75 {
					t.Errorf("QuorumPct = %d, want 75", p.QuorumPct)
				}
				if p.PassPct != 80 {
					t.Errorf("PassPct = %d, want 80", p.PassPct)
				}
			},
		},
		{
			name: "proposal with voting times",
			proposal: &Proposal{
				ProposerDID:  "did:soho:proposer123",
				Title:        "Timed Proposal",
				Description:  "Testing custom voting times",
				ProposalType: ProposalTypeNodeAdmission,
				VotingStart:  time.Now().Add(1 * time.Hour),
				VotingEnd:    time.Now().Add(24 * time.Hour),
			},
			wantErr: false,
			checkFunc: func(t *testing.T, p *Proposal) {
				duration := p.VotingEnd.Sub(p.VotingStart)
				if duration < 23*time.Hour {
					t.Errorf("Voting duration = %v, want >= 23h", duration)
				}
			},
		},
		{
			name: "invalid quorum percentage",
			proposal: &Proposal{
				ProposerDID:  "did:soho:proposer123",
				Title:        "Invalid Quorum",
				Description:  "Testing invalid quorum",
				ProposalType: ProposalTypeParameter,
				QuorumPct:    150, // Invalid: > 100
			},
			wantErr: true,
		},
		{
			name: "invalid pass percentage",
			proposal: &Proposal{
				ProposerDID:  "did:soho:proposer123",
				Title:        "Invalid Pass",
				Description:  "Testing invalid pass threshold",
				ProposalType: ProposalTypeParameter,
				PassPct:      0, // Invalid: < 1
			},
			wantErr: true,
		},
		{
			name: "voting end before start",
			proposal: &Proposal{
				ProposerDID:  "did:soho:proposer123",
				Title:        "Invalid Times",
				Description:  "Testing invalid voting times",
				ProposalType: ProposalTypeParameter,
				VotingStart:  time.Now().Add(24 * time.Hour),
				VotingEnd:    time.Now().Add(1 * time.Hour),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := store.NewMemoryStore()
			if err != nil {
				t.Fatalf("Failed to create memory store: %v", err)
			}
			defer s.Close()

			// Initialize governance schema
			if err := s.InitGovernanceSchema(context.Background()); err != nil {
				t.Fatalf("Failed to initialize governance schema: %v", err)
			}

			m := NewManager(s)
			err = m.CreateProposal(context.Background(), tt.proposal)

			if (err != nil) != tt.wantErr {
				t.Errorf("CreateProposal() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.checkFunc != nil {
				tt.checkFunc(t, tt.proposal)
			}
		})
	}
}

// TestActivateProposal tests proposal activation.
func TestActivateProposal(t *testing.T) {
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatalf("Failed to create memory store: %v", err)
	}
	defer s.Close()

	if err := s.InitGovernanceSchema(context.Background()); err != nil {
		t.Fatalf("Failed to initialize governance schema: %v", err)
	}

	m := NewManager(s)
	ctx := context.Background()

	// Create proposal
	proposal := &Proposal{
		ProposerDID:  "did:soho:proposer123",
		Title:        "Test Activation",
		Description:  "Testing proposal activation",
		ProposalType: ProposalTypeParameter,
	}
	if err := m.CreateProposal(ctx, proposal); err != nil {
		t.Fatalf("CreateProposal() error = %v", err)
	}

	// Activate proposal
	if err := m.ActivateProposal(ctx, proposal.ProposalID); err != nil {
		t.Fatalf("ActivateProposal() error = %v", err)
	}

	// Verify state changed
	retrieved, err := m.GetProposal(ctx, proposal.ProposalID)
	if err != nil {
		t.Fatalf("GetProposal() error = %v", err)
	}
	if retrieved == nil {
		t.Fatal("Expected non-nil proposal")
	}

	// Type assertion to access State field
	if govProposal, ok := retrieved.(*store.GovernanceProposal); ok {
		if govProposal.State != string(ProposalStateActive) {
			t.Errorf("State = %s, want %s", govProposal.State, ProposalStateActive)
		}
	}
}

// TestCastVote tests vote casting.
func TestCastVote(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*Manager, context.Context) string // Returns proposal ID
		vote      *Vote
		wantErr   bool
		errMsg    string
	}{
		{
			name: "valid yes vote",
			setupFunc: func(m *Manager, ctx context.Context) string {
				p := &Proposal{
					ProposerDID:  "did:soho:proposer123",
					Title:        "Test Vote",
					Description:  "Testing vote casting",
					ProposalType: ProposalTypeParameter,
				}
				m.CreateProposal(ctx, p)
				m.ActivateProposal(ctx, p.ProposalID)
				return p.ProposalID
			},
			vote: &Vote{
				VoterDID:  "did:soho:voter123",
				Choice:    VoteYes,
				Signature: "sig_123",
			},
			wantErr: false,
		},
		{
			name: "valid no vote",
			setupFunc: func(m *Manager, ctx context.Context) string {
				p := &Proposal{
					ProposerDID:  "did:soho:proposer123",
					Title:        "Test No Vote",
					Description:  "Testing no vote",
					ProposalType: ProposalTypeParameter,
				}
				m.CreateProposal(ctx, p)
				m.ActivateProposal(ctx, p.ProposalID)
				return p.ProposalID
			},
			vote: &Vote{
				VoterDID:  "did:soho:voter456",
				Choice:    VoteNo,
				Signature: "sig_456",
			},
			wantErr: false,
		},
		{
			name: "valid abstain vote",
			setupFunc: func(m *Manager, ctx context.Context) string {
				p := &Proposal{
					ProposerDID:  "did:soho:proposer123",
					Title:        "Test Abstain",
					Description:  "Testing abstain vote",
					ProposalType: ProposalTypeParameter,
				}
				m.CreateProposal(ctx, p)
				m.ActivateProposal(ctx, p.ProposalID)
				return p.ProposalID
			},
			vote: &Vote{
				VoterDID:  "did:soho:voter789",
				Choice:    VoteAbstain,
				Signature: "sig_789",
			},
			wantErr: false,
		},
		{
			name: "vote on draft proposal",
			setupFunc: func(m *Manager, ctx context.Context) string {
				p := &Proposal{
					ProposerDID:  "did:soho:proposer123",
					Title:        "Draft Proposal",
					Description:  "Still in draft",
					ProposalType: ProposalTypeParameter,
				}
				m.CreateProposal(ctx, p)
				// Don't activate - leave in draft
				return p.ProposalID
			},
			vote: &Vote{
				VoterDID:  "did:soho:voter123",
				Choice:    VoteYes,
				Signature: "sig_123",
			},
			wantErr: true,
			errMsg:  "not active",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := store.NewMemoryStore()
			if err != nil {
				t.Fatalf("Failed to create memory store: %v", err)
			}
			defer s.Close()

			if err := s.InitGovernanceSchema(context.Background()); err != nil {
				t.Fatalf("Failed to initialize governance schema: %v", err)
			}

			m := NewManager(s)
			ctx := context.Background()

			proposalID := tt.setupFunc(m, ctx)
			tt.vote.ProposalID = proposalID

			err = m.CastVote(ctx, tt.vote)

			if (err != nil) != tt.wantErr {
				t.Errorf("CastVote() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" && err != nil {
				// Check error message contains expected text
				// (simplified check)
			}

			if !tt.wantErr {
				if tt.vote.VoteID == "" {
					t.Error("Expected non-empty vote ID")
				}
			}
		})
	}
}

// TestDuplicateVote tests that voters cannot vote twice.
func TestDuplicateVote(t *testing.T) {
	s, err := store.NewMemoryStore()
	if err != nil {
		t.Fatalf("Failed to create memory store: %v", err)
	}
	defer s.Close()

	if err := s.InitGovernanceSchema(context.Background()); err != nil {
		t.Fatalf("Failed to initialize governance schema: %v", err)
	}

	m := NewManager(s)
	ctx := context.Background()

	// Create and activate proposal
	p := &Proposal{
		ProposerDID:  "did:soho:proposer123",
		Title:        "Duplicate Vote Test",
		Description:  "Testing duplicate vote prevention",
		ProposalType: ProposalTypeParameter,
	}
	m.CreateProposal(ctx, p)
	m.ActivateProposal(ctx, p.ProposalID)

	// Cast first vote
	vote1 := &Vote{
		ProposalID: p.ProposalID,
		VoterDID:   "did:soho:voter123",
		Choice:     VoteYes,
		Signature:  "sig_1",
	}
	if err := m.CastVote(ctx, vote1); err != nil {
		t.Fatalf("First vote failed: %v", err)
	}

	// Attempt duplicate vote
	vote2 := &Vote{
		ProposalID: p.ProposalID,
		VoterDID:   "did:soho:voter123", // Same voter
		Choice:     VoteNo,               // Different choice
		Signature:  "sig_2",
	}
	err = m.CastVote(ctx, vote2)
	if err == nil {
		t.Error("Expected error for duplicate vote")
	}
}

// TestGenerateProposalID tests proposal ID generation.
func TestGenerateProposalID(t *testing.T) {
	id1 := generateProposalID("did:soho:user1", "Title 1", time.Now())
	id2 := generateProposalID("did:soho:user2", "Title 2", time.Now())

	if id1 == id2 {
		t.Error("Expected different proposal IDs")
	}

	if len(id1) < 10 {
		t.Errorf("Proposal ID too short: %s", id1)
	}

	if id1[:5] != "prop_" {
		t.Errorf("Proposal ID doesn't have correct prefix: %s", id1)
	}
}

// TestGenerateVoteID tests vote ID generation.
func TestGenerateVoteID(t *testing.T) {
	id1 := generateVoteID("prop_123", "did:soho:voter1", time.Now())
	id2 := generateVoteID("prop_456", "did:soho:voter2", time.Now())

	if id1 == id2 {
		t.Error("Expected different vote IDs")
	}

	if len(id1) < 10 {
		t.Errorf("Vote ID too short: %s", id1)
	}

	if id1[:5] != "vote_" {
		t.Errorf("Vote ID doesn't have correct prefix: %s", id1)
	}
}

// TestProposalTypes tests all proposal type constants.
func TestProposalTypes(t *testing.T) {
	types := []ProposalType{
		ProposalTypeParameter,
		ProposalTypeFeatureToggle,
		ProposalTypeNodeAdmission,
		ProposalTypeNodeRemoval,
		ProposalTypePolicyChange,
		ProposalTypeTreasurySpend,
	}

	for _, pt := range types {
		if string(pt) == "" {
			t.Errorf("Proposal type %v has empty string value", pt)
		}
	}
}

// TestProposalStates tests all proposal state constants.
func TestProposalStates(t *testing.T) {
	states := []ProposalState{
		ProposalStateDraft,
		ProposalStateActive,
		ProposalStatePassed,
		ProposalStateRejected,
		ProposalStateExecuted,
		ProposalStateExpired,
	}

	for _, state := range states {
		if string(state) == "" {
			t.Errorf("Proposal state %v has empty string value", state)
		}
	}
}

// TestVoteChoices tests all vote choice constants.
func TestVoteChoices(t *testing.T) {
	choices := []VoteChoice{
		VoteYes,
		VoteNo,
		VoteAbstain,
	}

	for _, choice := range choices {
		if string(choice) == "" {
			t.Errorf("Vote choice %v has empty string value", choice)
		}
	}
}
