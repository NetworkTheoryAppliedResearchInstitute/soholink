package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

)

// handleListWorkloads returns all workloads (GET /api/workloads)
func (s *Server) handleListWorkloads(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.scheduler == nil {
		http.Error(w, "Scheduler not available", http.StatusServiceUnavailable)
		return
	}

	workloads := s.scheduler.ListWorkloads()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"workloads": workloads,
		"count":     len(workloads),
	})
}

// handleGetWorkload returns a specific workload (GET /api/workloads/{id})
func (s *Server) handleGetWorkload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract workload ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/workloads/")
	workloadID := strings.Split(path, "/")[0]

	if workloadID == "" {
		http.Error(w, "Workload ID required", http.StatusBadRequest)
		return
	}

	if s.scheduler == nil {
		http.Error(w, "Scheduler not available", http.StatusServiceUnavailable)
		return
	}

	workload := s.scheduler.GetWorkload(workloadID)
	if workload == nil {
		http.Error(w, "Workload not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(workload)
}

// handleScaleWorkload scales a workload (PUT /api/workloads/{id}/scale)
func (s *Server) handleScaleWorkload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract workload ID
	path := strings.TrimPrefix(r.URL.Path, "/api/workloads/")
	workloadID := strings.TrimSuffix(path, "/scale")

	if workloadID == "" || workloadID == path {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	var req struct {
		Replicas int `json:"replicas"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Replicas < 0 {
		http.Error(w, "Replicas must be >= 0", http.StatusBadRequest)
		return
	}

	if s.scheduler == nil {
		http.Error(w, "Scheduler not available", http.StatusServiceUnavailable)
		return
	}

	if err := s.scheduler.ScaleWorkload(r.Context(), workloadID, req.Replicas); err != nil {
		http.Error(w, fmt.Sprintf("Scale failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"workload_id": workloadID,
		"replicas":    req.Replicas,
		"status":      "scaling",
	})
}

// handleDeleteWorkload terminates a workload (DELETE /api/workloads/{id})
func (s *Server) handleDeleteWorkload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract workload ID
	path := strings.TrimPrefix(r.URL.Path, "/api/workloads/")
	workloadID := strings.Split(path, "/")[0]

	if workloadID == "" {
		http.Error(w, "Workload ID required", http.StatusBadRequest)
		return
	}

	if s.scheduler == nil {
		http.Error(w, "Scheduler not available", http.StatusServiceUnavailable)
		return
	}

	if err := s.scheduler.DeleteWorkload(r.Context(), workloadID); err != nil {
		http.Error(w, fmt.Sprintf("Delete failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleUpdateWorkload updates workload configuration (PATCH /api/workloads/{id})
func (s *Server) handleUpdateWorkload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/workloads/")
	workloadID := strings.Split(path, "/")[0]

	if workloadID == "" {
		http.Error(w, "Workload ID required", http.StatusBadRequest)
		return
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if s.scheduler == nil {
		http.Error(w, "Scheduler not available", http.StatusServiceUnavailable)
		return
	}

	// Apply updates
	if err := s.scheduler.UpdateWorkload(r.Context(), workloadID, updates); err != nil {
		http.Error(w, fmt.Sprintf("Update failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"workload_id": workloadID,
		"status":      "updated",
	})
}

// handleRestartWorkload restarts a workload (POST /api/workloads/{id}/restart)
func (s *Server) handleRestartWorkload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/workloads/")
	workloadID := strings.TrimSuffix(path, "/restart")

	if workloadID == "" || workloadID == path {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	if s.scheduler == nil {
		http.Error(w, "Scheduler not available", http.StatusServiceUnavailable)
		return
	}

	if err := s.scheduler.RestartWorkload(r.Context(), workloadID); err != nil {
		http.Error(w, fmt.Sprintf("Restart failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"workload_id": workloadID,
		"status":      "restarting",
	})
}

// handleGetWorkloadLogs returns logs for a workload (GET /api/workloads/{id}/logs)
func (s *Server) handleGetWorkloadLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/workloads/")
	workloadID := strings.TrimSuffix(path, "/logs")

	if workloadID == "" || workloadID == path {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// Parse query parameters
	tailLines := 100
	if tail := r.URL.Query().Get("tail"); tail != "" {
		if n, err := strconv.Atoi(tail); err == nil && n > 0 {
			tailLines = n
		}
	}

	follow := r.URL.Query().Get("follow") == "true"

	if s.scheduler == nil {
		http.Error(w, "Scheduler not available", http.StatusServiceUnavailable)
		return
	}

	logs, err := s.scheduler.GetWorkloadLogs(r.Context(), workloadID, tailLines, follow)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get logs: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(logs))
}

// handleGetWorkloadMetrics returns metrics for a workload (GET /api/workloads/{id}/metrics)
func (s *Server) handleGetWorkloadMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/workloads/")
	workloadID := strings.TrimSuffix(path, "/metrics")

	if workloadID == "" || workloadID == path {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	if s.scheduler == nil {
		http.Error(w, "Scheduler not available", http.StatusServiceUnavailable)
		return
	}

	metrics, err := s.scheduler.GetWorkloadMetrics(r.Context(), workloadID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get metrics: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

// handleGetWorkloadEvents returns events for a workload (GET /api/workloads/{id}/events)
func (s *Server) handleGetWorkloadEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/workloads/")
	workloadID := strings.TrimSuffix(path, "/events")

	if workloadID == "" || workloadID == path {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	if s.scheduler == nil {
		http.Error(w, "Scheduler not available", http.StatusServiceUnavailable)
		return
	}

	events, err := s.scheduler.GetWorkloadEvents(r.Context(), workloadID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get events: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"workload_id": workloadID,
		"events":      events,
	})
}

// Workload router - routes /api/workloads/* paths to appropriate handlers
func (s *Server) routeWorkload(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/workloads")

	// List workloads: GET /api/workloads
	if path == "" || path == "/" {
		if r.Method == http.MethodGet {
			s.handleMobileWorkloads(w, r)
			return
		}
		if r.Method == http.MethodPost {
			s.handleSubmitWorkload(w, r)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Remove leading slash
	path = strings.TrimPrefix(path, "/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	_ = parts[0]

	// Handle sub-resources
	if len(parts) > 1 {
		action := parts[1]
		switch action {
		case "scale":
			s.handleScaleWorkload(w, r)
		case "restart":
			s.handleRestartWorkload(w, r)
		case "logs":
			s.handleGetWorkloadLogs(w, r)
		case "metrics":
			s.handleGetWorkloadMetrics(w, r)
		case "events":
			s.handleGetWorkloadEvents(w, r)
		case "status":
			s.handleWorkloadStatus(w, r)
		default:
			http.Error(w, "Unknown action", http.StatusNotFound)
		}
		return
	}

	// Handle workload CRUD
	switch r.Method {
	case http.MethodGet:
		s.handleGetWorkload(w, r)
	case http.MethodPatch:
		s.handleUpdateWorkload(w, r)
	case http.MethodDelete:
		s.handleDeleteWorkload(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
