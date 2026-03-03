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

# ---------------------------------------------------------------------------
# Mobile node policies
# ---------------------------------------------------------------------------

# task_replication_factor returns the number of replicas required for a task
# assigned to a given node class.  mobile-android requires a shadow desktop
# replica so results can be verified before the HTLC payment releases.
# android-tv and desktop tasks require only the single primary replica.
#
# R1 note: callers MUST supply input.node.class.  If the field is absent OPA
# evaluates this rule as undefined and falls through to the default below,
# yielding 1 (no shadow replica).  The allow_mobile_task rule guards against
# this by requiring input.node.class to be non-empty.
task_replication_factor := factor if {
    factor := {
        "mobile-android": 2,
        "android-tv":     1,
        "mobile-ios":     0,
        "desktop":        1
    }[input.node.class]
}

# Default replication factor for unknown or missing node classes.
# allow_mobile_task requires input.node.class != "" to prevent unintentional
# fallback to 1 for mobile-android tasks (R1).
default task_replication_factor := 1

# mobile_eligible_task is true when a submitted task may be routed to a
# mobile node.  Tasks must declare a segment duration within the mobile
# node's maximum to be eligible.
mobile_eligible_task if {
    input.task.segment_duration_seconds > 0
    input.task.segment_duration_seconds <= 120
    input.task.wasm_cid != ""
}

# mobile_eligible_task is false by default.
default mobile_eligible_task := false

# android_tv_eligible_task is true for tasks that may run on an Android TV
# node.  Android TV has no segment-duration constraint but still requires a
# Wasm task format.
android_tv_eligible_task if {
    input.task.wasm_cid != ""
}

default android_tv_eligible_task := false

# allow_mobile_task combines authentication, eligibility, and resource limits.
#
# R1 fix: input.node.class must be non-empty so callers cannot accidentally
# omit it and bypass the task_replication_factor map lookup (defaulting to 1
# and skipping the required shadow replica for mobile-android).
#
# R2 fix: job_within_limits now receives input.task instead of input.job_spec
# so that mobile task submissions use a single consistent input schema.
# Callers must provide: input.task.{segment_duration_seconds, wasm_cid,
# cpu_cores, memory_mb, timeout_seconds, disk_mb}.
allow_mobile_task if {
    input.action == "mobile_task_submit"
    input.user_did != ""
    input.authenticated == true
    input.node.class != ""
    mobile_eligible_task
    job_within_limits(input.task)
}

default allow_mobile_task := false

# ---------------------------------------------------------------------------
# HTLC lifecycle authorization (R3)
# ---------------------------------------------------------------------------

# allow_htlc_cancel is true when the coordinator is permitted to cancel a
# hold invoice (releasing funds back to the payer).  Cancellation is allowed
# when:
#   - the requesting entity is the coordinator (authenticated coordinator DID)
#   - the reason is a recognised failure mode (verification failure or timeout)
#
# Required input fields:
#   input.action          == "htlc_cancel"
#   input.coordinator_did  — non-empty DID of the coordinator
#   input.cancel_reason   — "verification_failed" | "node_timeout" | "node_disconnected"
allow_htlc_cancel if {
    input.action == "htlc_cancel"
    input.coordinator_did != ""
    input.cancel_reason != ""
    valid_cancel_reasons[input.cancel_reason]
}

default allow_htlc_cancel := false

valid_cancel_reasons := {
    "verification_failed",
    "node_timeout",
    "node_disconnected",
}

# allow_htlc_settle is true when the coordinator may settle a hold invoice
# (releasing funds to the provider).  Settlement is only permitted after the
# shadow-replica result hash has been verified.
#
# Required input fields:
#   input.action              == "htlc_settle"
#   input.coordinator_did      — non-empty DID of the coordinator
#   input.shadow_verified      — must be true (shadow replica result matched)
allow_htlc_settle if {
    input.action == "htlc_settle"
    input.coordinator_did != ""
    input.shadow_verified == true
}

default allow_htlc_settle := false

# allow_mobile_preempt is true when the coordinator may reassign a mobile
# workload to a desktop node (e.g. because the mobile node disconnected).
#
# Required input fields:
#   input.action          == "mobile_preempt"
#   input.coordinator_did  — non-empty DID of the coordinator
#   input.preempt_reason  — "node_disconnected" | "node_timeout" | "thermal_throttle"
allow_mobile_preempt if {
    input.action == "mobile_preempt"
    input.coordinator_did != ""
    input.preempt_reason != ""
    valid_preempt_reasons[input.preempt_reason]
}

default allow_mobile_preempt := false

valid_preempt_reasons := {
    "node_disconnected",
    "node_timeout",
    "thermal_throttle",
}
