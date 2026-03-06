# SoHoLINK — Law Enforcement Response Procedure

**Item 7 of 8 — Pre-Launch Legal Compliance**

---

## 1. Purpose

This document defines how SoHoLINK operators respond to law enforcement requests, mandatory
reporting obligations, and evidence preservation requirements. All node operators who run
a coordinator or storage node must read and follow this procedure.

**Legal contact:** `legal@soholink.network` *(placeholder — configure before launch)*

---

## 2. Mandatory Reporting — CSAM (18 U.S.C. § 2258A)

### 2.1 Legal Obligation

Under 18 U.S.C. § 2258A, any electronic service provider that obtains actual knowledge of
apparent child sexual abuse material (CSAM) on its system **must report it to NCMEC within
24 hours** of obtaining that knowledge, via the CyberTipline. Failure to report is a federal
crime.

This obligation applies regardless of platform size, revenue, or whether the content was
immediately removed.

### 2.2 CyberTipline Reporting

**URL:** https://www.missingkids.org/gethelpnow/cybertipline
**Direct API:** https://www.missingkids.org/content/ncmec/en_us/cybertipline.html

**Required information to include in the report:**
- Electronic communication service provider name ("SoHoLINK")
- Date/time of discovery (UTC, RFC3339)
- Type of apparent violation (CSAM)
- Content identifier: IPFS CID and/or SHA-256 hash
- Geographic information if available (IP address of uploader, region)
- User account identifier: DID (`did:key:...`) of the uploader
- Any available metadata from the LBTAS transaction record

**24-hour clock starts when:** a content hash check returns a CSAM match, OR a host provider
submits an LBTAS rating identifying CSAM, OR any other mechanism produces actual knowledge.

### 2.3 Post-Report Actions

After submitting to NCMEC:

1. **Do not delete the content immediately** — preserve for potential law enforcement seizure
2. Block the content hash from further access: `POST /api/admin/blocklist/hashes`
3. Suspend the uploader DID: `POST /api/admin/blocklist/dids` (reason: "csam_report")
4. Preserve all evidence (see Section 4)
5. Record the CyberTipline report ID in internal logs

---

## 3. Incident Classification & Response Times

| Incident Type | Classification | Response Target |
|--------------|----------------|-----------------|
| CSAM (apparent) | **CRITICAL** | Report to NCMEC within **24 hours** |
| Other apparent child exploitation material | **CRITICAL** | 24 hours |
| Weapons / explosive manufacturing | HIGH | 48 hours |
| Terrorism-related content | HIGH | 48 hours (also contact FBI tip line) |
| Human trafficking facilitation | HIGH | 48 hours |
| Non-CSAM illegal content | MEDIUM | 72 hours |
| Suspected law enforcement impersonation | LOW | Review within 5 days |

---

## 4. Evidence Preservation

### 4.1 What to Preserve

When illegal content is detected or a law enforcement request is received, immediately
preserve and do NOT alter or delete:

| Item | Location | Retention |
|------|----------|-----------|
| File content (as-is) | IPFS pin + filesystem copy | 90 days minimum |
| SHA-256 hash of content | `content_hash_blocklist` table | Permanent |
| IPFS CID | `content_hash_blocklist` table | Permanent |
| Uploader DID | `blocked_dids` table | Permanent |
| Transaction record | `resource_transactions` table | 90 days minimum |
| Workload manifest | `orders.manifest_json` column | 90 days minimum |
| LBTAS ratings for the transaction | `lbtas_ratings` table | 90 days minimum |
| IP address logs (if captured) | Server access logs | 90 days |
| Wallet topup records (uploader) | `wallet_topups` table | 90 days |
| Blockchain anchor (if anchored) | `blockchain_batches` table | Permanent |

### 4.2 Chain of Custody

For each preserved evidence item:

```
1. Record the discovery timestamp (UTC) — do not round or approximate
2. Compute SHA-256 of each file; record in an evidence manifest
3. Store the evidence manifest itself in an append-only log
4. Do not modify evidence files; make read-only copies if processing is needed
5. Record the identity of each person who accesses evidence files
6. If law enforcement requests a copy: hash the copy before transmission and record
```

### 4.3 Evidence Manifest Format

Create `evidence_<incident_id>.json` for each incident:

