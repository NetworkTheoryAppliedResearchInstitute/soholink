# SoHoLINK — Acceptable Use Policy

*Version: 1.0 | Effective Date: 2026-03-06*
*Operator: Network Theory Applied Research Institute*

> **DRAFT — Review by legal counsel required before public launch.**

This Acceptable Use Policy ("AUP") governs all use of the SoHoLINK federated compute marketplace,
including Provider nodes (hardware hosts), Requesters (workload submitters), and any other
participant interacting with the platform API.

By registering a device or submitting workloads to the network, you agree to comply with this AUP
in addition to the [Terms of Service](TERMS_OF_SERVICE.md).

---

## 1. Prohibited Workloads and Content

The following categories are **absolutely prohibited** on SoHoLINK and will result in immediate
account suspension, DID block, and potential law enforcement referral.

### 1.1 Child Sexual Abuse Material (CSAM)

Uploading, storing, transmitting, requesting, or facilitating access to any material that sexually
exploits minors is a **federal crime** (18 U.S.C. § 2252) and is absolutely prohibited with no
exceptions.

- Content hashes are automatically screened against NCMEC databases on every upload
- Any match triggers immediate DID block, platform-wide federation propagation of the block, and
  a mandatory NCMEC CyberTipline report within 24 hours (18 U.S.C. § 2258A)
- Accounts associated with CSAM are **permanently suspended without appeal**

### 1.2 Malware and Command-and-Control Infrastructure

Uploading, storing, executing, or distributing:

- Malware, ransomware, spyware, or other malicious code intended for third-party systems
- Command-and-control (C2) servers for botnet operation
- Exploit kits, dropper payloads, or code whose primary purpose is unauthorized system compromise
- Credential harvesters or phishing infrastructure

### 1.3 Distributed Denial-of-Service (DDoS) Tools

Operating or hosting tools designed to:

- Flood third-party networks or services with traffic
- Amplify reflection attacks (DNS, NTP, SSDP, etc.)
- Stress-test infrastructure you do not own without written authorization from the target

### 1.4 Cryptomining Without Host Consent

Running cryptocurrency mining workloads (proof-of-work or similar) **without the explicit, written
consent of the hardware Provider** who bears the electricity and hardware wear costs. Providers
must opt in to mining workloads explicitly via their provisioning limits configuration.

### 1.5 Surveillance and Stalkerware

Deploying tools that:

- Capture audio, video, location, or keystrokes from persons without their informed, explicit
  consent
- Operate as stalkerware concealed from the device owner
- Conduct screen capture or screenshot harvesting on third-party systems without authorization

*Exception:* Security monitoring and conferencing software operated with full user consent and
declared in the workload manifest is permitted.

### 1.6 Network Reconnaissance and IP Scanning

Conducting unauthorized:

- Port scanning, banner grabbing, or enumeration of IP ranges you do not own or have written
  permission to test
- Vulnerability scanning against third-party infrastructure
- BGP hijacking, ARP spoofing, or other network-layer attacks against third parties

### 1.7 Sanctions and Export Control Violations

Using SoHoLINK to provide services to, or on behalf of, entities listed on:

- OFAC Specially Designated Nationals (SDN) list
- EU consolidated financial sanctions list
- UN Security Council consolidated sanctions list

Or to circumvent export controls under EAR/ITAR regulations.

### 1.8 Other Illegal Content

Any content or workload that is illegal under applicable federal, state, or international law,
including but not limited to:

- Material facilitating human trafficking (18 U.S.C. § 1591)
- Counterfeit currency or fraudulent financial instruments
- Copyright-infringing content subject to valid DMCA takedown notices (see [DMCA Policy](DMCA.md))
- Violence-enabling code or weapons guidance without required legal authorization

---

## 2. Provider Responsibilities

Hardware operators running SoHoLINK Provider nodes are responsible for:

1. **Configuring provisioning limits** — setting appropriate CPU, memory, storage, and
   workload-type restrictions in their OPA policy (`configs/policies/resource_sharing.rego`)
2. **Monitoring auto-accept rules** — reviewing the rental audit log (`rental_audit` table)
   periodically to ensure auto-accepted workloads comply with this AUP
3. **Reporting violations** — contacting abuse@soholink.network immediately upon discovering
   suspected AUP violations on their hardware
4. **Securing access** — maintaining physical and network security for hardware; preventing
   unauthorized third-party access to the node

Providers are not liable for workloads that pass platform screening and their own policy rules in
good faith, but must cooperate with any investigation.

---

## 3. Enforcement

### 3.1 Technical Controls

SoHoLINK enforces this AUP through multiple layers:

| Layer | Mechanism |
|-------|-----------|
| Content hash screening | SHA-256 checked against CSAM and DMCA blocklists on every upload |
| OPA policy evaluation | Workload manifests evaluated against `resource_sharing.rego` |
| DID blocklist | Violating DIDs blocked and propagated across the federation |
| Rental audit log | Every auto-accept/reject decision recorded in `rental_audit` table |
| Manual review | Flagged accounts reviewed by the trust & safety team |

### 3.2 Sanctions

Violations may result in any combination of:

- Immediate suspension of the violating DID (temporary or permanent)
- Federation-wide propagation of the block to all known nodes
- Forfeiture of wallet balance associated with the violating DID
- Referral to law enforcement for criminal violations
- Civil legal action for recoverable damages

### 3.3 Appeal Process

To appeal a DID suspension (except CSAM, which is not appealable):

1. Email **appeals@soholink.network** within **30 days** of the suspension notice
2. Include your DID, a description of your use case, and evidence that your workloads comply with
   this AUP
3. Appeals are reviewed within 10 business days
4. Decisions after the appeal process are final

---

## 4. Reporting Violations

To report suspected AUP violations by other network participants:

| Category | Contact |
|----------|---------|
| General abuse | abuse@soholink.network |
| CSAM / child safety | safety@soholink.network *(also reported to NCMEC)* |
| Security vulnerabilities | security@soholink.network *(see [Security Policy](SECURITY.md))* |

Include the relevant DID, workload ID, order ID, or content CID where available.

---

## 5. Amendments

Network Theory Applied Research Institute reserves the right to amend this AUP at any time.
Material changes will be:

1. Published at `docs/AUP.md` in the main repository with at least **14 days' notice**
2. Announced in the release notes for the next SoHoLINK release
3. Sent to the registered contact email of active Provider nodes

Continued use of the platform after the effective date of any amendment constitutes acceptance of
the revised AUP.

---

## 6. Cross-References

| Document | Purpose |
|----------|---------|
| [Terms of Service](TERMS_OF_SERVICE.md) | Full contractual terms, payment, disputes, liability |
| [Privacy Policy](PRIVACY_POLICY.md) | Data collection, retention, and user rights |
| [Security Policy](SECURITY.md) | Vulnerability disclosure and bug reporting |
| [DMCA Policy](DMCA.md) | Copyright takedown procedures |
| [Law Enforcement Guide](LAW_ENFORCEMENT.md) | Legal process and data preservation requests |

---

*This document is provided for informational purposes and does not constitute legal advice.
Consult qualified legal counsel before deploying SoHoLINK in a regulated environment.*
