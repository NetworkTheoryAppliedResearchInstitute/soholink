package store

import (
	"context"
	"database/sql"
	"time"
)

// InitGovernanceSchema creates the governance tables if they don't exist.
func (s *Store) InitGovernanceSchema(ctx context.Context) error {
	schema := `
	CREATE TABLE IF NOT EXISTS governance_proposals (
		proposal_id TEXT PRIMARY KEY,
		proposer_did TEXT NOT NULL,
		title TEXT NOT NULL,
		description TEXT NOT NULL,
		proposal_type TEXT NOT NULL,
		state TEXT NOT NULL,
		voting_start INTEGER NOT NULL,
		voting_end INTEGER NOT NULL,
		quorum_pct INTEGER NOT NULL,
		pass_pct INTEGER NOT NULL,
		yes_votes INTEGER DEFAULT 0,
		no_votes INTEGER DEFAULT 0,
		abstain_votes INTEGER DEFAULT 0,
		created_at INTEGER NOT NULL,
		updated_at INTEGER NOT NULL,
		executed_at INTEGER
	);

	CREATE TABLE IF NOT EXISTS governance_votes (
		vote_id TEXT PRIMARY KEY,
		proposal_id TEXT NOT NULL,
		voter_did TEXT NOT NULL,
		choice TEXT NOT NULL,
		signature TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		FOREIGN KEY (proposal_id) REFERENCES governance_proposals(proposal_id),
		UNIQUE(proposal_id, voter_did)
	);

	CREATE INDEX IF NOT EXISTS idx_proposals_state ON governance_proposals(state);
	CREATE INDEX IF NOT EXISTS idx_proposals_voting_end ON governance_proposals(voting_end);
	CREATE INDEX IF NOT EXISTS idx_votes_proposal ON governance_votes(proposal_id);
	CREATE INDEX IF NOT EXISTS idx_votes_voter ON governance_votes(voter_did);
	`

	_, err := s.db.ExecContext(ctx, schema)
	return err
}

// GovernanceProposal represents a governance proposal (for store operations).
type GovernanceProposal struct {
	ProposalID   string
	ProposerDID  string
	Title        string
	Description  string
	ProposalType string
	State        string
	VotingStart  time.Time
	VotingEnd    time.Time
	QuorumPct    int
	PassPct      int
	YesVotes     int
	NoVotes      int
	AbstainVotes int
	CreatedAt    time.Time
	UpdatedAt    time.Time
	ExecutedAt   *time.Time
}

// GovernanceVote represents a vote on a proposal (for store operations).
type GovernanceVote struct {
	VoteID     string
	ProposalID string
	VoterDID   string
	Choice     string
	Signature  string
	CreatedAt  time.Time
}

