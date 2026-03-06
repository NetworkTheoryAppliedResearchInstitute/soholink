# SoHoLINK — Terms of Service & Acceptable Use Policy

**Item 8 of 8 — Pre-Launch Legal Compliance**

> **DRAFT — Review by legal counsel required before public launch.**
> Governing law, dispute resolution, and jurisdiction clauses must be completed
> by a licensed attorney in the operator's jurisdiction before this document
> is presented to users.

**Effective Date:** `[Date — complete before launch]`
**Operator:** Network Theory Applied Research Institute

---

## 1. Agreement to Terms

By registering a device, creating a DID (`did:key:...`), or using any SoHoLINK API endpoint,
you ("User", "Provider", or "Requester") agree to be bound by these Terms of Service ("Terms").

If you do not agree to these Terms, you may not use the SoHoLINK platform.

---

## 2. What SoHoLINK Is

SoHoLINK is a federated peer-to-peer compute marketplace that allows:
- **Providers** (host nodes) to offer spare compute, storage, and network capacity
- **Requesters** (buyers) to rent that capacity for workload execution and data storage

SoHoLINK operates as a marketplace intermediary. The platform does not itself store, execute,
or transmit user content — content is stored and processed by independent Provider nodes.

---

## 3. Prohibited Content and Uses

The following are **absolutely prohibited** on the SoHoLINK platform, by all parties:

### 3.1 Child Sexual Abuse Material (CSAM)

Uploading, storing, transmitting, requesting, or facilitating access to any content that depicts
sexual abuse of a minor is a **federal crime** (18 U.S.C. § 2252) and is strictly prohibited.

- Any content matching NCMEC hash databases is automatically blocked and reported
- Accounts associated with CSAM are permanently suspended without appeal
- SoHoLINK is legally required to report CSAM to NCMEC within 24 hours (18 U.S.C. § 2258A)

### 3.2 Violence-Enabling Code

Uploading or executing code or workloads that are designed or primarily used to:
- Cause physical harm to humans or animals
- Control robotic or mechanical systems for the purpose of executing violence
- Operate weapons systems without required legal authorization

### 3.3 Surveillance Without Consent

Uploading or executing tools that capture audio, video, or sensor data from persons without
their informed consent, including but not limited to:
- Spyware and stalkerware
- Hidden camera or microphone applications
- Screen capture tools deployed on third-party systems without authorization

Exception: Security monitoring and conferencing applications operated with full user consent
and disclosed in the workload manifest are permitted.

### 3.4 Botnet and DDoS Tools

Uploading, storing, or executing:
- Command-and-control (C2) software for botnet operation
- Distributed denial-of-service (DDoS) attack tools
- Code designed to compromise other systems without authorization

### 3.5 Other Illegal Content

Content that is illegal under applicable law, including but not limited to:
- Material that facilitates human trafficking
- Counterfeit currency or financial instruments
- Malware, ransomware, or destructive code intended for third-party systems
- Content that infringes third-party intellectual property rights (see DMCA policy)

---

## 4. Workload Manifest Requirement

**All workload purchases require a truthful Workload Manifest** declaring:

| Field | Requirement |
|-------|-------------|
| `purpose_category` | Must accurately describe the workload's primary purpose |
| `description` | Minimum 20 characters; must describe actual intended use |
| `network_access` | Must accurately declare whether and how the workload accesses the network |
| `external_endpoints` | Must list all intended outbound destinations if network access is declared |
| `hardware_access` | Must be `true` if the workload uses GPIO, serial, USB, or physical hardware |

**False declarations in the manifest are grounds for:**
- Immediate DID suspension
- Forfeiture of any wallet balance without refund
- Law enforcement referral if the false declaration conceals illegal activity

The manifest is stored permanently as part of the order audit trail and may be produced in
legal proceedings.

---

## 5. Provider (Host Node) Obligations

By operating a SoHoLINK Provider node, you agree to:

### 5.1 Content Review

- Review content flagged by the platform's automated moderation system within **72 hours**
- Submit an LBTAS rating for flagged content as required by the platform's review workflow
- Report content you believe is illegal to appropriate authorities within the applicable window

### 5.2 CSAM Reporting

If you discover or are notified of apparent CSAM on your node:
- Report to NCMEC CyberTipline (cybertipline.org) within 24 hours if the platform has not
  already filed an automatic report
- Preserve all evidence — do not delete content before law enforcement has had an opportunity
  to act
- Contact `legal@soholink.network` to coordinate the response

### 5.3 Law Enforcement Cooperation

- Respond to valid law enforcement requests as required by applicable law
- Preserve data subject to legal hold orders
- Do not tip off subjects of active law enforcement investigations

