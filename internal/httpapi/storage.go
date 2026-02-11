package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// StorageBackend interface for object storage operations
type StorageBackend interface {
	CreateBucket(name string) error
	DeleteBucket(name string) error
	ListBuckets() ([]string, error)
	PutObject(bucket, key string, data io.Reader, size int64) error
	GetObject(bucket, key string) (io.ReadCloser, error)
	DeleteObject(bucket, key string) error
	ListObjects(bucket, prefix string) ([]ObjectInfo, error)
	GetObjectMetadata(bucket, key string) (*ObjectMetadata, error)
}

// ObjectInfo represents object metadata
type ObjectInfo struct {
	Key          string    `json:"key"`
	Size         int64     `json:"size"`
	LastModified time.Time `json:"last_modified"`
	ETag         string    `json:"etag"`
}

// ObjectMetadata represents detailed object metadata
type ObjectMetadata struct {
	Key          string            `json:"key"`
	Size         int64             `json:"size"`
	ContentType  string            `json:"content_type"`
	LastModified time.Time         `json:"last_modified"`
	ETag         string            `json:"etag"`
	Metadata     map[string]string `json:"metadata"`
}

// SetStorageBackend sets the storage backend for the API server
func (s *Server) SetStorageBackend(sb StorageBackend) {
	s.storageBackend = sb
}

// handleCreateBucket creates a new storage bucket
// POST /api/storage/buckets
func (s *Server) handleCreateBucket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		http.Error(w, "Bucket name required", http.StatusBadRequest)
		return
	}

	// Validate bucket name (S3-compatible naming rules)
	if len(req.Name) < 3 || len(req.Name) > 63 {
		http.Error(w, "Bucket name must be 3-63 characters", http.StatusBadRequest)
		return
	}

	if s.storageBackend == nil {
		http.Error(w, "Storage backend not configured", http.StatusServiceUnavailable)
		return
	}

	if err := s.storageBackend.CreateBucket(req.Name); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create bucket: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"bucket": req.Name,
		"status": "created",
	})
}

// handleListBuckets lists all buckets
// GET /api/storage/buckets
func (s *Server) handleListBuckets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if s.storageBackend == nil {
		http.Error(w, "Storage backend not configured", http.StatusServiceUnavailable)
		return
	}

	buckets, err := s.storageBackend.ListBuckets()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list buckets: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"buckets": buckets,
		"count":   len(buckets),
	})
}

// handleDeleteBucket deletes a bucket
// DELETE /api/storage/buckets/{name}
func (s *Server) handleDeleteBucket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/storage/buckets/")
	bucketName := strings.Split(path, "/")[0]

	if bucketName == "" {
		http.Error(w, "Bucket name required", http.StatusBadRequest)
		return
	}

	if s.storageBackend == nil {
		http.Error(w, "Storage backend not configured", http.StatusServiceUnavailable)
		return
	}

	if err := s.storageBackend.DeleteBucket(bucketName); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete bucket: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handlePutObject uploads an object to a bucket
// PUT /api/storage/objects/{bucket}/{key}
func (s *Server) handlePutObject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/storage/objects/")
	parts := strings.SplitN(path, "/", 2)

	if len(parts) < 2 {
		http.Error(w, "Bucket and key required", http.StatusBadRequest)
		return
	}

	bucket := parts[0]
	key := parts[1]

	if bucket == "" || key == "" {
		http.Error(w, "Bucket and key required", http.StatusBadRequest)
		return
	}

	if s.storageBackend == nil {
		http.Error(w, "Storage backend not configured", http.StatusServiceUnavailable)
		return
	}

	// Upload object
	size := r.ContentLength
	if size < 0 {
		size = 0
	}

	if err := s.storageBackend.PutObject(bucket, key, r.Body, size); err != nil {
		http.Error(w, fmt.Sprintf("Failed to upload object: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"bucket": bucket,
		"key":    key,
		"size":   size,
		"status": "uploaded",
	})
}

// handleGetObject downloads an object from a bucket
// GET /api/storage/objects/{bucket}/{key}
func (s *Server) handleGetObject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/storage/objects/")
	parts := strings.SplitN(path, "/", 2)

	if len(parts) < 2 {
		http.Error(w, "Bucket and key required", http.StatusBadRequest)
		return
	}

	bucket := parts[0]
	key := parts[1]

	if s.storageBackend == nil {
		http.Error(w, "Storage backend not configured", http.StatusServiceUnavailable)
		return
	}

	// Check if this is a metadata request
	if r.URL.Query().Get("metadata") == "true" {
		metadata, err := s.storageBackend.GetObjectMetadata(bucket, key)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get metadata: %v", err), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(metadata)
		return
	}

	// Download object
	reader, err := s.storageBackend.GetObject(bucket, key)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get object: %v", err), http.StatusNotFound)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", key))

	io.Copy(w, reader)
}

// handleDeleteObject deletes an object from a bucket
// DELETE /api/storage/objects/{bucket}/{key}
func (s *Server) handleDeleteObject(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/storage/objects/")
	parts := strings.SplitN(path, "/", 2)

	if len(parts) < 2 {
		http.Error(w, "Bucket and key required", http.StatusBadRequest)
		return
	}

	bucket := parts[0]
	key := parts[1]

	if s.storageBackend == nil {
		http.Error(w, "Storage backend not configured", http.StatusServiceUnavailable)
		return
	}

	if err := s.storageBackend.DeleteObject(bucket, key); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete object: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleListObjects lists objects in a bucket
// GET /api/storage/buckets/{bucket}/objects
func (s *Server) handleListObjects(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/storage/buckets/")
	bucketName := strings.TrimSuffix(path, "/objects")

	if bucketName == "" || bucketName == path {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	prefix := r.URL.Query().Get("prefix")

	if s.storageBackend == nil {
		http.Error(w, "Storage backend not configured", http.StatusServiceUnavailable)
		return
	}

	objects, err := s.storageBackend.ListObjects(bucketName, prefix)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list objects: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"bucket":  bucketName,
		"prefix":  prefix,
		"objects": objects,
		"count":   len(objects),
	})
}
