# Contract System Integration Plan

**Date:** 2026-02-10
**Status:** Ready to Begin
**Estimated Time:** 24-32 hours

---

## Overview

Based on the earlier discussion comparing SoHoLINK to AgriNet's seasonal planning system, we need to implement a contract-based resource allocation system for the federated marketplace.

### Key Insight from AgriNet Comparison

**AgriNet Model:**
- Users plan seasonal meals (pumpkin pie, turkey, wheat)
- Each resource has a lead time (pumpkins = 85-120 days)
- System coordinates distributed production across farms
- Contracts establish agreements with planning distance

**SoHoLINK Application:**
- Users request compute resources (VMs, storage, bandwidth)
- Each resource has setup/provisioning lead time
- System coordinates distributed resources across nodes
- Contracts establish agreements with federated providers

---

## Why Contracts Are Critical

### Problem Without Contracts
```
User: "I need 10 VMs right now!"
System: Searches available nodes...
Result: Maybe available, maybe not. No guarantees.
```

### Solution With Contracts
```
User: "I need 10 VMs in 7 days for 30 days duration"
System: Creates contract request
Providers: Review and accept/reject
Result: Guaranteed resources at agreed time/price
```

### Benefits
1. **Planning Distance** - Providers have time to provision resources
2. **Price Discovery** - Market-based pricing through bids
3. **Guarantees** - SLA enforcement through contracts
4. **Trust** - Cryptographic signatures (Ed25519) for all parties
5. **Federation** - Works across organizational boundaries
6. **Governance** - Policy engine validates contracts

---

## Architecture

### Contract Lifecycle

```
┌─────────────┐
│  REQUESTED  │ ← User creates contract request
└──────┬──────┘
       │
       v
┌─────────────┐
│   PENDING   │ ← Providers review and bid
└──────┬──────┘
       │
       v
┌─────────────┐
│  ACCEPTED   │ ← Provider accepts, user confirms
└──────┬──────┘
       │
       v
┌─────────────┐
│   ACTIVE    │ ← Resources provisioned and running
└──────┬──────┘
       │
       v
┌─────────────┐
│  COMPLETED  │ ← Contract fulfilled
└─────────────┘
       │
       ├── CANCELLED (by user)
       ├── REJECTED (by provider)
       └── EXPIRED (timeout)
```

---

## Data Model

### Contract Structure

```go
type Contract struct {
    // Identifiers
    ID          string    // UUID for contract
    UserDID     string    // User's decentralized identifier
    ProviderDID string    // Provider's decentralized identifier

    // Resource Requirements
    Resources   ResourceRequirements

    // Timeline
    RequestTime   time.Time  // When contract was created
    StartTime     time.Time  // When resources should be available
    Duration      time.Duration  // How long resources are needed
    LeadTime      time.Duration  // Planning distance (StartTime - RequestTime)

    // Pricing
    ProposedPrice *Price    // User's proposed price (optional)
    AcceptedPrice *Price    // Final agreed price

    // Lifecycle
    State         ContractState
    CreatedAt     time.Time
    UpdatedAt     time.Time

    // Signatures (Ed25519)
    UserSignature     []byte  // User signs request
    ProviderSignature []byte  // Provider signs acceptance

    // SLA
    SLA *ServiceLevelAgreement
}

type ResourceRequirements struct {
    VMCount       int
    CPUCores      int       // Per VM
    MemoryGB      int       // Per VM
    StorageGB     int       // Per VM
    BandwidthMbps int       // Total
    Region        string    // Geographic preference (optional)
}

type Price struct {
    Amount   decimal.Decimal
    Currency string  // "USD", "BTC", etc.
    Period   string  // "hourly", "daily", "total"
}

type ServiceLevelAgreement struct {
    UptimePercent    float64  // e.g., 99.9%
    MaxLatencyMs     int      // e.g., 100ms
    SupportLevel     string   // "basic", "premium"
    PenaltyRate      float64  // % refund per SLA violation
}

type ContractState string

const (
    ContractStateRequested ContractState = "REQUESTED"
    ContractStatePending   ContractState = "PENDING"
    ContractStateAccepted  ContractState = "ACCEPTED"
    ContractStateActive    ContractState = "ACTIVE"
    ContractStateCompleted ContractState = "COMPLETED"
    ContractStateCancelled ContractState = "CANCELLED"
    ContractStateRejected  ContractState = "REJECTED"
    ContractStateExpired   ContractState = "EXPIRED"
)
```

---

## Database Schema

### SQLite Schema

