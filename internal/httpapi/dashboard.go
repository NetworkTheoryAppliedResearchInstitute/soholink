package httpapi

import (
	"encoding/json"
	"net/http"
	"runtime"
	"time"
)

// nodeStartTime is set once at process start and used to report uptime.
var nodeStartTime = time.Now()

// handleStatus returns a JSON snapshot of node health and resource usage.
// Consumed by the FlutterFlow frontend via GET /api/status.
// Only GET is accepted.
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	uptime := time.Since(nodeStartTime)

	// Active rentals — use existing store method if available.
	activeRentals := 0
	if s.store != nil {
		if rentals, err := s.store.GetActiveRentals(ctx); err == nil {
			activeRentals = len(rentals)
		}
	}

	// Federation nodes — mobile hub gives connected mobile count;
	// p2p mesh gives LAN-discovered peer count.
	federationNodes := 0
	mobileNodes := 0
	if s.mobileHub != nil {
		mobileNodes = len(s.mobileHub.ActiveNodes())
		federationNodes = mobileNodes
	}
	if s.p2pMesh != nil {
		federationNodes += s.p2pMesh.PeerCount()
	}

	// Earnings today — revenue events since midnight UTC.
	earnedSatsToday := int64(0)
	if s.store != nil {
		midnight := time.Now().UTC().Truncate(24 * time.Hour)
		if earned, err := s.store.GetRevenueSince(ctx, midnight); err == nil {
			earnedSatsToday = earned
		}
	}

	// Resource utilisation fields remain zero until gopsutil readings are wired.
	// The frontend handles zero gracefully.
	type statusResponse struct {
		UptimeSeconds    int64   `json:"uptime_seconds"`
		OS               string  `json:"os"`
		ActiveRentals    int     `json:"active_rentals"`
		FederationNodes  int     `json:"federation_nodes"`
		MobileNodes      int     `json:"mobile_nodes"`
		EarnedSatsToday  int64   `json:"earned_sats_today"`
		CPUOfferedPct    int     `json:"cpu_offered_pct"`
		CPUUsedPct       float64 `json:"cpu_used_pct"`
		RAMOfferedGB     float64 `json:"ram_offered_gb"`
		RAMUsedPct       float64 `json:"ram_used_pct"`
		StorageOfferedGB float64 `json:"storage_offered_gb"`
		StorageUsedPct   float64 `json:"storage_used_pct"`
		NetOfferedMbps   int     `json:"net_offered_mbps"`
		NetUsedPct       float64 `json:"net_used_pct"`
		BtcUsdRate       float64 `json:"btc_usd_rate"`
	}

	resp := statusResponse{
		UptimeSeconds:   int64(uptime.Seconds()),
		OS:              runtime.GOOS + "/" + runtime.GOARCH,
		ActiveRentals:   activeRentals,
		FederationNodes: federationNodes,
		MobileNodes:     mobileNodes,
		EarnedSatsToday: earnedSatsToday,
		CPUOfferedPct:   50,
		BtcUsdRate:      GetBtcUsdRate(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp) // #nosec G104 -- response write errors are non-actionable
}
