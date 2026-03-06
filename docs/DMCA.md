# SoHoLINK — DMCA Designated Agent & Copyright Policy

**Item 6 of 8 — Pre-Launch Legal Compliance**

---

## 1. Why This Document Exists

The Digital Millennium Copyright Act (17 U.S.C. § 512) requires online service providers to
designate a registered agent to receive copyright infringement notices in order to qualify for
safe-harbor protection against liability for user-uploaded content.

**Action required before public launch:** Register the designated agent with the US Copyright
Office at https://www.copyright.gov/dmca-agent/ and complete the fields below.

---

## 2. Designated Agent Information

> **TODO: Complete before launch. Do NOT launch with placeholder values.**

| Field | Value |
|-------|-------|
| **Service provider legal name** | `[Full legal name of entity operating SoHoLINK]` |
| **Designated agent full name** | `[Name of designated individual or legal department]` |
| **Mailing address** | `[Street, City, State, ZIP, Country]` |
| **Email address** | `legal@soholink.network` *(placeholder — confirm before registration)* |
| **Phone number** | `[Phone number]` |
| **Copyright Office registration date** | `[Date — update once registered]` |
| **Copyright Office agent ID** | `[Assigned after registration]` |

**Registration URL:** https://www.copyright.gov/dmca-agent/
**Fee:** $6 per designation (as of 2026); renewable every 3 years.

---

## 3. Designated Agent Contact (Public-Facing)

Once registered, this information must appear publicly on the SoHoLINK website or API root:

```
DMCA Agent: [Name]
SoHoLINK — Network Theory Applied Research Institute
[Mailing Address]
Email: legal@soholink.network
```

Add this to: `ui/dashboard/index.html` footer, API `/api/legal` endpoint, and any public-facing website.

---

## 4. Inbound DMCA Takedown Procedure

### 4.1 Valid DMCA Notice Requirements (17 U.S.C. § 512(c)(3))

A takedown notice must contain:

1. Physical or electronic signature of the copyright owner (or authorized agent)
2. Identification of the copyrighted work claimed to be infringed
3. Identification of the infringing material with enough detail to locate it (IPFS CID, file URL)
4. Contact information of the complaining party
5. Statement of good-faith belief that the use is not authorized
6. Statement under penalty of perjury that the information is accurate

Notices missing any required element should be treated as defective. Respond asking for the
missing information before taking action.

### 4.2 Response Timeline

| Step | Timeframe |
|------|-----------|
| Acknowledge receipt of notice | Within 24 hours |
| Review notice for completeness | Within 48 hours |
| Take down or block access to identified content | Within 14 days of valid notice |
| Notify affected user of takedown | Upon or before removal |

### 4.3 Takedown Actions

For **IPFS-stored content** (CID identified in notice):
```bash
# Add CID to content blocklist (prevents re-upload)
curl -XPOST http://localhost:8080/api/admin/blocklist/hashes \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"hash_sha256":"<sha256_of_content>","reason":"dmca_takedown","source":"manual"}'

# Unpin from local IPFS node (removes from local availability)
ipfs pin rm <CID>
```

For **user content in filesystem pool**: Remove the file from `files/<hash>` directory and
add the SHA-256 to the content blocklist.

For **user account** (repeated infringers — 17 U.S.C. § 512(i) repeat infringer policy):
```bash
curl -XPOST http://localhost:8080/api/admin/blocklist/dids \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{"did":"did:key:<DID>","reason":"dmca_repeat_infringer"}'
```

### 4.4 Repeat Infringer Policy

SoHoLINK will terminate the accounts (DID suspension) of users who are repeat copyright
infringers. "Repeat infringer" means a user who has received two or more valid takedown
notices within any 12-month period.

---

## 5. Counter-Notice Procedure (17 U.S.C. § 512(g))

If a user believes their content was removed in error:

1. User submits counter-notice to `legal@soholink.network` containing:
   - Identification of the removed content and its former location
   - Statement under penalty of perjury of good-faith belief that removal was a mistake
   - User's name, address, phone number, and consent to federal court jurisdiction

2. SoHoLINK forwards counter-notice to the original complainant within 3 business days

3. If complainant does not file a lawsuit within 10-14 business days, content may be restored

---

## 6. CSAM Exception

Content depicting child sexual abuse material (CSAM) is **not eligible** for counter-notice
restoration. CSAM reports are handled exclusively under the NCMEC CyberTipline procedure
documented in `docs/LAW_ENFORCEMENT.md`. CDA Section 230 does NOT apply to CSAM.

---

## 7. Automated Detection Integration

SoHoLINK's content safety infrastructure (`internal/moderation/hashmatch.go`) maintains a
local SHA-256 blocklist. DMCA takedowns should be recorded there to prevent re-upload:

```go
// Platform administrator adds hash after takedown
hashChecker.AddHash(ctx, sha256hex, "illegal_content", "dmca_takedown", adminDID)
```

The blocklist table (`content_hash_blocklist`) is auditable and admissible as evidence of
good-faith takedown compliance.

---

## 8. Record-Keeping

Retain the following for each takedown for a minimum of 3 years:

- Original notice (all fields)
- Date and time of receipt
- Identity of the complaining party
- Content identifier (CID / hash / URL)
- Action taken and date
- Any counter-notice received

Store records in an append-only log. Do not modify or delete takedown records.
