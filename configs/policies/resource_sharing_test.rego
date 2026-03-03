package resource_sharing_test

import data.resource_sharing

# ==========================================================================
# allow_compute_submit
# ==========================================================================

test_compute_submit_allowed if {
	resource_sharing.allow_compute_submit with input as {
		"action": "compute_submit",
		"user_did": "did:example:user1",
		"authenticated": true,
		"job_spec": {
			"cpu_cores": 2,
			"memory_mb": 4096,
			"timeout_seconds": 1800,
			"disk_mb": 5120,
		},
	}
}

test_compute_submit_at_limits if {
	resource_sharing.allow_compute_submit with input as {
		"action": "compute_submit",
		"user_did": "did:example:user1",
		"authenticated": true,
		"job_spec": {
			"cpu_cores": 4,
			"memory_mb": 8192,
			"timeout_seconds": 3600,
			"disk_mb": 10240,
		},
	}
}

test_compute_submit_denied_missing_did if {
	not resource_sharing.allow_compute_submit with input as {
		"action": "compute_submit",
		"user_did": "",
		"authenticated": true,
		"job_spec": {
			"cpu_cores": 2,
			"memory_mb": 4096,
			"timeout_seconds": 1800,
			"disk_mb": 5120,
		},
	}
}

test_compute_submit_denied_not_authenticated if {
	not resource_sharing.allow_compute_submit with input as {
		"action": "compute_submit",
		"user_did": "did:example:user1",
		"authenticated": false,
		"job_spec": {
			"cpu_cores": 2,
			"memory_mb": 4096,
			"timeout_seconds": 1800,
			"disk_mb": 5120,
		},
	}
}

test_compute_submit_denied_wrong_action if {
	not resource_sharing.allow_compute_submit with input as {
		"action": "storage_upload",
		"user_did": "did:example:user1",
		"authenticated": true,
		"job_spec": {
			"cpu_cores": 2,
			"memory_mb": 4096,
			"timeout_seconds": 1800,
			"disk_mb": 5120,
		},
	}
}

test_compute_submit_denied_cpu_over_limit if {
	not resource_sharing.allow_compute_submit with input as {
		"action": "compute_submit",
		"user_did": "did:example:user1",
		"authenticated": true,
		"job_spec": {
			"cpu_cores": 5,
			"memory_mb": 4096,
			"timeout_seconds": 1800,
			"disk_mb": 5120,
		},
	}
}

test_compute_submit_denied_memory_over_limit if {
	not resource_sharing.allow_compute_submit with input as {
		"action": "compute_submit",
		"user_did": "did:example:user1",
		"authenticated": true,
		"job_spec": {
			"cpu_cores": 2,
			"memory_mb": 8193,
			"timeout_seconds": 1800,
			"disk_mb": 5120,
		},
	}
}

test_compute_submit_denied_timeout_over_limit if {
	not resource_sharing.allow_compute_submit with input as {
		"action": "compute_submit",
		"user_did": "did:example:user1",
		"authenticated": true,
		"job_spec": {
			"cpu_cores": 2,
			"memory_mb": 4096,
			"timeout_seconds": 3601,
			"disk_mb": 5120,
		},
	}
}

test_compute_submit_denied_disk_over_limit if {
	not resource_sharing.allow_compute_submit with input as {
		"action": "compute_submit",
		"user_did": "did:example:user1",
		"authenticated": true,
		"job_spec": {
			"cpu_cores": 2,
			"memory_mb": 4096,
			"timeout_seconds": 1800,
			"disk_mb": 10241,
		},
	}
}

# ==========================================================================
# allow_storage_upload
# ==========================================================================

test_storage_upload_allowed if {
	resource_sharing.allow_storage_upload with input as {
		"action": "storage_upload",
		"user_did": "did:example:user1",
		"authenticated": true,
	}
}

test_storage_upload_denied_not_authenticated if {
	not resource_sharing.allow_storage_upload with input as {
		"action": "storage_upload",
		"user_did": "did:example:user1",
		"authenticated": false,
	}
}

