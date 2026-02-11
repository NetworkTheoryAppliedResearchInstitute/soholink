package resource_sharing

# Resource sharing authorization policies for SoHoLINK
# These policies govern access to compute, storage, print, and portal resources.

default allow_compute_submit = false
default allow_storage_upload = false
default allow_print_submit = false
default allow_portal_access = false

# Compute job submission requires verified identity and job within limits
allow_compute_submit if {
    input.action == "compute_submit"
    input.user_did != ""
    input.authenticated == true
    job_within_limits(input.job_spec)
}

job_within_limits(spec) if {
    spec.cpu_cores <= 4
    spec.memory_mb <= 8192
    spec.timeout_seconds <= 3600
    spec.disk_mb <= 10240
}

# Storage upload requires authenticated identity
allow_storage_upload if {
    input.action == "storage_upload"
    input.user_did != ""
    input.authenticated == true
}

# Print job requires authenticated identity
allow_print_submit if {
    input.action == "print_submit"
    input.user_did != ""
    input.authenticated == true
}

# Portal access requires authenticated identity
allow_portal_access if {
    input.action == "portal_access"
    input.user_did != ""
    input.authenticated == true
}

# Deny reasons for debugging
deny_reasons contains reason if {
    input.user_did == ""
    reason := "no_user_did"
}

deny_reasons contains reason if {
    input.authenticated != true
    reason := "not_authenticated"
}
