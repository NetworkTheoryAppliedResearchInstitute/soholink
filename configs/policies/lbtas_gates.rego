package lbtas_gates

# LBTAS reputation-gated access policies for SoHoLINK
# These policies enforce minimum reputation scores for resource access.

default allow_resource_transaction = false
default allow_provisional_access = false
default allow_premium_access = false

# Minimum score requirements by resource type
min_scores := {
    "compute": {"user": 30, "provider": 40},
    "storage": {"user": 25, "provider": 35},
    "print":   {"user": 35, "provider": 45},
    "portal":  {"user": 20, "provider": 30},
}

# Allow resource access only if both parties meet minimum scores
allow_resource_transaction if {
    input.resource_type
    input.user_score >= min_scores[input.resource_type].user
    input.provider_score >= min_scores[input.resource_type].provider
}

# New users get provisional access with limits
allow_provisional_access if {
    input.user_score == 0
    input.transaction_count == 0
    input.resource_type != "print"
    input.transaction_value <= 1000
}

# High-reputation users get premium access
allow_premium_access if {
    input.user_score >= 80
    input.provider_score >= 75
}
