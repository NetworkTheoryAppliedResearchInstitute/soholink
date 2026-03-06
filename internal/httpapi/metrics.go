package httpapi

import (
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ---------------------------------------------------------------------------
// Live Prometheus counters (registered at init via promauto)
// ---------------------------------------------------------------------------

// httpRequestsTotal counts every HTTP request handled by the API server,
// labelled by HTTP method, URL path, and response status code.
// NOTE: r.URL.Path is used as-is; paths containing resource IDs (e.g.
// /api/lbtas/score/did:key:...) produce high-cardinality labels.
// Normalise or truncate such paths before a high-traffic production deployment.
var httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "soholink_http_requests_total",
	Help: "Total HTTP requests handled by the API server, by method, path, and status code.",
}, []string{"method", "path", "status"})

// walletTopupTotal counts successfully initiated wallet topup requests.
// A topup is counted when the ledger returns a payment invoice without error;
// it does NOT indicate that the payment was completed.
var walletTopupTotal = promauto.NewCounter(prometheus.CounterOpts{
	Name: "soholink_wallet_topup_total",
	Help: "Total wallet topup requests successfully initiated.",
})

// workloadPurchaseTotal counts workload purchase attempts, labelled by result:
// "success", "manifest_rejected", "policy_denied", "payment_failed".
var workloadPurchaseTotal = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "soholink_workload_purchase_total",
	Help: "Workload purchase attempts by result label.",
}, []string{"result"})

// ---------------------------------------------------------------------------
// HTTP status-recording middleware
// ---------------------------------------------------------------------------

// statusRecorder is a minimal http.ResponseWriter wrapper that captures the
// HTTP status code written by the downstream handler, used by metricsMiddleware.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

// metricsMiddleware wraps next and increments httpRequestsTotal for every
// request after the downstream handler returns.
func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rw := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		httpRequestsTotal.WithLabelValues(r.Method, r.URL.Path, strconv.Itoa(rw.status)).Inc()
	})
}

// ---------------------------------------------------------------------------
// Metric name catalogue (kept for reference / dashboard generation)
// ---------------------------------------------------------------------------

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
	WalletTopupTotal        string
	WorkloadPurchaseTotal   string
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
	HTTPRequestsTotal:       "soholink_http_requests_total",
	HTTPRequestDuration:     "http_request_duration_seconds",
	HTTPActiveRequests:      "http_active_requests",
	WalletTopupTotal:        "soholink_wallet_topup_total",
	WorkloadPurchaseTotal:   "soholink_workload_purchase_total",
}
