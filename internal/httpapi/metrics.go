package httpapi

// Metrics defines Prometheus metric names for resource sharing subsystems.
// These are registered with the Prometheus client when the metrics endpoint is enabled.
//
// Resource usage metrics:
//   compute_jobs_total          (counter, labels: status, provider)
//   compute_queue_size          (gauge)
//   compute_cpu_seconds_total   (counter, labels: provider)
//   storage_uploads_total       (counter, labels: mime_type, scanner_result)
//   storage_bytes_stored        (gauge)
//   storage_malware_blocked     (counter)
//   print_jobs_total            (counter, labels: printer_type, status)
//   print_filament_grams_total  (counter, labels: printer_type)
//   portal_sessions_active      (gauge)
//   portal_auth_failures_total  (counter, labels: reason)
//   portal_bandwidth_bytes      (counter, labels: direction)
//
// Payment metrics:
//   payments_pending            (gauge)
//   payments_settled_total      (counter, labels: processor, currency)
//   payments_failed_total       (counter, labels: processor, reason)
//
// LBTAS metrics:
//   lbtas_ratings_total         (counter, labels: resource_type, rater_role)
//   lbtas_disputes_total        (counter)
//   lbtas_auto_resolved_total   (counter)
//
// HTTP API metrics:
//   http_requests_total         (counter, labels: method, path, status)
//   http_request_duration_seconds (histogram, labels: method, path)
//   http_active_requests        (gauge)

// MetricNames holds the full set of metric names used by the system.
var MetricNames = struct {
	ComputeJobsTotal        string
	ComputeQueueSize        string
	ComputeCPUSecondsTotal  string
	StorageUploadsTotal     string
	StorageBytesStored      string
	StorageMalwareBlocked   string
	PrintJobsTotal          string
	PrintFilamentGramsTotal string
	PortalSessionsActive    string
	PortalAuthFailuresTotal string
	PortalBandwidthBytes    string
	PaymentsPending         string
	PaymentsSettledTotal    string
	PaymentsFailedTotal     string
	LBTASRatingsTotal       string
	LBTASDisputesTotal      string
	LBTASAutoResolvedTotal  string
	HTTPRequestsTotal       string
	HTTPRequestDuration     string
	HTTPActiveRequests      string
}{
	ComputeJobsTotal:        "compute_jobs_total",
	ComputeQueueSize:        "compute_queue_size",
	ComputeCPUSecondsTotal:  "compute_cpu_seconds_total",
	StorageUploadsTotal:     "storage_uploads_total",
	StorageBytesStored:      "storage_bytes_stored",
	StorageMalwareBlocked:   "storage_malware_blocked",
	PrintJobsTotal:          "print_jobs_total",
	PrintFilamentGramsTotal: "print_filament_grams_total",
	PortalSessionsActive:    "portal_sessions_active",
	PortalAuthFailuresTotal: "portal_auth_failures_total",
	PortalBandwidthBytes:    "portal_bandwidth_bytes",
	PaymentsPending:         "payments_pending",
	PaymentsSettledTotal:    "payments_settled_total",
	PaymentsFailedTotal:     "payments_failed_total",
	LBTASRatingsTotal:       "lbtas_ratings_total",
	LBTASDisputesTotal:      "lbtas_disputes_total",
	LBTASAutoResolvedTotal:  "lbtas_auto_resolved_total",
	HTTPRequestsTotal:       "http_requests_total",
	HTTPRequestDuration:     "http_request_duration_seconds",
	HTTPActiveRequests:      "http_active_requests",
}
