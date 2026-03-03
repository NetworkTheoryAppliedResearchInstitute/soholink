# SoHoLINK: Current Capabilities — An Overview

**Project:** SoHoLINK — Federated SOHO Compute Marketplace
**Date:** 2026-03-01
**Status:** Internal Capability Assessment

---

## Introduction

SoHoLINK is a federated compute marketplace designed for Small Office/Home Office (SOHO) hardware. Its purpose is to aggregate underutilized computing resources — desktops, NAS devices, mini-PCs, and eventually mobile devices — into a cohesive, incentivized network where resource providers earn real payments for sharing their idle capacity. Built entirely in Go 1.24.6 with a vendored dependency tree, SQLite-backed state, a Fyne-based graphical interface, and Open Policy Agent (OPA) for governance, SoHoLINK is a vertically integrated platform: from hardware detection to payment settlement, every layer is purpose-built and under direct control.

This document surveys the current state of the system's capabilities across each major subsystem.

---

## Hardware Discovery and Profiling

SoHoLINK begins with the hardware it runs on. On startup, the platform executes a comprehensive hardware discovery pass across CPU, GPU, memory, storage, and network interfaces using `gopsutil` and platform-specific probes. On Windows, this supplements `gopsutil` with WMI queries for GPU information that the cross-platform library cannot surface. On Linux, `/proc` and `/sys` filesystem reads provide fine-grained hardware data including thermal zones and CPU frequency governors. On macOS, `system_profiler` and IOKit queries surface Apple Silicon details.

The output of discovery is a structured hardware profile that feeds directly into the cost calculator: the system computes a per-hour provider rate based on CPU core count and clock speed, available RAM, storage throughput, and GPU presence. This rate becomes the floor for the provider's listing in the marketplace. The wizard UI walks new users through this profile and allows them to review and adjust their pricing before activating.

**Current status:** ✅ Fully operational across Windows, Linux, and macOS. Platform-specific detection files handle each OS family.

---

## Kubernetes-Inspired Federated Scheduling

At the heart of SoHoLINK's compute orchestration is the `FedScheduler`, a custom workload scheduler modeled after Kubernetes scheduling semantics but purpose-built for the federated SOHO context. Where Kubernetes assumes a trusted cluster of nodes under centralized administrative control, FedScheduler operates across nodes owned by independent providers who may join or leave the network at any time.

The scheduler maintains a registry of active nodes with their capability profiles and health status. When a workload submission arrives, the scheduler filters nodes by the workload's placement constraints (required CPU cores, minimum RAM, required architecture, geographic preference) and ranks qualifying nodes by a composite fitness score incorporating current load, latency to the requester, and historical reliability. The winning node receives the workload placement, and the scheduler begins monitoring its execution.

The `FedScheduler` also incorporates an auto-scaler that monitors workload metrics and adjusts replica counts in response to demand — adding placements when throughput targets are being missed, and removing them when resources are underutilized. This mirrors the Horizontal Pod Autoscaler pattern from Kubernetes but operates in the federated trust model where "scaling up" means recruiting additional independent provider nodes rather than spinning up VMs in a controlled cluster.

`ListActiveWorkloads()` provides a safe, deep-copied snapshot of current placements for external callers (the dashboard, the HTTP API, monitoring systems) without exposing internal scheduler locks.

**Current status:** ✅ Operational. K8s HTTP adapter (`k8s_edge.go`) provides a bridge to real Kubernetes clusters at the edge.

---

## IPFS-Backed Distributed Storage

SoHoLINK's storage subsystem integrates with IPFS (InterPlanetary File System) via a purpose-built HTTP API client targeting a locally running Kubo daemon. Rather than reinventing content-addressed storage, SoHoLINK treats the local IPFS node as a managed service and wraps it with the platform's own access control and metering.

The `IPFSStoragePool` exposes `Upload`, `Download`, `Delete`, and `LookupByCID` operations with proper concurrent access control via a `sync.RWMutex`. When a compute job requires input data or produces output artifacts, those artifacts flow through the IPFS storage pool: inputs are pinned to the local node and their CIDs are passed to the executing worker, outputs are uploaded and their CIDs returned to the requester. This gives SoHoLINK content-addressable, verifiable, decentralized data movement without building a custom DHT.

