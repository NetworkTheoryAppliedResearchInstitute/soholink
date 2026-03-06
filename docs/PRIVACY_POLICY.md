# SoHoLINK Privacy Policy

*Effective Date: 2026-03-06 | Version: 1.0*

SoHoLINK ("we", "us", "our") is a federated compute marketplace operated by Network Theory Applied Research Institute. This Privacy Policy explains what data we collect, how we use it, and your rights regarding that data.

---

## 1. Data We Collect

### 1.1 Identity Data
- **Decentralized Identifier (DID):** Each node generates a `did:key:` identifier locally. We store this identifier to route payments, record ratings, and enforce access policies. You control your private key; we never receive it.
- **Ed25519 Public Key:** Stored alongside your DID to verify signed announcements and API requests.

### 1.2 Hardware and Resource Data
- CPU cores, RAM (GB), disk capacity (GB), and GPU model — collected during onboarding by `internal/wizard/detection.go` using open-source system information libraries.
- Node region hint (e.g. `us-east-1`) used for scheduling locality.
- Real-time utilisation metrics (CPU %, RAM %, disk %) collected by the metering loop during active workloads.

### 1.3 Usage and Metering Logs
- Per-hour resource consumption records: workload ID, start/end timestamps, resource type, satoshi amounts charged.
- SLA violation events: contract ID, violation type, measured vs. target values, credit amounts issued.
- Wallet transaction history: topup IDs, processor references, amounts (in satoshis), confirmation timestamps.

### 1.4 Payment References
- **Lightning Network:** Payment request strings (invoices) and settlement hashes. We do not receive Lightning channel private keys or seed phrases.
- **Stripe:** Payment Intent IDs, Checkout Session IDs, and webhook event IDs provided by Stripe. We store references, not raw card numbers. Card data is handled exclusively by Stripe under PCI-DSS.
- We do **not** store full credit card numbers, CVV codes, bank account numbers, or routing numbers.

### 1.5 Content Hashes
- SHA-256 hashes of content uploaded to IPFS via the SoHoLINK storage pool, used for content safety screening against the CSAM blocklist and DMCA takedown compliance. Raw content is stored in IPFS and is not accessible to SoHoLINK operators unless explicitly requested for compliance purposes.

### 1.6 Auto-Accept Rule Audit Log
- Every rental engine decision (auto-accept, auto-reject, pending) is recorded with: request ID, user DID, rule ID, action, reason, and timestamp. This log is for compliance review only and is not shared with third parties.

### 1.7 DID Blocklist
- DIDs blocked for policy violations are recorded with: the DID, reason, timestamp, blocking operator, and optional expiry.

---

## 2. How We Use Your Data

| Purpose | Data Used | Legal Basis |
|---------|-----------|-------------|
| Routing payments and calculating fees | DID, wallet balance, payment references | Contract performance |
| Scheduling and matching workloads | Hardware specs, region, reputation score | Contract performance |
| SLA enforcement and credit issuance | Metering logs, SLA violation events | Contract performance |
| Fraud prevention and abuse detection | DID, usage patterns, blocked DID list | Legitimate interest |
| Content safety compliance (CSAM, DMCA) | Content hashes | Legal obligation |
| Audit trail for regulatory review | Rental audit log, payment records | Legal obligation |
| Product improvement and debugging | Anonymised performance metrics | Legitimate interest |

We **do not** sell, rent, or trade your personal data to third parties for marketing purposes.

---

## 3. Third-Party Services

| Service | Purpose | Privacy Policy |
|---------|---------|----------------|
| **Stripe** | Credit/debit card payment processing | [stripe.com/privacy](https://stripe.com/privacy) |
| **Lightning Network** | Bitcoin Lightning payment routing | Decentralised protocol — no single privacy policy |
| **IPFS / Kubo** | Distributed content storage | [ipfs.tech](https://ipfs.tech) |

Data shared with third parties is limited to the minimum necessary for the stated purpose.

---

## 4. Data Retention

| Data Type | Retention Period |
|-----------|-----------------|
| Metering and usage logs | 12 months from event date |
| Payment transaction records | 7 years (financial/tax compliance) |
| SLA violation records | 24 months |
| Rental audit log | 24 months |
| Blocked content hashes (CSAM) | Indefinite (legal obligation) |
| Blocked DID records | Until expiry or manual removal |
| Wallet topup records | 7 years (financial/tax compliance) |

---

## 5. Data Security

- All API traffic is encrypted in transit using TLS 1.2 or higher when TLS certificates are configured.
- RADIUS shared secrets and payment processor credentials are stored in environment variables, not in configuration files committed to source control.
- The SQLite database is stored locally on the operator's hardware. Operators are responsible for disk encryption and physical security.
- Stripe webhook payloads are verified using HMAC-SHA256 signatures before processing.

---

## 6. Your Rights

Depending on your jurisdiction, you may have the right to:

- **Access:** Request a copy of the data we hold about your DID (`GET /api/admin/export?did=<your-did>` when logged in as an admin).
- **Correction:** Request correction of inaccurate data by contacting us.
- **Deletion / Account Closure:** Request deletion of your account and associated data (`DELETE /api/account`). Note that payment records required for tax compliance are retained for 7 years.
- **Portability:** Export your usage data in JSON format via the admin API.
- **Objection:** Object to processing for legitimate interest purposes.

To exercise these rights, email **privacy@soholink.network** with your DID and a description of your request.

---

## 7. Children's Privacy

SoHoLINK is intended for adults operating SOHO hardware. We do not knowingly collect data from persons under 18 years of age. If you believe a minor has created a node, contact us immediately at **privacy@soholink.network**.

---

## 8. Changes to This Policy

We will post updated versions of this policy at the repository path `docs/PRIVACY_POLICY.md` and announce material changes in the release notes. Continued use of the platform after the effective date constitutes acceptance of the updated policy.

---

## 9. Contact

**Privacy enquiries:** privacy@soholink.network  
**General enquiries:** hello@soholink.network  
**Postal address:** Network Theory Applied Research Institute *(address on file with incorporation documents)*

---

*This document is provided for informational purposes and does not constitute legal advice. Consult qualified legal counsel before deploying SoHoLINK in a regulated environment.*