### 5.4 No Obstruction

Providers must not:
- Delete or modify evidence after becoming aware of illegal content on their node
- Assist users in evading detection or law enforcement
- Accept payment, threats, or other inducements to conceal illegal content

**Violation of this section may constitute obstruction of justice (18 U.S.C. § 1519).**

---

## 6. Requester (Buyer) Obligations

By purchasing compute or storage through SoHoLINK, you agree to:

- Provide truthful and accurate Workload Manifests for all purchases
- Use purchased capacity only for the purposes declared in the manifest
- Comply with all applicable laws in your jurisdiction and the Provider's jurisdiction
- Accept that your DID, transaction records, workload manifests, and wallet history may be
  produced in legal proceedings

---

## 7. Platform Enforcement

SoHoLINK may take the following actions in response to violations:

| Violation | Response |
|-----------|----------|
| CSAM upload | Immediate DID suspension + NCMEC report + evidence preservation |
| Other illegal content | DID suspension + law enforcement referral |
| False manifest declaration | DID suspension + wallet balance forfeiture |
| Repeated copyright infringement | DID suspension (after 2+ valid DMCA notices in 12 months) |
| Threats to host providers | DID suspension + law enforcement referral |
| Botnet / DDoS tool | Immediate DID suspension + evidence preservation |
| Surveillance tool (non-consented) | DID suspension |

**DID suspension** means the `did:key:...` identifier is added to the `blocked_dids` table
and propagated to all federation nodes. Suspended DIDs cannot authenticate or purchase services.

Suspended accounts may appeal by emailing `legal@soholink.network` with supporting
documentation. Appeals will not be considered for CSAM-related suspensions.

---

## 8. Payment Terms

- All marketplace purchases are debited from the prepaid sats wallet at time of purchase
- The platform fee is 1% of the net transaction amount
- Wallet balances are non-refundable except as follows:
  - Workload cancellation: proportional refund for unused hours (computed at cancellation time)
  - Provider failure: full refund if the workload fails to start within 1 hour
  - Platform error: full refund at platform discretion
- Wallets associated with suspended accounts are frozen pending legal review

---

## 9. Privacy

SoHoLINK stores the following data associated with your DID:
- Wallet balance and transaction history
- Order records including workload manifests
- LBTAS reputation scores and rating history
- Device token hashes (not the raw tokens)

SoHoLINK does not store:
- Raw device tokens after they are hashed on receipt
- Content of workload outputs (stored by Provider, not platform)
- Personally identifiable information beyond what you voluntarily provide

Data may be disclosed to law enforcement as required by law (see `docs/LAW_ENFORCEMENT.md`).

---

## 10. Disclaimer of Warranties

THE SOHOLINK PLATFORM IS PROVIDED "AS IS" WITHOUT WARRANTY OF ANY KIND. THE OPERATOR DOES NOT
WARRANT THAT THE PLATFORM WILL BE UNINTERRUPTED, ERROR-FREE, OR FREE OF HARMFUL COMPONENTS.

---

## 11. Limitation of Liability

TO THE MAXIMUM EXTENT PERMITTED BY LAW, THE OPERATOR'S TOTAL LIABILITY TO ANY USER FOR ANY
CLAIM ARISING OUT OF OR RELATING TO THESE TERMS SHALL NOT EXCEED THE AMOUNTS PAID BY THAT
USER TO THE PLATFORM IN THE 12 MONTHS PRECEDING THE CLAIM.

THE OPERATOR IS NOT LIABLE FOR CONTENT STORED OR PROCESSED BY PROVIDER NODES.

---

## 12. Governing Law and Dispute Resolution

> **TODO: Complete with legal counsel.**
>
> Insert:
> - Governing law (e.g., "Laws of the State of [State], USA")
> - Dispute resolution mechanism (arbitration clause, class action waiver, or court)
> - Jurisdiction and venue
> - Notice requirements for legal claims

---

## 13. Changes to These Terms

SoHoLINK may update these Terms at any time. Users will be notified via the dashboard or API
response headers. Continued use after 30 days constitutes acceptance of the updated Terms.

---

## 14. Contact

For legal inquiries, law enforcement requests, DMCA notices, or safety reports:

**Email:** `legal@soholink.network` *(configure before launch)*
**DMCA Agent:** See `docs/DMCA.md`
**Law enforcement:** See `docs/LAW_ENFORCEMENT.md`

---

*This document is a draft template. It has not been reviewed by legal counsel and does not
constitute legal advice. Operators must have this document reviewed by a licensed attorney
in their jurisdiction before presenting it to users.*