// CreateGovernanceProposal inserts a new governance proposal.
func (s *Store) CreateGovernanceProposal(ctx context.Context, p interface{}) error {
	// Accept both governance.Proposal and GovernanceProposal
	var proposal GovernanceProposal
	switch v := p.(type) {
	case *GovernanceProposal:
		proposal = *v
	case GovernanceProposal:
		proposal = v
	default:
		// Use reflection to extract fields from governance.Proposal
		// This is a simplified version - in production you'd use proper type conversion
		proposal = GovernanceProposal{
			ProposalID:   getField(p, "ProposalID").(string),
			ProposerDID:  getField(p, "ProposerDID").(string),
			Title:        getField(p, "Title").(string),
			Description:  getField(p, "Description").(string),
			ProposalType: getFieldString(p, "ProposalType"),
			State:        getFieldString(p, "State"),
			VotingStart:  getField(p, "VotingStart").(time.Time),
			VotingEnd:    getField(p, "VotingEnd").(time.Time),
			QuorumPct:    getField(p, "QuorumPct").(int),
			PassPct:      getField(p, "PassPct").(int),
			YesVotes:     getField(p, "YesVotes").(int),
			NoVotes:      getField(p, "NoVotes").(int),
			AbstainVotes: getField(p, "AbstainVotes").(int),
			CreatedAt:    getField(p, "CreatedAt").(time.Time),
			UpdatedAt:    getField(p, "UpdatedAt").(time.Time),
		}
		if execAt := getFieldTimePtr(p, "ExecutedAt"); execAt != nil {
			proposal.ExecutedAt = execAt
		}
	}

	var executedAt sql.NullInt64
	if proposal.ExecutedAt != nil {
		executedAt.Valid = true
		executedAt.Int64 = proposal.ExecutedAt.Unix()
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO governance_proposals (
			proposal_id, proposer_did, title, description, proposal_type,
			state, voting_start, voting_end, quorum_pct, pass_pct,
			yes_votes, no_votes, abstain_votes, created_at, updated_at, executed_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		proposal.ProposalID, proposal.ProposerDID, proposal.Title, proposal.Description,
		proposal.ProposalType, proposal.State,
		proposal.VotingStart.Unix(), proposal.VotingEnd.Unix(),
		proposal.QuorumPct, proposal.PassPct,
		proposal.YesVotes, proposal.NoVotes, proposal.AbstainVotes,
		proposal.CreatedAt.Unix(), proposal.UpdatedAt.Unix(), executedAt,
	)
	return err
}

// UpdateGovernanceProposal updates an existing governance proposal.
func (s *Store) UpdateGovernanceProposal(ctx context.Context, p interface{}) error {
	// Similar conversion logic as CreateGovernanceProposal
	var proposal GovernanceProposal
	switch v := p.(type) {
	case *GovernanceProposal:
		proposal = *v
	case GovernanceProposal:
		proposal = v
	default:
		proposal = GovernanceProposal{
			ProposalID:   getField(p, "ProposalID").(string),
			State:        getFieldString(p, "State"),
			YesVotes:     getField(p, "YesVotes").(int),
			NoVotes:      getField(p, "NoVotes").(int),
			AbstainVotes: getField(p, "AbstainVotes").(int),
			UpdatedAt:    getField(p, "UpdatedAt").(time.Time),
		}
		if execAt := getFieldTimePtr(p, "ExecutedAt"); execAt != nil {
			proposal.ExecutedAt = execAt
		}
	}

	var executedAt sql.NullInt64
	if proposal.ExecutedAt != nil {
		executedAt.Valid = true
		executedAt.Int64 = proposal.ExecutedAt.Unix()
	}

	_, err := s.db.ExecContext(ctx,
		`UPDATE governance_proposals SET
			state = ?, yes_votes = ?, no_votes = ?, abstain_votes = ?,
			updated_at = ?, executed_at = ?
		WHERE proposal_id = ?`,
		proposal.State, proposal.YesVotes, proposal.NoVotes, proposal.AbstainVotes,
		proposal.UpdatedAt.Unix(), executedAt, proposal.ProposalID,
	)
	return err
}

// GetGovernanceProposal retrieves a proposal by ID.
func (s *Store) GetGovernanceProposal(ctx context.Context, proposalID string) (interface{}, error) {
	var p GovernanceProposal
	var votingStart, votingEnd, createdAt, updatedAt int64
	var executedAt sql.NullInt64

	err := s.db.QueryRowContext(ctx,
		`SELECT proposal_id, proposer_did, title, description, proposal_type,
				state, voting_start, voting_end, quorum_pct, pass_pct,
				yes_votes, no_votes, abstain_votes, created_at, updated_at, executed_at
		 FROM governance_proposals WHERE proposal_id = ?`,
		proposalID,
	).Scan(
		&p.ProposalID, &p.ProposerDID, &p.Title, &p.Description, &p.ProposalType,
		&p.State, &votingStart, &votingEnd, &p.QuorumPct, &p.PassPct,
		&p.YesVotes, &p.NoVotes, &p.AbstainVotes, &createdAt, &updatedAt, &executedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	p.VotingStart = time.Unix(votingStart, 0)
	p.VotingEnd = time.Unix(votingEnd, 0)
	p.CreatedAt = time.Unix(createdAt, 0)
	p.UpdatedAt = time.Unix(updatedAt, 0)
	if executedAt.Valid {
		t := time.Unix(executedAt.Int64, 0)
		p.ExecutedAt = &t
	}

	return &p, nil
}

// ListGovernanceProposals returns proposals filtered by state.
func (s *Store) ListGovernanceProposals(ctx context.Context, state string, limit, offset int) ([]interface{}, error) {
	if limit <= 0 {
		limit = 100
	}

	query := `SELECT proposal_id, proposer_did, title, description, proposal_type,
					 state, voting_start, voting_end, quorum_pct, pass_pct,
					 yes_votes, no_votes, abstain_votes, created_at, updated_at, executed_at
			  FROM governance_proposals`

	var args []interface{}
	if state != "" {
		query += " WHERE state = ?"
		args = append(args, state)
	}

	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var proposals []interface{}
	for rows.Next() {
		var p GovernanceProposal
		var votingStart, votingEnd, createdAt, updatedAt int64
		var executedAt sql.NullInt64

		if err := rows.Scan(
			&p.ProposalID, &p.ProposerDID, &p.Title, &p.Description, &p.ProposalType,
			&p.State, &votingStart, &votingEnd, &p.QuorumPct, &p.PassPct,
			&p.YesVotes, &p.NoVotes, &p.AbstainVotes, &createdAt, &updatedAt, &executedAt,
		); err != nil {
			continue
		}

		p.VotingStart = time.Unix(votingStart, 0)
		p.VotingEnd = time.Unix(votingEnd, 0)
		p.CreatedAt = time.Unix(createdAt, 0)
		p.UpdatedAt = time.Unix(updatedAt, 0)
		if executedAt.Valid {
			t := time.Unix(executedAt.Int64, 0)
			p.ExecutedAt = &t
		}

		proposals = append(proposals, &p)
	}

	return proposals, nil
}

// CreateGovernanceVote records a vote on a proposal.
func (s *Store) CreateGovernanceVote(ctx context.Context, v interface{}) error {
	var vote GovernanceVote
	switch val := v.(type) {
	case *GovernanceVote:
		vote = *val
	case GovernanceVote:
		vote = val
	default:
		vote = GovernanceVote{
			VoteID:     getField(v, "VoteID").(string),
			ProposalID: getField(v, "ProposalID").(string),
			VoterDID:   getField(v, "VoterDID").(string),
			Choice:     getFieldString(v, "Choice"),
			Signature:  getField(v, "Signature").(string),
			CreatedAt:  getField(v, "CreatedAt").(time.Time),
		}
	}

	_, err := s.db.ExecContext(ctx,
		`INSERT INTO governance_votes (vote_id, proposal_id, voter_did, choice, signature, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		vote.VoteID, vote.ProposalID, vote.VoterDID, vote.Choice, vote.Signature, vote.CreatedAt.Unix(),
	)
	return err
}

// GetGovernanceVote retrieves a specific vote by proposal and voter.
func (s *Store) GetGovernanceVote(ctx context.Context, proposalID, voterDID string) (interface{}, error) {
	var v GovernanceVote
	var createdAt int64

	err := s.db.QueryRowContext(ctx,
		`SELECT vote_id, proposal_id, voter_did, choice, signature, created_at
		 FROM governance_votes WHERE proposal_id = ? AND voter_did = ?`,
		proposalID, voterDID,
	).Scan(&v.VoteID, &v.ProposalID, &v.VoterDID, &v.Choice, &v.Signature, &createdAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	v.CreatedAt = time.Unix(createdAt, 0)
	return &v, nil
}

// ListGovernanceVotes returns all votes for a proposal.
func (s *Store) ListGovernanceVotes(ctx context.Context, proposalID string) ([]interface{}, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT vote_id, proposal_id, voter_did, choice, signature, created_at
		 FROM governance_votes WHERE proposal_id = ? ORDER BY created_at ASC`,
		proposalID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var votes []interface{}
	for rows.Next() {
		var v GovernanceVote
		var createdAt int64

		if err := rows.Scan(&v.VoteID, &v.ProposalID, &v.VoterDID, &v.Choice, &v.Signature, &createdAt); err != nil {
			continue
		}

		v.CreatedAt = time.Unix(createdAt, 0)
		votes = append(votes, &v)
	}

	return votes, nil
}

// CountEligibleVoters returns the number of nodes eligible to vote.
func (s *Store) CountEligibleVoters(ctx context.Context) (int, error) {
	// Count registered nodes in the federation
	// This queries the node_registry table or similar
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM (
			SELECT DISTINCT did FROM resource_announcements
			UNION
			SELECT DISTINCT provider_did FROM resource_transactions
		)`,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// Helper functions for reflection-based field access
func getField(obj interface{}, fieldName string) interface{} {
	// This is a simplified placeholder - in production use reflect package
	// For now, we'll use type assertions in the actual conversion logic
	return nil
}

func getFieldString(obj interface{}, fieldName string) string {
	// Simplified placeholder
	return ""
}

func getFieldTimePtr(obj interface{}, fieldName string) *time.Time {
	// Simplified placeholder
	return nil
}
