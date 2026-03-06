package p2p

import (
	"encoding/json"
	"net/http"
	"time"
)

// PeerJSON is the JSON representation of a peer for the /api/peers endpoint.
type PeerJSON struct {
	DID      string    `json:"did"`
	APIAddr  string    `json:"api_addr"`
	IPFSAddr string    `json:"ipfs_addr,omitempty"`
	CPU      float64   `json:"cpu_cores"`
	RAMGB    float64   `json:"ram_gb"`
	DiskGB   int64     `json:"disk_gb"`
	GPU      string    `json:"gpu,omitempty"`
	Region   string    `json:"region,omitempty"`
	LastSeen time.Time `json:"last_seen"`
}

// HandlePeers returns an http.HandlerFunc that serves GET /api/peers.
// It lists all currently live LAN-discovered peers from the mesh.
func (m *Mesh) HandlePeers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	peers := m.Peers()
	out := make([]PeerJSON, 0, len(peers))
	for _, p := range peers {
		out = append(out, PeerJSON{
			DID:      p.DID,
			APIAddr:  p.APIAddr,
			IPFSAddr: p.IPFSAddr,
			CPU:      p.CPU,
			RAMGB:    p.RAMGB,
			DiskGB:   p.DiskGB,
			GPU:      p.GPU,
			Region:   p.Region,
			LastSeen: p.LastSeen,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{ // #nosec G104
		"count": len(out),
		"peers": out,
	})
}
