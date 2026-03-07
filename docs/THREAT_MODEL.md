# SoHoLINK STRIDE Threat Model
## Federated SOHO Compute Marketplace

**Organization:** Network Theory Applied Research Institute (NTARI)
**Location:** Louisville, Kentucky | ntari.org | AGPL-3.0
**Document Version:** 1.0 | March 2026

> **CLASSIFICATION: INTERNAL — RESTRICTED**
> This document contains security-sensitive information. Distribution limited to NTARI core team and authorized security reviewers.

---

## Table of Contents

1. [Purpose and Scope](#1-purpose-and-scope)
2. [System Overview](#2-system-overview)
3. [Trust Boundaries](#3-trust-boundaries)
4. [Threat Catalog](#4-threat-catalog)
   - [4.1 TB-1: Internet → Node API](#41-tb-1-internet--node-api)
   - [4.2 TB-2: LAN Mesh → Node](#42-tb-2-lan-mesh--node)
   - [4.3 TB-3: Wasm Executor → Host OS](#43-tb-3-wasm-executor--host-os)
   - [4.4 TB-4: Node → Payment Processors](#44-tb-4-node--payment-processors)
   - [4.5 TB-5: Node → IPFS / Kubo](#45-tb-5-node--ipfs--kubo)
   - [4.6 TB-6: Mobile App → Node API](#46-tb-6-mobile-app--node-api)
   - [4.7 TB-7: Provider → OPA Policy Engine](#47-tb-7-provider--opa-policy-engine)
5. [Risk Summary and Priority Matrix](#5-risk-summary-and-priority-matrix)
6. [Immediate Action Items](#6-immediate-action-items)
7. [External Audit Pathway](#7-external-audit-pathway)
8. [Document Maintenance](#8-document-maintenance)

---

## 1. Purpose and Scope

This document applies the STRIDE threat modeling framework to SoHoLINK, NTARI's federated compute marketplace. Its purpose is to establish a systematic, living reference for the security posture of the system — replacing reactive, session-based security patches with proactive, design-level threat awareness.

**STRIDE** stands for:

- **S**poofing — impersonating another entity
- **T**ampering — unauthorized modification of data or code
- **R**epudiation — denying actions without being able to be contradicted
- **I**nformation Disclosure — exposing information to unauthorized parties
- **D**enial of Service — degrading or denying service to legitimate users
- **E**levation of Privilege — gaining capabilities beyond what is authorized

**Scope:**

- SoHoLINK node daemon (Go binary: `soholink` / `soholink-gui`)
- Federation layer: P2P LAN mesh, coordinator API, WebSocket hub
- Payment subsystems: Stripe integration, Lightning Network / LND, HTLC settlement
- Workload execution: Wasm task executor (wazero), OPA policy enforcement
- Storage layer: SQLite local store, IPFS / Kubo integration
- Mobile participation: Flutter app, Android TV node, iOS monitor
- Installer and first-run wizard (Windows, macOS, Linux)

**Out of scope:**

- Third-party infrastructure security (Stripe, Lightning Network protocol, IPFS protocol)
- Physical security of provider hardware
- Social engineering attacks against NTARI staff

---

## 2. System Overview

SoHoLINK enables home and small-office hardware owners (providers) to rent idle compute resources to requesters. The system is structured around five interacting layers:

- **Node Layer** — the Go daemon running on provider hardware, exposing a REST API and Fyne GUI dashboard
- **Federation Layer** — Ed25519-signed multicast UDP peer discovery on LAN (`239.255.42.99:7946`); WebSocket hub for inter-node coordination
- **Scheduler Layer** — FedScheduler (Kubernetes-inspired); LinUCBBandit ML for node selection; OPA Rego policy enforcement on resource sharing
- **Payment Layer** — Stripe (card), Lightning Network (micropayments), HTLC hold invoices, Merkle-chained billing ledger
- **Workload Layer** — Wasm executor (wazero, v0.3 stub), IPFS content-addressed storage for inputs/outputs

The threat surface is large relative to a typical SOHO application because SoHoLINK: (a) accepts arbitrary code execution from internet strangers, (b) handles real financial transactions, (c) runs on home networks co-located with personal and IoT devices, and (d) federates with untrusted external nodes.

---

## 3. Trust Boundaries

The following trust boundaries define where data crosses security domains and where the majority of threats concentrate:

| Trust Boundary | Components | Trust Level | Primary Risks |
|---|---|---|---|
| **TB-1: Internet → Node API** | HTTP/REST endpoints (port 8080); rate-limited; Ed25519 auth for federation | Zero trust — all input treated as adversarial | Injection, DoS, authentication bypass, workload abuse |
| **TB-2: LAN Mesh → Node** | Multicast UDP discovery (`239.255.42.99:7946`); Ed25519 signed announcements | Low trust — LAN peers unvetted unless allowlisted | Lateral movement, rogue node injection, multicast spoofing |
| **TB-3: Wasm Executor → Host OS** | wazero runtime; WASI imports; filesystem, network, env access | Untrusted — requester-supplied code | Container escape, resource exhaustion, host filesystem access |
| **TB-4: Node → Payment Processors** | Stripe API (HTTPS); LND gRPC (TLS + macaroons); HTLC settlement | High trust outbound, verified via certs/macaroons | Credential theft, missing cert pinning, payment manipulation |
| **TB-5: Node → IPFS / Kubo** | Local Kubo daemon HTTP API; content-addressed CID pinning | Medium — local daemon, but CID resolution touches public DHT | Malicious content via CID, DHT poisoning, data exfiltration |
| **TB-6: Mobile App → Node API** | Flutter app (Dart); hardcoded LAN IP; REST + WebSocket | Low — app binary distributable, IP discoverable on LAN | Credential exposure in binary, MITM on LAN, unauthorized wallet access |
| **TB-7: Provider → OPA Policy Engine** | Rego policies in `configs/policies/`; auto-accept engine | High — provider-controlled, but misconfiguration risk | Policy bypass, overly permissive defaults, policy injection |

---

## 4. Threat Catalog

Threats are identified per trust boundary and categorized by STRIDE. Severity ratings: **HIGH** (immediate risk to funds, host OS, or user data), **MED** (significant risk requiring near-term remediation), **LOW** (manageable with operational controls).

### 4.1 TB-1: Internet → Node API

| Threat ID | STRIDE Category | Severity | Description | Attack Vector | Mitigation |
|---|---|---|---|---|---|
| T-001 <!-- status: open --> | Spoofing | HIGH | Attacker impersonates a legitimate provider node to the federation coordinator by forging Ed25519 announcements using a stolen or weak key. | Compromised node key; key reuse across deployments | Enforce key rotation policy; store private keys in OS keychain (not flat config file); add key fingerprint registry at coordinator |
| T-002 <!-- status: open --> | Tampering | HIGH | Malicious requester submits a crafted workload that escapes the Wasm sandbox and modifies host filesystem or process memory. Critical while Wasm executor is a stub. | Incomplete wazero capability restrictions; missing WASI deny-list | Complete wazero implementation with explicit deny-all WASI policy; add seccomp-bpf or AppArmor profile on Linux; hard timeout via context cancellation |
| T-003 <!-- status: open --> | Denial of Service | HIGH | Attacker floods API endpoints with requests to exhaust goroutine pool, memory, or file descriptors, taking the node offline. | Rate limiter gaps; unbounded goroutine spawning per workload request | Add global connection limit; enforce workload queue depth cap; implement circuit breaker pattern in FedScheduler |
| T-004 <!-- status: open --> | Elevation of Privilege | HIGH | Requester submits workload that, via Wasm escape or IPFS fetch, achieves code execution as the node daemon's OS user, potentially root if run without privilege separation. | Daemon running as root (common in naive installs); Wasm stub not enforcing isolation | Installer should create dedicated low-privilege `soholink` OS user; document and enforce non-root deployment; complete Wasm isolation |
| T-005 <!-- status: open --> | Information Disclosure | MED | API error responses leak internal file paths, Go stack traces, or configuration details that aid further attacks. | Default Go error propagation to HTTP responses | Implement error scrubbing middleware; log full errors internally, return only opaque error codes externally |
| T-006 <!-- status: open --> | Repudiation | MED | Requester denies submitting a malicious workload; provider cannot prove attribution without tamper-evident request logs. | Absence of signed workload receipts | Extend Merkle chain to cover workload submissions with requester signature; issue signed receipts at `POST /api/marketplace/purchase` |

### 4.2 TB-2: LAN Mesh → Node

| Threat ID | STRIDE Category | Severity | Description | Attack Vector | Mitigation |
|---|---|---|---|---|---|
| T-007 <!-- status: open --> | Spoofing | HIGH | Attacker on provider's LAN (e.g., via guest WiFi or compromised IoT device) sends forged Ed25519-signed multicast announcements to inject a rogue node into the federation scheduler. | Shared LAN with untrusted devices; no CIDR allowlist enforced by default | Implement allowlist mode in P2P discovery config (`allowed_cidrs`); default installer to allowlist-only; add announcement replay prevention via nonce+timestamp |
| T-008 <!-- status: open --> | Tampering | MED | Rogue LAN peer sends manipulated heartbeat messages to corrupt the scheduler's node health state, causing legitimate workloads to be routed to attacker-controlled nodes. | Scheduler trusts heartbeat content after signature verification without cross-validating metrics | Cross-validate resource metrics from heartbeats against historical baselines; flag statistical outliers for manual review |
| T-009 <!-- status: open --> | Denial of Service | MED | Multicast flood from a LAN device saturates the UDP discovery handler, consuming CPU and preventing legitimate peer discovery. | No rate limit on UDP multicast processing; multicast group open to all LAN devices | Add per-source-IP rate limit on multicast handler; drop packets exceeding threshold without processing |
| T-010 <!-- status: open --> | Information Disclosure | MED | Any device on the provider's LAN can passively observe multicast announcements and map the federation topology, node capabilities, and pricing. | Unencrypted UDP multicast; no confidentiality on peer announcements | Encrypt announcement payload (keep signature wrapper for integrity); document topology exposure risk prominently in installer |

### 4.3 TB-3: Wasm Executor → Host OS

> **⚠ CRITICAL NOTE:** This is the highest-risk boundary. Until the Wasm executor is fully implemented, all threats in this section are CRITICAL in practice, as there is no isolation layer between requester code and the host.

| Threat ID | STRIDE Category | Severity | Description | Attack Vector | Mitigation |
|---|---|---|---|---|---|
| T-011 <!-- status: open --> | Tampering | HIGH | Malicious Wasm module uses unrestricted filesystem WASI imports to read, write, or delete provider files (SSH keys, wallet credentials, personal documents). | wazero stub with no `FSConfig` restrictions; WASI `fs.open` not denied | Pass `wazero.NewFSConfig()` (empty) to deny all filesystem access; whitelist only in-memory I/O buffers for task inputs/outputs |
| T-012 <!-- status: open --> | Elevation of Privilege | HIGH | Wasm module invokes WASI socket imports to establish outbound network connections, exfiltrating provider data or downloading second-stage payloads. | wazero default WASI includes network capability if not explicitly denied | Deny all WASI network imports; workload I/O must go through IPFS CID mechanism only; validate output CIDs before settlement |
| T-013 <!-- status: open --> | Denial of Service | HIGH | Malicious workload enters infinite loop or allocates unbounded memory, exhausting provider resources and preventing other workloads or system functions. | No execution timeout; no memory cap enforced in wazero stub | Enforce hard timeout via `context.WithTimeout` (120s max per mobile spec); set memory limit in wazero `RuntimeConfig`; kill and refund if exceeded |
| T-014 <!-- status: open --> | Information Disclosure | MED | Wasm module reads environment variables via WASI env imports, exposing LND macaroons, Stripe API keys, or other credentials stored in the daemon environment. | WASI env capability not explicitly denied in wazero configuration | Deny WASI env imports; daemon should not store secrets in environment variables — use encrypted config file or OS keychain |

### 4.4 TB-4: Node → Payment Processors

| Threat ID | STRIDE Category | Severity | Description | Attack Vector | Mitigation |
|---|---|---|---|---|---|
| T-015 <!-- status: open --> | Tampering | HIGH | MITM attacker intercepts LND gRPC connection (if TLS cert pinning is absent) to manipulate HTLC settlement, redirecting payments or preventing payout. | Startup warning for missing cert pinning allows nodes to run without it | Make `lnd_tls_cert_path` a fatal startup requirement; validate cert at boot; reject connection if cert fingerprint does not match pinned value |
| T-016 <!-- status: open --> | Information Disclosure | HIGH | LND macaroon file readable by other OS users or accessible to malicious workload, allowing full Lightning wallet control. | Macaroon stored as plain file; improper file permissions; Wasm escape (T-011/T-014) | Set macaroon file permissions to 600 (owner-only read); store path in encrypted config; use baked macaroons with minimum permissions (`invoices:read`, `invoices:write` only) |
| T-017 <!-- status: open --> | Denial of Service | MED | Requester repeatedly initiates payment flows and abandons them, exhausting HTLC slots on the provider's LND node and preventing legitimate Lightning payments. | No limit on pending HTLC count per requester; no reputation gate before payment initiation | Gate HTLC creation behind LBTAS reputation score minimum; limit open HTLCs per requester IP; implement backoff for repeated abandoned payments |
| T-018 <!-- status: open --> | Repudiation | MED | Provider disputes a payout, claiming it was never received. Without cryptographic proof of settlement, ledger disputes are unresolvable. | Merkle chain covers billing events but may not include LND payment preimage | Include Lightning payment preimage in Merkle chain entries at settlement; export preimage-linked receipts via `GET /api/revenue/payouts` |

### 4.5 TB-5: Node → IPFS / Kubo

| Threat ID | STRIDE Category | Severity | Description | Attack Vector | Mitigation |
|---|---|---|---|---|---|
| T-019 <!-- status: open --> | Tampering | MED | Requester supplies a malicious CID that, when fetched from the IPFS DHT, delivers different content than expected — poisoning the workload input. | IPFS content-addressing guarantees integrity of fetched content but not the content's intent | Validate CID content against declared MIME type and size bounds before passing to Wasm executor; reject unknown or oversized inputs |
| T-020 <!-- status: open --> | Information Disclosure | MED | Workload output CIDs pinned to local Kubo are publicly fetchable from the IPFS DHT by anyone who knows the CID, potentially leaking sensitive computation results. | IPFS is a public network by default; pinned CIDs are retrievable globally | Document CID public exposure clearly; offer private IPFS swarm option for sensitive workloads; consider time-limited pins with automatic garbage collection |
| T-021 <!-- status: open --> | Denial of Service | LOW | Requester submits workloads with massive input CIDs, saturating local IPFS storage and disk I/O. | No size cap on pinned input CIDs | Enforce maximum input CID size in OPA policy (`resource_sharing.rego`); reject pin requests exceeding `storage_limit_gb` configured by provider |

### 4.6 TB-6: Mobile App → Node API

| Threat ID | STRIDE Category | Severity | Description | Attack Vector | Mitigation |
|---|---|---|---|---|---|
| T-022 <!-- status: open --> | Information Disclosure | HIGH | Flutter app ships with node IP hardcoded in `kNodeUrl` constant. If the APK/IPA is decompiled, the provider's home IP address is exposed. | Hardcoded string in `mobile/flutter-app/lib/api/soholink_client.dart` | Replace hardcoded IP with local network discovery (mDNS/Bonjour) or QR-code pairing flow; never ship provider IP in distributed binary |
| T-023 <!-- status: open --> | Spoofing | MED | On a shared or public WiFi, attacker sets up a rogue AP with the same SSID, intercepting Flutter app traffic to the node API and stealing wallet balance or session tokens. | No certificate pinning in Flutter HTTP client; LAN assumed trusted | Implement TLS with self-signed cert on node API; Flutter app must pin the provider cert via `http_certificate_pinning` package; prompt user to generate cert on first run |
| T-024 <!-- status: open --> | Elevation of Privilege | MED | Unauthenticated access to wallet top-up or order-cancellation endpoints from any device on the LAN, as the Flutter app relies on LAN adjacency rather than authentication tokens for authorization. | API authentication model not described; endpoints may rely on network-level trust | Issue HMAC or JWT session tokens at app pairing; require token on all wallet and order endpoints; document authentication model explicitly in `docs/ARCHITECTURE.md` |

### 4.7 TB-7: Provider → OPA Policy Engine

| Threat ID | STRIDE Category | Severity | Description | Attack Vector | Mitigation |
|---|---|---|---|---|---|
| T-025 <!-- status: open --> | Tampering | MED | Requester submits a workload with metadata crafted to exploit evaluation logic in `resource_sharing.rego`, causing the auto-accept engine to approve requests that should be denied. | Rego policy logic bugs; insufficient input validation before OPA evaluation | Fuzz OPA policy inputs with adversarial workload metadata; add unit tests for boundary conditions in Rego (`cpu_share=1.0`, `reputation=0`, `bandwidth=-1`) |
| T-026 <!-- status: open --> | Information Disclosure | LOW | OPA policy bundle loaded over HTTP without integrity verification could be replaced by a MITM to change provider resource sharing rules. | OPA bundle fetch without signature verification | Sign OPA bundles; verify signature at load time; load from local filesystem only in default configuration |
| T-027 <!-- status: open --> | Denial of Service | LOW | Complex or recursive Rego rules cause OPA evaluation timeout, blocking the auto-accept engine and preventing any workload acceptance. | Unbounded Rego evaluation; no OPA timeout configured | Set OPA decision timeout (100ms max); use partial evaluation where possible; add circuit breaker: if OPA unavailable, reject all workloads (fail closed, not open) |

---

## 5. Risk Summary and Priority Matrix

The following matrix summarizes all threats by severity and recommended action timeline:

| ID | Severity | Threat Summary | Recommended Timeline |
|---|---|---|---|
| T-002 | HIGH | Wasm sandbox escape → host OS | Blocking — complete before provider recruitment |
| T-004 | HIGH | Node daemon running as root | Sprint 1 — installer fix, low effort |
| T-011 | HIGH | Wasm filesystem access via WASI | Blocking — part of Wasm completion work |
| T-012 | HIGH | Wasm network access / data exfiltration | Blocking — part of Wasm completion work |
| T-013 | HIGH | Wasm infinite loop / resource exhaustion | Blocking — context timeout, low effort |
| T-015 | HIGH | LND MITM via missing cert pinning | Sprint 1 — one-day config enforcement change |
| T-016 | HIGH | LND macaroon disclosure | Sprint 1 — file permissions + config encryption |
| T-022 | HIGH | Provider home IP exposed in mobile binary | Sprint 2 — replace hardcoded IP with discovery |
| T-001 | HIGH | Federation node impersonation | Sprint 2 — key rotation policy + keychain storage |
| T-003 | HIGH | API goroutine exhaustion DoS | Sprint 2 — connection limits + circuit breaker |
| T-007 | HIGH | Rogue LAN node injection | Sprint 2 — CIDR allowlist default on |
| T-014 | MED | Wasm env var access (credentials) | Sprint 2 — deny WASI env imports |
| T-006 | MED | Requester repudiation of malicious workload | Sprint 3 — signed workload receipts |
| T-008 | MED | Heartbeat manipulation → scheduler poisoning | Sprint 3 — metric cross-validation |
| T-017 | MED | HTLC slot exhaustion via abandoned payments | Sprint 3 — reputation gate on HTLC creation |
| T-023 | MED | Flutter app MITM on rogue AP | Sprint 3 — TLS + cert pinning in app |
| T-024 | MED | Unauthenticated wallet API access from LAN | Sprint 3 — JWT/HMAC session tokens |
| T-025 | MED | OPA Rego policy logic exploitation | Sprint 3 — fuzz testing + Rego unit tests |
| T-005 | MED | API error response information leakage | Sprint 3 — error scrubbing middleware |
| T-009 | MED | Multicast UDP flood DoS | Sprint 3 — per-source rate limiting |
| T-010 | MED | Topology disclosure via plaintext multicast | Sprint 4 — announcement payload encryption |
| T-018 | MED | Payment settlement repudiation | Sprint 4 — preimage in Merkle chain |
| T-019 | MED | IPFS DHT content poisoning | Sprint 4 — CID validation before execution |
| T-020 | MED | IPFS output CID publicly accessible | Sprint 4 — private swarm option + documentation |
| T-021 | LOW | IPFS storage exhaustion | Sprint 4 — OPA policy size cap |
| T-026 | LOW | OPA bundle integrity (MITM) | Sprint 5 — bundle signing |
| T-027 | LOW | OPA evaluation timeout / DoS | Sprint 5 — timeout config + fail-closed |

---

## 6. Immediate Action Items

The following actions should be completed before any public provider recruitment or mainnet payment handling:

### 6.1 Blocking (Pre-Launch)

- Complete wazero Wasm executor with explicit deny-all WASI policy (T-002, T-011, T-012, T-013, T-014)
- Make `lnd_tls_cert_path` a fatal startup requirement — no warnings, no bypass (T-015)
- Installer: create dedicated `soholink` OS user with minimum privileges; document and enforce non-root operation (T-004)

### 6.2 Sprint 1 (Within 2 Weeks)

- Set LND macaroon file to permissions 600; migrate secrets from environment variables to encrypted config (T-016)
- Add `context.WithTimeout` to all workload execution paths (T-013)
- Implement error scrubbing middleware on all API error responses (T-005)
- Add global API connection limit and workload queue depth cap (T-003)

### 6.3 Sprint 2 (Within 4 Weeks)

- Replace hardcoded `kNodeUrl` in Flutter app with mDNS discovery or QR pairing (T-022)
- Enable CIDR allowlist mode by default in P2P discovery; surface in installer wizard Step 3 (T-007)
- Implement Ed25519 key rotation policy; store private keys in OS keychain (T-001)
- Add JWT/HMAC session token requirement on wallet and order endpoints (T-024)

### 6.4 Publish Responsible Disclosure Policy

See `SECURITY.md` at the repository root. Email: `security@ntari.org`. Response commitment: 72 hours acknowledgment, 14 days assessment.

---

## 7. External Audit Pathway

Given NTARI's nonprofit structure and AGPL-3.0 licensing, two no-cost or reduced-cost audit pathways are available:

### 7.1 Open Source Security Foundation (OpenSSF)

OpenSSF funds security audits for open-source projects with meaningful community impact. SoHoLINK's cooperative infrastructure mission and AGPL license align with their program criteria.
Application: <https://openssf.org/>

### 7.2 Radically Open Security

A worker-owned security cooperative that provides penetration testing and audit services to nonprofits and open-source projects, frequently at reduced rates. Mission alignment with NTARI's cooperative infrastructure model is strong.
Contact: <https://radicallyopensecurity.com/>

Both audits should be pursued after the Blocking and Sprint 1 items above are complete — auditing a system with known critical gaps is low-value. Target external audit after the Wasm executor is production-ready.

---

## 8. Document Maintenance

This document is a living reference. It must be updated when:

- A new system component or integration is added (e.g., new payment processor, new mobile platform)
- A threat is remediated — update the `<!-- status: open -->` marker to `<!-- status: resolved:COMMITHASH -->` with the commit hash and date
- A security finding is reported via responsible disclosure
- A dependency with known CVEs is identified

**Owner:** NTARI Core Team.
**Review cadence:** Minimum quarterly, or within 30 days of any major architectural change.

---

*Network Theory Applied Research Institute | ntari.org | AGPL-3.0*
*SoHoLINK Threat Model v1.0 | March 2026*
