package soholink.network

# Network egress filtering policy for SoHoLINK workloads.
# These rules guard against workloads that attempt to reach private network
# addresses — a common pattern in lateral-movement and SSRF attacks.
#
# Full enforcement of these rules at runtime requires the Wasm sandbox host
# to call OPA before granting a workload outbound network access.
#
# Input schema:
#   input.manifest.external_endpoints  — declared outbound URLs/IPs
#   input.manifest.network_access      — "none"|"declared_only"|"unrestricted"
#
# deny_private_network: true when any declared endpoint resolves to RFC 1918
#   or loopback space. Callers should reject the workload submission.
# warn_unrestricted_network: true when network_access is "unrestricted".
#   Callers should log a warning but may still allow (policy-dependent).

import future.keywords.if
import future.keywords.in

# Block workloads that declare RFC 1918 / loopback endpoints
default deny_private_network := false

deny_private_network if {
    endpoint := input.manifest.external_endpoints[_]
    is_rfc1918(endpoint)
}

# Warn on unrestricted network access (not a hard deny — logged by callers)
default warn_unrestricted_network := false

warn_unrestricted_network if {
    input.manifest.network_access == "unrestricted"
}

# ── RFC 1918 + loopback detection ─────────────────────────────────────────────

is_rfc1918(a) if { startswith(a, "10.") }
is_rfc1918(a) if { startswith(a, "192.168.") }
is_rfc1918(a) if { startswith(a, "172.16.") }
is_rfc1918(a) if { startswith(a, "172.17.") }
is_rfc1918(a) if { startswith(a, "172.18.") }
is_rfc1918(a) if { startswith(a, "172.19.") }
is_rfc1918(a) if { startswith(a, "172.20.") }
is_rfc1918(a) if { startswith(a, "172.21.") }
is_rfc1918(a) if { startswith(a, "172.22.") }
is_rfc1918(a) if { startswith(a, "172.23.") }
is_rfc1918(a) if { startswith(a, "172.24.") }
is_rfc1918(a) if { startswith(a, "172.25.") }
is_rfc1918(a) if { startswith(a, "172.26.") }
is_rfc1918(a) if { startswith(a, "172.27.") }
is_rfc1918(a) if { startswith(a, "172.28.") }
is_rfc1918(a) if { startswith(a, "172.29.") }
is_rfc1918(a) if { startswith(a, "172.30.") }
is_rfc1918(a) if { startswith(a, "172.31.") }
is_rfc1918(a) if { startswith(a, "127.") }
is_rfc1918(a) if { startswith(a, "169.254.") }   # link-local
is_rfc1918(a) if { a == "::1" }                   # IPv6 loopback
is_rfc1918(a) if { startswith(a, "fc00:") }       # IPv6 ULA
is_rfc1918(a) if { startswith(a, "fd") }           # IPv6 ULA (fd00::/8)
is_rfc1918(a) if { startswith(a, "localhost") }    # hostname form
