package soholink.safety

# Platform-level content safety prohibition rules.
# These rules apply to ALL workloads and provider nodes in the federation.
# Individual provider OPA policies CANNOT override these prohibitions.
#
# Evaluation: a workload is allowed only if ALL deny_* rules are false.
# The deny_reasons set accumulates human-readable rejection codes for API responses.
#
# Input schema:
#   input.cid                          — IPFS CID of uploaded content, or ""
#   input.manifest.purpose_category    — declared purpose string
#   input.manifest.network_access      — "none"|"declared_only"|"unrestricted"
#   input.manifest.external_endpoints  — array of declared outbound URLs/IPs
#   input.manifest.hardware_access     — bool (GPIO/serial/USB declared)
#   input.manifest.capabilities        — array of declared capabilities
#   input.manifest.output_destinations — array of output destination strings
#
# data.blocked_cids — set of blocked IPFS CIDs (injected from content_hash_blocklist at eval time)

import future.keywords.if
import future.keywords.in
import future.keywords.contains

# Default: deny unless all checks pass
default allow := false

allow if {
    not deny_illegal_content
    not deny_violence_execution
    not deny_surveillance_tool
    not deny_botnet_operation
}

# ── Rule 1: Illegal content hash match ────────────────────────────────────────
# Blocks workloads referencing a CID that appears in the content_hash_blocklist.
deny_illegal_content if {
    input.cid != ""
    input.cid in data.blocked_cids
}

# ── Rule 2: Unmanifested hardware + unrestricted network ──────────────────────
# A workload declaring hardware access (GPIO/serial) AND unrestricted network
# access is a strong indicator of a remote-execution / physical-attack tool.
deny_violence_execution if {
    input.manifest.hardware_access == true
    input.manifest.network_access == "unrestricted"
}

# ── Rule 3: Potential surveillance tool ───────────────────────────────────────
# Captures workloads that declare video/audio capture AND send data externally,
# but are NOT filed under an acknowledged monitoring/conferencing purpose.
deny_surveillance_tool if {
    "capture_video" in input.manifest.capabilities
    count(input.manifest.external_endpoints) > 0
    not input.manifest.purpose_category in {"security_monitoring", "conferencing"}
}

deny_surveillance_tool if {
    "capture_audio" in input.manifest.capabilities
    count(input.manifest.external_endpoints) > 0
    not input.manifest.purpose_category in {"security_monitoring", "conferencing"}
}

# ── Rule 4: Potential botnet operation ────────────────────────────────────────
# A workload with no declared output destinations AND unrestricted network access
# is consistent with DDoS participation or C2 check-in patterns.
deny_botnet_operation if {
    count(input.manifest.output_destinations) == 0
    input.manifest.network_access == "unrestricted"
}

# ── Deny reasons set ──────────────────────────────────────────────────────────
deny_reasons contains reason if {
    deny_illegal_content
    reason := "illegal_content_hash_match"
}

deny_reasons contains reason if {
    deny_violence_execution
    reason := "unmanifested_hardware_plus_unrestricted_network"
}

deny_reasons contains reason if {
    deny_surveillance_tool
    reason := "potential_surveillance_tool"
}

deny_reasons contains reason if {
    deny_botnet_operation
    reason := "potential_botnet_operation"
}