test_storage_upload_denied_missing_did if {
	not resource_sharing.allow_storage_upload with input as {
		"action": "storage_upload",
		"user_did": "",
		"authenticated": true,
	}
}

# ==========================================================================
# allow_print_submit
# ==========================================================================

test_print_submit_allowed if {
	resource_sharing.allow_print_submit with input as {
		"action": "print_submit",
		"user_did": "did:example:user1",
		"authenticated": true,
	}
}

test_print_submit_denied_not_authenticated if {
	not resource_sharing.allow_print_submit with input as {
		"action": "print_submit",
		"user_did": "did:example:user1",
		"authenticated": false,
	}
}

# ==========================================================================
# allow_portal_access
# ==========================================================================

test_portal_access_allowed if {
	resource_sharing.allow_portal_access with input as {
		"action": "portal_access",
		"user_did": "did:example:user1",
		"authenticated": true,
	}
}

test_portal_access_denied_not_authenticated if {
	not resource_sharing.allow_portal_access with input as {
		"action": "portal_access",
		"user_did": "did:example:user1",
		"authenticated": false,
	}
}

# ==========================================================================
# task_replication_factor
# ==========================================================================

test_replication_factor_mobile_android if {
	resource_sharing.task_replication_factor == 2 with input as {"node": {"class": "mobile-android"}}
}

test_replication_factor_android_tv if {
	resource_sharing.task_replication_factor == 1 with input as {"node": {"class": "android-tv"}}
}

test_replication_factor_mobile_ios if {
	resource_sharing.task_replication_factor == 0 with input as {"node": {"class": "mobile-ios"}}
}

test_replication_factor_desktop if {
	resource_sharing.task_replication_factor == 1 with input as {"node": {"class": "desktop"}}
}

test_replication_factor_unknown_class_defaults_to_1 if {
	resource_sharing.task_replication_factor == 1 with input as {"node": {"class": "unknown-class"}}
}

test_replication_factor_missing_class_defaults_to_1 if {
	resource_sharing.task_replication_factor == 1 with input as {"node": {}}
}

# ==========================================================================
# mobile_eligible_task
# ==========================================================================

test_mobile_eligible_task_valid if {
	resource_sharing.mobile_eligible_task with input as {
		"task": {
			"segment_duration_seconds": 60,
			"wasm_cid": "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
		},
	}
}

test_mobile_eligible_task_min_segment if {
	resource_sharing.mobile_eligible_task with input as {
		"task": {
			"segment_duration_seconds": 1,
			"wasm_cid": "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
		},
	}
}

test_mobile_eligible_task_max_segment if {
	resource_sharing.mobile_eligible_task with input as {
		"task": {
			"segment_duration_seconds": 120,
			"wasm_cid": "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
		},
	}
}

test_mobile_eligible_task_denied_segment_too_long if {
	not resource_sharing.mobile_eligible_task with input as {
		"task": {
			"segment_duration_seconds": 121,
			"wasm_cid": "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
		},
	}
}

test_mobile_eligible_task_denied_zero_segment if {
	not resource_sharing.mobile_eligible_task with input as {
		"task": {
			"segment_duration_seconds": 0,
			"wasm_cid": "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
		},
	}
}

test_mobile_eligible_task_denied_missing_wasm_cid if {
	not resource_sharing.mobile_eligible_task with input as {
		"task": {
			"segment_duration_seconds": 60,
			"wasm_cid": "",
		},
	}
}

# ==========================================================================
# android_tv_eligible_task
# ==========================================================================

test_android_tv_eligible_task_valid if {
	resource_sharing.android_tv_eligible_task with input as {
		"task": {"wasm_cid": "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi"},
	}
}

test_android_tv_eligible_task_denied_missing_wasm_cid if {
	not resource_sharing.android_tv_eligible_task with input as {
		"task": {"wasm_cid": ""},
	}
}

# ==========================================================================
# allow_mobile_task
# ==========================================================================