```json
{
  "incident_id": "inc_2026MMDD_NNNN",
  "discovered_at": "2026-03-05T14:32:00Z",
  "discovered_by": "csam_hash_check",
  "content_sha256": "aabbcc...",
  "ipfs_cid": "QmXxx...",
  "uploader_did": "did:key:zXxx...",
  "uploader_ip": "redacted_or_actual",
  "transaction_id": "txn_xxx",
  "order_id": "ord_xxx",
  "ncmec_report_id": "pending",
  "ncmec_reported_at": null,
  "evidence_files": [
    {"path": "/evidence/inc_NNNN/content.bin", "sha256": "aabbcc...", "size_bytes": 12345}
  ],
  "access_log": [
    {"accessor": "did:key:admin", "accessed_at": "2026-03-05T14:33:00Z", "action": "preserved"}
  ]
}
```

### 4.4 Legal Hold

When a legal hold or preservation order is received:

1. Immediately suspend any automated deletion or data-expiry jobs for the affected records
2. Record the hold in an internal register: date, case number (if provided), scope, requestor
3. Do not notify the subject of the hold unless legally required to do so
4. Hold remains active until formal written release from the requesting authority

---

## 5. Law Enforcement Requests

### 5.1 Types of Requests

| Request Type | Authority Required | Response |
|-------------|-------------------|----------|
| Preservation request | No legal process required | Honor within 24h; preserve 90 days |
| Subpoena (civil) | Court-issued | Consult legal counsel; respond within deadline |
| Grand jury subpoena | Court-issued | Consult legal counsel; respond within deadline |
| Search warrant | Court-issued | Comply immediately; consult counsel |
| Emergency disclosure (18 U.S.C. § 2702(b)(8)) | No court order — imminent threat to life | Disclose immediately |
| NSL (National Security Letter) | FBI Director authorization | Consult specialist counsel |

### 5.2 Verification

Before disclosing any user data:

1. Verify the requesting officer's identity and agency (call back via official directory)
2. Confirm the request is within jurisdictional scope
3. Log all requests in the law enforcement request register (date, agency, officer, request type, data scope)
4. For non-emergency requests: do not respond before consulting legal counsel

### 5.3 What Can Be Disclosed Without a Court Order

Under 18 U.S.C. § 2702, the following may be disclosed voluntarily to law enforcement:

- Content if consent is obtained from the user
- Content in an emergency involving danger to life or serious physical injury
- Subscriber information (name, address, payment records) with a valid subpoena

**Do not disclose** the contents of communications or stored files without a valid warrant or
court order except in emergencies.

### 5.4 Non-US Requests

For requests from outside the United States:
- Route through the US Department of Justice MLAT (Mutual Legal Assistance Treaty) process
- Do not comply with direct foreign law enforcement requests without US DOJ involvement
- Exception: NCMEC CyberTipline reports are filed regardless of geography

---

## 6. Host Provider Physical Security (Silencing Attack Protection)

SoHoLINK host providers (node operators) who discover illegal content via the moderation
system face a specific physical security risk: a consumer who uploaded illegal content may
attempt to silence the host to prevent reporting.

**Platform protections built in:**

1. **Automatic evidence preservation** — as soon as a CSAM hash match triggers, evidence is
   recorded atomically in the SQLite store before any notification is sent to the uploader.

2. **No notification to uploader** — the platform never notifies the content uploader that
   their content was flagged. Only the host and platform administrators are notified.

3. **Tamper-evident audit trail** — the `content_hash_blocklist` and `blocked_dids` tables
   use `INSERT OR IGNORE` semantics: once a record is written, it is not overwritten.

4. **3-day reporting window** — per the LBTAS review protocol, hosts have up to 3 days after
   a moderation flag to notify authorities if NCMEC reporting has not yet occurred
   automatically. During this window, hosts should not confront the uploader directly.

**Host safety advice:**
- If you believe you are in immediate physical danger, call 911 (US) or local emergency services
- Do not delete evidence under any circumstances — this may be a crime (18 U.S.C. § 1519)
- Contact `legal@soholink.network` immediately if threatened
- Preserve all communications (texts, emails, calls) from the threatening party as evidence

---

## 7. FBI & Other Federal Resources

| Agency | Contact | Use Case |
|--------|---------|----------|
| NCMEC CyberTipline | cybertipline.org | CSAM mandatory report |
| FBI IC3 | ic3.gov | Cybercrime, extortion |
| FBI tips | tips.fbi.gov | Terrorism, serious threats |
| DHS CISA | cisa.gov/report | Critical infrastructure attacks |
| Local FBI Field Office | fbi.gov/contact-us/field-offices | Direct law enforcement contact |

---

## 8. Internal Contacts

| Role | Contact |
|------|---------|
| Legal counsel | `[Attorney / law firm — configure before launch]` |
| Platform security | `security@soholink.network` *(placeholder)* |
| Law enforcement liaison | `legal@soholink.network` *(placeholder)* |
| NCMEC reporting account | `[Register at missingkids.org before launch]` |