```sql
CREATE TABLE contracts (
    id TEXT PRIMARY KEY,
    user_did TEXT NOT NULL,
    provider_did TEXT,

    -- Resources
    vm_count INTEGER NOT NULL,
    cpu_cores INTEGER NOT NULL,
    memory_gb INTEGER NOT NULL,
    storage_gb INTEGER NOT NULL,
    bandwidth_mbps INTEGER NOT NULL,
    region TEXT,

    -- Timeline
    request_time TIMESTAMP NOT NULL,
    start_time TIMESTAMP NOT NULL,
    duration_seconds INTEGER NOT NULL,
    lead_time_seconds INTEGER NOT NULL,

    -- Pricing
    proposed_amount TEXT,
    proposed_currency TEXT,
    accepted_amount TEXT,
    accepted_currency TEXT,
    price_period TEXT,

    -- SLA
    uptime_percent REAL,
    max_latency_ms INTEGER,
    support_level TEXT,
    penalty_rate REAL,

    -- Lifecycle
    state TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,

    -- Signatures
    user_signature BLOB,
    provider_signature BLOB,

    -- Indexes
    FOREIGN KEY (user_did) REFERENCES identities(did),
    FOREIGN KEY (provider_did) REFERENCES identities(did)
);

CREATE INDEX idx_contracts_user ON contracts(user_did);
CREATE INDEX idx_contracts_provider ON contracts(provider_did);
CREATE INDEX idx_contracts_state ON contracts(state);
CREATE INDEX idx_contracts_start_time ON contracts(start_time);

CREATE TABLE contract_bids (
    id TEXT PRIMARY KEY,
    contract_id TEXT NOT NULL,
    provider_did TEXT NOT NULL,

    amount TEXT NOT NULL,
    currency TEXT NOT NULL,

    message TEXT,  -- Optional note from provider
    created_at TIMESTAMP NOT NULL,

    FOREIGN KEY (contract_id) REFERENCES contracts(id),
    FOREIGN KEY (provider_did) REFERENCES identities(did)
);

CREATE INDEX idx_bids_contract ON contract_bids(contract_id);

CREATE TABLE contract_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    contract_id TEXT NOT NULL,
    event_type TEXT NOT NULL,  -- "created", "bid_received", "accepted", "started", etc.
    actor_did TEXT,             -- Who caused the event
    details TEXT,               -- JSON with additional info
    timestamp TIMESTAMP NOT NULL,

    FOREIGN KEY (contract_id) REFERENCES contracts(id)
);

CREATE INDEX idx_events_contract ON contract_events(contract_id);
CREATE INDEX idx_events_timestamp ON contract_events(timestamp);
```

---

## Implementation Phases

### Phase 1: Core Contract Data Model (4-6 hours)

**Files to Create:**
1. `internal/contracts/types.go` - Contract data structures
2. `internal/contracts/database.go` - SQLite persistence
3. `internal/contracts/database_test.go` - Database tests
4. `internal/contracts/validation.go` - Contract validation logic

**Tasks:**
- [ ] Define Contract struct and related types
- [ ] Implement SQLite schema and migrations
- [ ] Create CRUD operations (Create, Read, Update, Delete)
- [ ] Add contract validation (lead time, resources, pricing)
- [ ] Write unit tests

### Phase 2: Contract Lifecycle Manager (6-8 hours)

**Files to Create:**
1. `internal/contracts/lifecycle.go` - State machine
2. `internal/contracts/lifecycle_test.go` - Lifecycle tests
3. `internal/contracts/signatures.go` - Ed25519 signing/verification

**Tasks:**
- [ ] Implement state machine (REQUESTED → PENDING → ACCEPTED → ACTIVE → COMPLETED)
- [ ] Add state transition validation
- [ ] Implement signature generation/verification
- [ ] Add expiration handling
- [ ] Write state machine tests

### Phase 3: Bidding System (4-6 hours)

**Files to Create:**
1. `internal/contracts/bidding.go` - Bid management
2. `internal/contracts/bidding_test.go` - Bidding tests

**Tasks:**
- [ ] Implement bid submission
- [ ] Add bid acceptance logic
- [ ] Implement price negotiation
- [ ] Add bid expiration
- [ ] Write bidding tests

### Phase 4: Policy Integration (3-4 hours)

**Files to Modify:**
1. `internal/policy/policy.rego` - Add contract policies

**Tasks:**
- [ ] Define contract creation policies (who can create?)
- [ ] Define contract acceptance policies (resource limits?)
- [ ] Add governance rules (max duration, min lead time)
- [ ] Integrate with existing policy engine
- [ ] Test policy enforcement

### Phase 5: API Endpoints (4-6 hours)

**Files to Create:**
1. `internal/api/contracts.go` - HTTP handlers
2. `internal/api/contracts_test.go` - API tests

**API Endpoints:**
```
POST   /api/v1/contracts              - Create contract request
GET    /api/v1/contracts              - List contracts (user or provider)
GET    /api/v1/contracts/:id          - Get contract details
PUT    /api/v1/contracts/:id/accept   - Provider accepts contract
PUT    /api/v1/contracts/:id/reject   - Provider rejects contract
DELETE /api/v1/contracts/:id          - Cancel contract (user only)

POST   /api/v1/contracts/:id/bids     - Submit bid (provider)
GET    /api/v1/contracts/:id/bids     - List bids for contract
PUT    /api/v1/contracts/:id/bids/:bid_id/accept - Accept bid (user)
```

