package httpapi

import (
	"encoding/json"
	"net/http"
	"time"
)

// ---------------------------------------------------------------------------
// GET /api/revenue — flat endpoint consumed by the Flutter mobile app.
// ---------------------------------------------------------------------------

// handleMobileRevenue serves GET /api/revenue.
// It aggregates data from the existing store methods into the single flat
// JSON shape that the Flutter client expects.
func (s *Server) handleMobileRevenue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	var (
		total   int64
		today   int64
		last7d  int64
		last30d int64
	)

	if s.store != nil {
		now := time.Now().UTC()
		midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		total, _ = s.store.GetTotalRevenue(ctx)
		today, _ = s.store.GetRevenueSince(ctx, midnight)
		last7d, _ = s.store.GetRevenueSince(ctx, now.Add(-7*24*time.Hour))
		last30d, _ = s.store.GetRevenueSince(ctx, now.Add(-30*24*time.Hour))
	}

	const feePct = 1.0
	netToday := int64(float64(today) * (1.0 - feePct/100.0))

	// Daily 30-day history for the bar chart.
	type histItem struct {
		Date string `json:"date"`
		Sats int64  `json:"sats"`
	}
	hist := make([]histItem, 0)
	if s.store != nil {
		if daily, err := s.store.GetDailyRevenueLast30Days(ctx); err == nil {
			for _, d := range daily {
				hist = append(hist, histItem{Date: d.Date, Sats: d.Sats})
			}
		}
	}

	btcRate := GetBtcUsdRate()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{ // #nosec G104
		"earned_sats_total": total,
		"earned_sats_today": today,
		"earned_sats_7d":    last7d,
		"earned_sats_30d":   last30d,
		"fee_pct":           feePct,
		"net_sats_today":    netToday,
		"btc_usd_rate":      btcRate,
		"history":           hist,
	})
}

// ---------------------------------------------------------------------------
// GET /api/workloads — Flutter-compatible list endpoint.
//
// The existing handleListWorkloads (workloads.go) emits raw WorkloadState
// structs.  This wrapper transforms each state into the flat shape the
// Flutter Workload model expects, and injects btc_usd_rate.
// ---------------------------------------------------------------------------

// handleMobileWorkloads serves GET /api/workloads for the mobile dashboard.
// It replaces the scheduler-oriented response with a Flutter-friendly shape.
func (s *Server) handleMobileWorkloads(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	type workloadItem struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		TenantDID   string `json:"tenant_did"`
		Status      string `json:"status"`
		CPUMillis   int    `json:"cpu_millis"`
		RAMMB       int64  `json:"ram_mb"`
		StorageMB   int64  `json:"storage_mb"`
		StartedUnix int64  `json:"started_unix"`
		EarnedSats  int64  `json:"earned_sats"`
	}

	items := make([]workloadItem, 0)

	if s.scheduler != nil {
		for _, ws := range s.scheduler.ListWorkloads() {
			if ws == nil || ws.Workload == nil {
				continue
			}
			w2 := ws.Workload
			items = append(items, workloadItem{
				ID:          w2.WorkloadID,
				Name:        w2.Name,
				TenantDID:   w2.OwnerDID,
				Status:      w2.Status,
				CPUMillis:   int(w2.Spec.CPUCores * 1000),
				RAMMB:       w2.Spec.MemoryMB,
				StorageMB:   w2.Spec.DiskGB * 1024,
				StartedUnix: w2.CreatedAt.Unix(),
				EarnedSats:  0, // per-workload earnings require metering integration
			})
		}
	}

	btcRate := GetBtcUsdRate()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{ // #nosec G104
		"count":        len(items),
		"btc_usd_rate": btcRate,
		"workloads":    items,
	})
}
