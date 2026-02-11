# Federated Resource Sharing Architecture

**Document Version:** 0.1 DRAFT
**Date:** 2026-02-07
**Author:** NTARI Architecture Team
**Status:** Design Review

## 1. Overview & Goals

This document specifies how compute, storage, printer spooling, and internet access
resources are shared across the SoHoLINK federation while maintaining the core
principles established in the main specification (Sections 1-36).

### 1.1 Design Goals

- **Federation-First:** Resources are discoverable and shareable across the entire federation, not just locally.
- **Sovereignty-Preserved:** Node operators retain full control over what they offer, to whom, and at what price.
- **Blockchain-Anchored:** All resource usage and payments are cryptographically verifiable and tamper-evident.
- **Offline-Capable:** Resource sharing continues during network partitions; settlement occurs when connectivity resumes.
- **Payment-Agnostic:** Support multiple payment rails (Stripe, federation tokens, Bitcoin Lightning, barter/mutual aid).
- **Security-Hardened:** Each subsystem operates in a least-privilege sandbox with defense-in-depth.

### 1.2 Relationship to Existing Architecture

This extends the existing federated AAA system (Sections 1-13) by treating computational
resources as authenticated, authorized, and accounted services:

- **Authentication:** Users authenticate via existing DID-based credentials (Section 10)
- **Authorization:** OPA policies govern resource access (Section 11)
- **Accounting:** Usage events recorded in append-only logs (Section 12)
- **Federation Trust:** Same blockchain provides global truth (Section 9)
- **Storage Integration:** Leverages existing erasure-coded shard system (Sections 19-22)

## 2. Federation Protocol Design

### 2.1 Resource Discovery Protocol

Resource announcements are propagated via the existing gossip protocol (piggybacked on
policy sync). Each node maintains a local cache of announcements in SQLite. Announcements
expire after TTL, requiring re-announcement. Blockchain commits Merkle root of
announcement set every 1000 blocks.

### 2.2 Cross-Node Resource Allocation

- **Compute Jobs:** User submits job + payment proof; provider validates, executes in sandbox, returns result + usage receipt.
- **Storage Files:** Integrates with existing erasure-coded shard infrastructure (Sections 19-22).
- **Printer Jobs:** Discovery finds nearby printer; job routed to printer owner's node; queued in local spooler.
- **Internet Access:** Local only (captive portal runs on access point node).

### 2.3 Session Coordination

Sessions tracked in SQLite on both user and provider nodes. Sync agent reconciles on reconnection.

## 3. Blockchain Integration

Resource usage commits to blockchain following the pattern from Section 12 (Accounting).
Commitment schedule: usage batches every 1000 events or 6 hours; payment settlements
every 100 payments or 1 hour; disputes immediately upon filing.

## 4. Payment Architecture

### 4.1 Pluggable Payment Processor Interface

A `PaymentProcessor` interface avoids vendor lock-in. Implementations:

- **Stripe** - Direct REST API calls
- **Federation Token** - On-chain smart contract transfers
- **Bitcoin Lightning** - Near-zero fees, instant settlement
- **Barter/Mutual Aid** - Credit ledger for cooperative federations

### 4.2 Offline Settlement

Follows Section 25 sync agent pattern. Pending payments queued with exponential backoff
and settled when connectivity resumes.

## 5. Security Model

### 5.1 Defense Layers

1. **Authentication** - DID-based credentials (existing)
2. **Authorization** - OPA policies with resource-specific rules
3. **Sandboxing** - Linux namespaces, seccomp, AppArmor for compute; G-code validation for printers
4. **Content Scanning** - ClamAV integration for storage uploads
5. **Monitoring & Alerting** - Prometheus metrics with security-focused alerts

## 6. Governance Integration

Resource sharing subject to federation governance (Section 32). Governable parameters
include resource limits, pricing floors, sandbox requirements, and dispute windows.

## 7. Migration Path

Phased rollout over 19+ weeks: Foundation, Internet Portal, Storage Sharing, Compute
Sharing, Printer Spooling, Integration & Security, Federation Testing, Production Deployment.

## 9. LBTAS Bidirectional Rating System

The Leveson-Based Trade Assessment Scale (LBTAS) is a 6-point bidirectional reputation
system where both transaction parties rate each other as a condition of transaction completion.

### 9.1 Core Principles

- Ratings are escrowed: neither party receives full value until both have rated
- 6-point scale: 0 (Catastrophic) to 5 (Exemplary)
- Bidirectional: both user and provider must rate
- Blockchain-anchored: all ratings signed and committed on-chain

### 9.2 Transaction State Machine

States: initiated -> executing -> awaiting_provider_rating -> results_escrowed ->
awaiting_user_rating -> completed (or disputed/timed_out/cancelled)

### 9.3 Rating Categories

Each resource type has specific rating dimensions for both user and provider perspectives.

### 9.4 Score Aggregation

Weighted combination of category scores (0-100 overall). Recent ratings weighted more
heavily. Dispute ratio applies penalties. Scores periodically anchored to blockchain.

### 9.5 Reputation-Gated Access

Minimum score requirements by resource type enforced via OPA policies. New users get
provisional access with limits. High-reputation users get premium access.

### 9.6 Deadlock Resolution

Auto-resolution after deadline: if provider doesn't rate, payment refunded; if user
doesn't rate, auto-rated as "Acceptable" (3/5) and payment released.

## 10. Integration Summary

See source code in `internal/lbtas/`, `internal/payment/`, `internal/compute/`,
`internal/storage/`, `internal/printer/`, `internal/portal/`, `internal/httpapi/`.