test_allow_mobile_task_allowed if {
	resource_sharing.allow_mobile_task with input as {
		"action": "mobile_task_submit",
		"user_did": "did:example:user1",
		"authenticated": true,
		"node": {"class": "mobile-android"},
		"task": {
			"segment_duration_seconds": 60,
			"wasm_cid": "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
			"cpu_cores": 1,
			"memory_mb": 2048,
			"timeout_seconds": 120,
			"disk_mb": 1024,
		},
	}
}

test_allow_mobile_task_allowed_android_tv if {
	resource_sharing.allow_mobile_task with input as {
		"action": "mobile_task_submit",
		"user_did": "did:example:user1",
		"authenticated": true,
		"node": {"class": "android-tv"},
		"task": {
			"segment_duration_seconds": 60,
			"wasm_cid": "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
			"cpu_cores": 1,
			"memory_mb": 2048,
			"timeout_seconds": 120,
			"disk_mb": 1024,
		},
	}
}

test_allow_mobile_task_denied_missing_did if {
	not resource_sharing.allow_mobile_task with input as {
		"action": "mobile_task_submit",
		"user_did": "",
		"authenticated": true,
		"node": {"class": "mobile-android"},
		"task": {
			"segment_duration_seconds": 60,
			"wasm_cid": "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
			"cpu_cores": 1,
			"memory_mb": 2048,
			"timeout_seconds": 120,
			"disk_mb": 1024,
		},
	}
}

test_allow_mobile_task_denied_empty_node_class if {
	not resource_sharing.allow_mobile_task with input as {
		"action": "mobile_task_submit",
		"user_did": "did:example:user1",
		"authenticated": true,
		"node": {"class": ""},
		"task": {
			"segment_duration_seconds": 60,
			"wasm_cid": "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
			"cpu_cores": 1,
			"memory_mb": 2048,
			"timeout_seconds": 120,
			"disk_mb": 1024,
		},
	}
}

test_allow_mobile_task_denied_not_eligible if {
	not resource_sharing.allow_mobile_task with input as {
		"action": "mobile_task_submit",
		"user_did": "did:example:user1",
		"authenticated": true,
		"node": {"class": "mobile-android"},
		"task": {
			"segment_duration_seconds": 200,
			"wasm_cid": "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
			"cpu_cores": 1,
			"memory_mb": 2048,
			"timeout_seconds": 120,
			"disk_mb": 1024,
		},
	}
}

test_allow_mobile_task_denied_resource_over_limit if {
	not resource_sharing.allow_mobile_task with input as {
		"action": "mobile_task_submit",
		"user_did": "did:example:user1",
		"authenticated": true,
		"node": {"class": "mobile-android"},
		"task": {
			"segment_duration_seconds": 60,
			"wasm_cid": "bafybeigdyrzt5sfp7udm7hu76uh7y26nf3efuylqabf3oclgtqy55fbzdi",
			"cpu_cores": 8,
			"memory_mb": 2048,
			"timeout_seconds": 120,
			"disk_mb": 1024,
		},
	}
}

# ==========================================================================
# allow_htlc_cancel
# ==========================================================================

test_htlc_cancel_verification_failed if {
	resource_sharing.allow_htlc_cancel with input as {
		"action": "htlc_cancel",
		"coordinator_did": "did:example:coordinator",
		"cancel_reason": "verification_failed",
	}
}

test_htlc_cancel_node_timeout if {
	resource_sharing.allow_htlc_cancel with input as {
		"action": "htlc_cancel",
		"coordinator_did": "did:example:coordinator",
		"cancel_reason": "node_timeout",
	}
}

test_htlc_cancel_node_disconnected if {
	resource_sharing.allow_htlc_cancel with input as {
		"action": "htlc_cancel",
		"coordinator_did": "did:example:coordinator",
		"cancel_reason": "node_disconnected",
	}
}

test_htlc_cancel_denied_invalid_reason if {
	not resource_sharing.allow_htlc_cancel with input as {
		"action": "htlc_cancel",
		"coordinator_did": "did:example:coordinator",
		"cancel_reason": "user_request",
	}
}

test_htlc_cancel_denied_missing_coordinator_did if {
	not resource_sharing.allow_htlc_cancel with input as {
		"action": "htlc_cancel",
		"coordinator_did": "",
		"cancel_reason": "verification_failed",
	}
}