### Phase 6: UI Integration (3-4 hours)

**Files to Create:**
1. `ui/contracts/contract_create.go` - Contract creation form
2. `ui/contracts/contract_list.go` - Contract dashboard
3. `ui/contracts/contract_detail.go` - Contract details view

**UI Components:**
- Contract request form (resources, timeline, pricing)
- Contract dashboard (active, pending, completed)
- Bid review interface (for users)
- Contract acceptance interface (for providers)

---

## Integration Points

### Existing Systems to Integrate With

1. **Policy Engine** (`internal/policy`)
   - Validate contract requests against governance rules
   - Check user permissions (can they create contracts?)
   - Enforce resource limits (max VMs, max duration)

2. **DID System** (`internal/did`)
   - Use Ed25519 keys for signing contracts
   - Verify signatures from both parties
   - Link contracts to decentralized identities

3. **RADIUS/Authentication** (`internal/radius`)
   - Enforce contracts during network access
   - Check if user has active contract before allowing VMs
   - Meter usage against contract limits

4. **Accounting** (`internal/accounting`)
   - Track contract events (created, accepted, started, completed)
   - Record resource usage against contracts
   - Generate billing from contract prices

5. **Compute** (`internal/compute`)
   - Provision VMs based on accepted contracts
   - Enforce resource limits from contract
   - Auto-provision at contract start time

---

## Testing Strategy

### Unit Tests
- Contract validation logic
- State machine transitions
- Signature generation/verification
- Database CRUD operations
- Bidding logic

### Integration Tests
- Full contract lifecycle (request → accept → active → complete)
- Policy enforcement during contract creation
- Signature verification across DID system
- Database persistence and retrieval

### End-to-End Tests
- User creates contract via UI
- Provider reviews and bids via API
- User accepts bid
- System provisions resources
- Contract completes and billing generated

---

## Configuration

### Contract System Config

```yaml
contracts:
  enabled: true

  # Defaults for contract parameters
  defaults:
    min_lead_time: "24h"        # Minimum planning distance
    max_duration: "720h"         # Max 30 days
    max_vms: 100                 # Max VMs per contract
    bid_timeout: "168h"          # 7 days for bids

  # Governance rules
  governance:
    require_approval: false      # Contracts auto-accepted or need approval?
    min_user_reputation: 0.0     # Minimum reputation to create contracts

  # Pricing
  pricing:
    allow_user_proposed: true    # Can users propose prices?
    require_bids: false          # Must have bids or can direct-accept?
    default_currency: "USD"
```

---

## Security Considerations

### Signature Verification
- All contracts MUST be signed by both parties (Ed25519)
- Signature verification happens before state transitions
- Invalid signatures = contract rejected

### Resource Limits
- Policy engine enforces max VMs, CPU, memory per contract
- Prevents resource exhaustion attacks
- Users can't exceed their quota

### Price Validation
- Reasonable price ranges enforced
- Prevents extreme pricing (too high or too low)
- Currency validation (only supported currencies)

### Lead Time Enforcement
- Minimum lead time prevents instant provisioning abuse
- Maximum duration prevents indefinite resource holding
- Expiration handling prevents stale contracts

---

## Migration from Existing System

### Current State
- `internal/rental/autoaccept.go` exists with basic auto-accept logic
- No formal contract system
- No bidding mechanism
- No SLA enforcement

### Migration Path
1. Keep `autoaccept.go` for backward compatibility
2. Add contract system as new feature
3. Add config flag: `use_contracts: true/false`
4. Gradually migrate users to contract system
5. Eventually deprecate autoaccept

**Code Example:**
```go
func (s *Server) HandleResourceRequest(req *Request) error {
    if s.config.UseContracts {
        return s.contractManager.CreateContract(req)
    } else {
        return s.autoAcceptor.Handle(req)  // Legacy path
    }
}
```

---

## Success Criteria

- [ ] Users can create contract requests via UI
- [ ] Providers can review and bid on contracts
- [ ] Users can accept bids and finalize contracts
- [ ] System auto-provisions resources at contract start time
- [ ] All contracts are cryptographically signed (Ed25519)
- [ ] Policy engine enforces governance rules
- [ ] Accounting tracks contract events
- [ ] Full test coverage (unit, integration, e2e)
- [ ] Documentation complete

---

## Timeline Estimate

| Phase | Task | Hours |
|-------|------|-------|
| 1 | Core data model | 4-6 |
| 2 | Lifecycle manager | 6-8 |
| 3 | Bidding system | 4-6 |
| 4 | Policy integration | 3-4 |
| 5 | API endpoints | 4-6 |
| 6 | UI integration | 3-4 |

**Total:** 24-34 hours (3-4 full days)

---

## Next Steps

1. **Review this plan** - Confirm approach aligns with vision
2. **Start Phase 1** - Create core contract data model
3. **Iterate** - Build incrementally with tests
4. **Deploy** - Roll out to federated network

---

**Ready to begin contract system integration!** 🚀
