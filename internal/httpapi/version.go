package httpapi

import (
	"encoding/json"
	"net/http"
)

// SetVersionInfo stores the build-time version metadata on the server for the
// GET /api/version endpoint.  Call this before Start().
func (s *Server) SetVersionInfo(version, commit, buildTime string) {
	s.version = version
	s.commit = commit
	s.buildTime = buildTime
}

// handleVersion serves GET /api/version.
// This endpoint is public (no authentication required — see publicPaths in
// auth_middleware.go) so that external tooling and the auto-updater can
// determine the running version without credentials.
//
// Response JSON:
//
//	{
//	  "version":    "0.1.0",
//	  "commit":     "490e7fa",
//	  "build_time": "2026-03-06"
//	}
func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"version":    s.version,
		"commit":     s.commit,
		"build_time": s.buildTime,
	})
}