test_htlc_cancel_denied_missing_reason if {
	not resource_sharing.allow_htlc_cancel with input as {
		"action": "htlc_cancel",
		"coordinator_did": "did:example:coordinator",
		"cancel_reason": "",
	}
}

test_htlc_cancel_denied_wrong_action if {
	not resource_sharing.allow_htlc_cancel with input as {
		"action": "htlc_settle",
		"coordinator_did": "did:example:coordinator",
		"cancel_reason": "verification_failed",
	}
}

# ==========================================================================
# allow_htlc_settle
# ==========================================================================

test_htlc_settle_allowed if {
	resource_sharing.allow_htlc_settle with input as {
		"action": "htlc_settle",
		"coordinator_did": "did:example:coordinator",
		"shadow_verified": true,
	}
}

test_htlc_settle_denied_shadow_not_verified if {
	not resource_sharing.allow_htlc_settle with input as {
		"action": "htlc_settle",
		"coordinator_did": "did:example:coordinator",
		"shadow_verified": false,
	}
}

test_htlc_settle_denied_missing_coordinator_did if {
	not resource_sharing.allow_htlc_settle with input as {
		"action": "htlc_settle",
		"coordinator_did": "",
		"shadow_verified": true,
	}
}

test_htlc_settle_denied_wrong_action if {
	not resource_sharing.allow_htlc_settle with input as {
		"action": "htlc_cancel",
		"coordinator_did": "did:example:coordinator",
		"shadow_verified": true,
	}
}

# ==========================================================================
# allow_mobile_preempt
# ==========================================================================

test_mobile_preempt_node_disconnected if {
	resource_sharing.allow_mobile_preempt with input as {
		"action": "mobile_preempt",
		"coordinator_did": "did:example:coordinator",
		"preempt_reason": "node_disconnected",
	}
}

test_mobile_preempt_node_timeout if {
	resource_sharing.allow_mobile_preempt with input as {
		"action": "mobile_preempt",
		"coordinator_did": "did:example:coordinator",
		"preempt_reason": "node_timeout",
	}
}

test_mobile_preempt_thermal_throttle if {
	resource_sharing.allow_mobile_preempt with input as {
		"action": "mobile_preempt",
		"coordinator_did": "did:example:coordinator",
		"preempt_reason": "thermal_throttle",
	}
}

test_mobile_preempt_denied_invalid_reason if {
	not resource_sharing.allow_mobile_preempt with input as {
		"action": "mobile_preempt",
		"coordinator_did": "did:example:coordinator",
		"preempt_reason": "user_request",
	}
}

test_mobile_preempt_denied_missing_coordinator_did if {
	not resource_sharing.allow_mobile_preempt with input as {
		"action": "mobile_preempt",
		"coordinator_did": "",
		"preempt_reason": "node_disconnected",
	}
}

test_mobile_preempt_denied_missing_reason if {
	not resource_sharing.allow_mobile_preempt with input as {
		"action": "mobile_preempt",
		"coordinator_did": "did:example:coordinator",
		"preempt_reason": "",
	}
}

test_mobile_preempt_denied_wrong_action if {
	not resource_sharing.allow_mobile_preempt with input as {
		"action": "htlc_cancel",
		"coordinator_did": "did:example:coordinator",
		"preempt_reason": "node_disconnected",
	}
}

# ==========================================================================
# deny_reasons
# ==========================================================================

test_deny_reasons_no_did if {
	resource_sharing.deny_reasons["no_user_did"] with input as {
		"user_did": "",
		"authenticated": true,
	}
}

test_deny_reasons_not_authenticated if {
	resource_sharing.deny_reasons["not_authenticated"] with input as {
		"user_did": "did:example:user1",
		"authenticated": false,
	}
}

test_deny_reasons_both_if_unauthenticated_no_did if {
	resource_sharing.deny_reasons["no_user_did"] with input as {
		"user_did": "",
		"authenticated": false,
	}
	resource_sharing.deny_reasons["not_authenticated"] with input as {
		"user_did": "",
		"authenticated": false,
	}
}