A local content-addressed pool provides a fast-path cache for frequently accessed content, avoiding redundant IPFS network fetches for hot data.

**Current status:** ✅ Operational. IPFS HTTP client fully implemented with proper locking.

---

## Payment Infrastructure

SoHoLINK implements a dual-rail payment system supporting both traditional card payments via Stripe and Lightning Network micropayments for near-instant crypto settlement. The architecture is designed around the reality that providers want to earn and requesters want to pay with minimal friction across both Web2 and Web3 payment preferences.

### Stripe Integration

The Stripe processor handles card-based charges using Stripe's payment intents API. Charge creation, confirmation, refund, and webhook event processing are all implemented. The processor is configured with testable base URLs (using `StripeProcessor.baseURL` for dependency injection in tests), enabling unit tests to run against `httptest.Server` instances rather than real Stripe API calls. Webhook signature verification uses HMAC-SHA256 to prevent replay attacks.

### Lightning Network Integration

The Lightning processor handles Lightning Network invoices — create, monitor, and settle. Lightning payments are ideal for the sub-cent micropayment model that per-minute resource metering produces: a 10-minute compute job at $0.02/hour generates a $0.003 payment, which is economically irrational on Stripe but trivial on Lightning.

### Metering

The `meter.go` subsystem implements a per-hour billing loop. For each active rental, it calculates elapsed time since the last billing tick, computes the charge at the provider's per-hour rate, and invokes the appropriate payment processor. An explicit guard prevents negative elapsed time (which could occur across clock adjustments) from producing billing anomalies.

### Fee Structure

The platform applies a 1% central fee on net payment (after processor fees). The `internal/central` package computes this split, and `internal/store` persists central revenue records for accounting and audit purposes. The effective provider payout is approximately 96–97% of the requester's gross payment after Stripe's ~2.9%+$0.30 and the 1% platform fee.

**Current status:** ✅ Stripe and Lightning both operational. Metering loop active. Fee calculation and ledger recording functional.

---

## Auto-Accept Rental Engine

SoHoLINK's `internal/rental` package implements an auto-accept engine that evaluates incoming resource requests against the provider's configured policies and automatically accepts or rejects them without requiring manual intervention for every transaction. This is essential for the "set it and forget it" SOHO operator experience — the provider configures their rules once and the platform handles incoming business automatically.

The auto-accept engine evaluates requests against OPA policies (described below) and against the provider's provisioning limits (maximum CPU share, maximum RAM commitment, reserved local capacity). Requests that pass all checks are automatically provisioned; those that fail are rejected with a reason code returned to the requester.

**Current status:** ✅ Operational. Integrates with OPA policy evaluation and provisioning limit configuration.

---

## OPA Policy Governance

Resource sharing policies are expressed in Rego and evaluated by Open Policy Agent. The primary policy file (`configs/policies/resource_sharing.rego`) defines rules for what fraction of CPU, RAM, storage, and network bandwidth a provider is willing to share, under what conditions, and with what rate limits.

Policies can express nuanced rules: share up to 80% of CPU only when local CPU utilization is below 20%; share storage only with requesters who have completed at least 5 successful prior transactions; reject requests that would exceed a configured monthly bandwidth quota. OPA evaluation is fast (sub-millisecond for simple policies) and the Rego language is expressive enough to encode complex multi-variable governance logic without embedding business rules in Go code.

The `PolicyConfig` struct in `wizard/types.go` exposes the provider-facing configuration surface — the fields a SOHO operator sets in the wizard UI — and the policy evaluation layer translates these into OPA input documents.

**Current status:** ✅ OPA evaluation operational. Policy configuration surfaced in wizard and dashboard Settings dialogs.

---

## 3D Globe Visualization

The globe interface (`ui/globe-interface/ntarios-globe.html`) provides a real-time three-dimensional visualization of the SoHoLINK network. Built with Three.js and WebSocket connectivity, it renders a wireframe globe with node markers and animated data flow arcs between connected nodes.

The visualization operates in two modes:

- **Topology mode:** Nodes are positioned based on the DDS (Data Distribution Service) graph topology — their logical distance in the federation network determines their visual position.
- **Geographic mode:** When nodes report `lat`/`lon` coordinates (as part of their node info), a "GEO" button appears and enables repositioning of nodes on their real-world geographic coordinates. This mode reveals the physical distribution of the SOHO network and makes latency characteristics visually intuitive.

