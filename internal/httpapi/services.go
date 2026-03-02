package httpapi

import (
	"encoding/json"
	"strconv"
	"time"
	"fmt"
	"net/http"
	"strings"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/services"
)

// ServiceManager interface for managing services
type ServiceManager interface {
	ProvisionService(serviceType services.ServiceType, req services.ProvisionRequest) (*services.ServiceInstance, error)
	DeprovisionService(instanceID string) error
	GetService(instanceID string) (*services.ServiceInstance, error)
	ListServices() []*services.ServiceInstance
	GetServiceMetrics(instanceID string) (*services.ServiceMetrics, error)
	GetServiceLogs(instanceID string, tailLines int) (string, error)
}

// SetServiceManager sets the service manager for the API server
func (s *Server) SetServiceManager(sm ServiceManager) {
	s.serviceManager = sm
}

// handleProvisionService creates a new managed service instance
// POST /api/services
func (s *Server) handleProvisionService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ServiceType string                 `json:"service_type"`
		PlanID      string                 `json:"plan_id"`
		Config      map[string]string      `json:"config"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.ServiceType == "" {
		http.Error(w, "service_type required", http.StatusBadRequest)
		return
	}

	if req.PlanID == "" {
		http.Error(w, "plan_id required", http.StatusBadRequest)
		return
	}

	serviceType := services.ServiceType(req.ServiceType)

	// Validate service type
	validTypes := []services.ServiceType{
		services.ServiceTypePostgres,
		services.ServiceTypeMySQL,
		services.ServiceTypeMongoDB,
		services.ServiceTypeRedis,
		services.ServiceTypeObjectStore,
		services.ServiceTypeMessageQueue,
	}

	valid := false
	for _, t := range validTypes {
		if serviceType == t {
			valid = true
			break
		}
	}

	if !valid {
		http.Error(w, "Invalid service_type", http.StatusBadRequest)
		return
	}

	provisionReq := services.ProvisionRequest{
		Name: fmt.Sprintf("%s-%d", req.ServiceType, time.Now().Unix()),
		Plan:   req.PlanID,
		Config:     req.Config,
	}

	instance, err := s.serviceManager.ProvisionService(serviceType, provisionReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Provisioning failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(instance)
}

// handleListServices returns all service instances
// GET /api/services
func (s *Server) handleListServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	instances := s.serviceManager.ListServices()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"services": instances,
		"count":    len(instances),
	})
}

// handleGetService returns a specific service instance
// GET /api/services/{id}
func (s *Server) handleGetService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/services/")
	instanceID := strings.Split(path, "/")[0]

	if instanceID == "" {
		http.Error(w, "Instance ID required", http.StatusBadRequest)
		return
	}

	instance, err := s.serviceManager.GetService(instanceID)
	if err != nil {
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	// Include connection string in response
	response := map[string]interface{}{
		"instance_id":       instance.InstanceID,
		"service_type":      instance.ServiceType,
		"status":            instance.Status,
		"endpoint":          instance.Endpoint,
		"connection_string": instance.Credentials.Token,
		"credentials":       instance.Credentials,
		"config":            instance.Config,
		"created_at":        instance.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleDeleteService deprovisions a service instance
// DELETE /api/services/{id}
func (s *Server) handleDeleteService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/services/")
	instanceID := strings.Split(path, "/")[0]

	if instanceID == "" {
		http.Error(w, "Instance ID required", http.StatusBadRequest)
		return
	}

	if err := s.serviceManager.DeprovisionService(instanceID); err != nil {
		http.Error(w, fmt.Sprintf("Deprovisioning failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleGetServiceMetrics returns metrics for a service instance
// GET /api/services/{id}/metrics
func (s *Server) handleGetServiceMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/services/")
	instanceID := strings.TrimSuffix(path, "/metrics")

	if instanceID == "" || instanceID == path {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	metrics, err := s.serviceManager.GetServiceMetrics(instanceID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get metrics: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

// handleGetServiceLogs returns logs for a service instance
// GET /api/services/{id}/logs
func (s *Server) handleGetServiceLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/services/")
	instanceID := strings.TrimSuffix(path, "/logs")

	if instanceID == "" || instanceID == path {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	tailLines := 100
	if tail := r.URL.Query().Get("tail"); tail != "" {
		if n, err := strconv.Atoi(tail); err == nil && n > 0 {
			tailLines = n
		}
	}

	logs, err := s.serviceManager.GetServiceLogs(instanceID, tailLines)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get logs: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(logs))
}

// handleRestartService restarts a service instance
// POST /api/services/{id}/restart
func (s *Server) handleRestartService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/services/")
	instanceID := strings.TrimSuffix(path, "/restart")

	if instanceID == "" || instanceID == path {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// Get the service
	instance, err := s.serviceManager.GetService(instanceID)
	if err != nil {
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	// Restart = deprovision + reprovision with same config
	if err := s.serviceManager.DeprovisionService(instanceID); err != nil {
		http.Error(w, fmt.Sprintf("Failed to stop service: %v", err), http.StatusInternalServerError)
		return
	}

	provisionReq := services.ProvisionRequest{
		Name: instance.Name,
		Plan: instance.Plan,
		Config:     instance.Config,
	}

	newInstance, err := s.serviceManager.ProvisionService(instance.ServiceType, provisionReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to restart service: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"instance_id": newInstance.InstanceID,
		"status":      "restarted",
	})
}

// Service router - routes /api/services/* paths to appropriate handlers
func (s *Server) routeService(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/services")

	// List or create services: /api/services
	if path == "" || path == "/" {
		if r.Method == http.MethodGet {
			s.handleListServices(w, r)
			return
		}
		if r.Method == http.MethodPost {
			s.handleProvisionService(w, r)
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

	// Handle sub-resources
	if len(parts) > 1 {
		action := parts[1]
		switch action {
		case "metrics":
			s.handleGetServiceMetrics(w, r)
		case "logs":
			s.handleGetServiceLogs(w, r)
		case "restart":
			s.handleRestartService(w, r)
		default:
			http.Error(w, "Unknown action", http.StatusNotFound)
		}
		return
	}

	// Handle service CRUD
	switch r.Method {
	case http.MethodGet:
		s.handleGetService(w, r)
	case http.MethodDelete:
		s.handleDeleteService(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