The WebSocket bridge delivers node snapshots in a defined JSON schema including `id`, `name`, `lat`, `lon`, `region`, `health`, `latency`, and `topics`. The globe auto-enables GEO mode when latitude and longitude are present in the data stream.

**Current status:** ✅ Fully operational. GEO/topology toggle working. Node positioning on real coordinates functional.

---

## GUI Dashboard

The unified Fyne-based GUI provides a single entry point for both initial setup and ongoing node management. The architecture routes to either the setup wizard or the operating dashboard based on configuration state.

### Setup Wizard (8 Steps)

New users are guided through: hardware detection and review, pricing configuration, network settings, payment rail setup (Stripe API keys or Lightning node connection), Kubernetes edge cluster configuration, IPFS daemon connection, provisioning limits, and policy configuration. Each step validates its inputs before advancing.

### Operating Dashboard (8 Tabs)

Once configured, the dashboard presents:

- **Overview:** Current earnings, active rentals, network status, and recent activity
- **Hardware:** Live hardware utilization metrics from gopsutil
- **Orchestration:** Active workloads, scheduler state, node registry
- **Storage:** IPFS pool status, content inventory, bandwidth metrics
- **Billing:** Payment history, pending settlements, revenue breakdown by fee layer
- **Users:** Provider and requester account management
- **Policies:** Live OPA policy configuration and evaluation testing
- **Logs:** Streaming application logs with level filtering

**Current status:** ✅ Complete rewrite (~1,650 lines in `internal/gui/dashboard/dashboard.go`). All 8 tabs and 7 settings dialogs implemented. Requires CGO + GCC/MinGW to compile (Fyne's OpenGL dependency).

---

## HTTP API

The `internal/httpapi` package exposes a REST API for headless operation, enabling programmatic interaction with SoHoLINK from scripts, monitoring systems, or remote management tools. Endpoints cover workload submission and status, node registry queries, storage operations, and billing ledger access.

The API is designed with testability first: all handlers accept injected dependencies (store, scheduler, payment processors) rather than accessing globals, enabling comprehensive unit testing with mock implementations.

**Current status:** ✅ Operational. Full test coverage via `server_test.go`.

---

## Distribution and Packaging

SoHoLINK's build pipeline produces zero-dependency distributable packages for all major platforms:

- **Windows:** A statically linked `.exe` with MinGW C++ runtime embedded (no DLL dependencies on the user's machine), plus an NSIS setup wizard installer
- **macOS:** A universal binary (`lipo`-merged Intel + Apple Silicon) packaged as a `.pkg` installer
- **Linux:** A `.tar.gz` archive, `.deb` and `.rpm` packages via nFPM, and an AppImage self-contained executable

GoReleaser (`goreleaser release --snapshot --clean`) drives the entire multi-platform build pipeline from a single command. GitHub Actions automates snapshot builds on every push and publishes full releases when a `v*` tag is pushed.

**Current status:** ✅ Full pipeline operational as of 2026-03-01.

---

## Dependency Management and Updates

All Go module dependencies are vendored for reproducible offline builds. Dependabot monitors both Go modules and GitHub Actions versions weekly, creating individual PRs for package updates grouped by ecosystem (Fyne, OTel, Prometheus, etc.). A monthly scheduled workflow performs a full `go get -u ./...` sweep, runs the test suite against the upgraded dependencies, and opens a consolidated PR if all tests pass.

**Current status:** ✅ Automated update pipeline operational as of 2026-03-02.

---

## Summary

SoHoLINK is a production-capable federated compute marketplace with all core subsystems operational: hardware discovery, workload scheduling, distributed storage, dual-rail payments, policy governance, and a full GUI. The platform's distributed-first design — content-addressed storage, federated scheduling, Lightning micropayments, OPA policy evaluation — reflects the operational reality of SOHO hardware: independently owned, intermittently available, and collectively powerful when properly coordinated. The immediate roadmap centers on mobile device participation (see `docs/research/MOBILE_PARTICIPATION.md`) and continued hardening of the security and observability layers.
