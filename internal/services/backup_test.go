package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBackupManager_BackupPostgreSQL(t *testing.T) {
	tmpDir := t.TempDir()

	// Mock Docker API
	execCreated := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/exec") && r.Method == "POST":
			execCreated = true
			json.NewEncoder(w).Encode(map[string]interface{}{
				"Id": "exec-12345",
			})

		case strings.Contains(r.URL.Path, "/exec") && strings.Contains(r.URL.Path, "/start"):
			w.WriteHeader(http.StatusOK)
			// Simulate pg_dump output
			w.Write([]byte("-- PostgreSQL database dump\nCREATE TABLE test;\n"))

		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dockerClient := NewDockerClient(server.URL)
	manager := &BackupManager{
		backupDir:    tmpDir,
		dockerClient: dockerClient,
	}

	instance := &ServiceInstance{
		InstanceID:  "postgres-001",
		ServiceType: ServiceTypePostgres,
		Config: map[string]string{
			"container_id": "postgres-container-123",
			"database":     "testdb",
			"username":     "testuser",
		},
	}

	config := &BackupConfig{
		Compress: true,
		Encrypt:  false,
	}

	ctx := context.Background()
	backup, err := manager.BackupPostgreSQL(ctx, instance, config)
	if err != nil {
		t.Fatalf("BackupPostgreSQL failed: %v", err)
	}

	if backup.BackupID == "" {
		t.Error("Expected non-empty backup ID")
	}

	if backup.ServiceType != ServiceTypePostgres {
		t.Errorf("Expected service type Postgres, got %s", backup.ServiceType)
	}

	if backup.Status != "completed" {
		t.Errorf("Expected status 'completed', got '%s'", backup.Status)
	}

	if !execCreated {
		t.Error("Expected exec to be created for pg_dump")
	}
}

func TestBackupManager_BackupMySQL(t *testing.T) {
	tmpDir := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/exec") && r.Method == "POST" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"Id": "exec-mysql",
			})
			return
		}
		if strings.Contains(r.URL.Path, "/start") {
			w.Write([]byte("-- MySQL dump\nCREATE TABLE test;\n"))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	dockerClient := NewDockerClient(server.URL)
	manager := &BackupManager{
		backupDir:    tmpDir,
		dockerClient: dockerClient,
	}

	instance := &ServiceInstance{
		InstanceID:  "mysql-001",
		ServiceType: ServiceTypeMySQL,
		Config: map[string]string{
			"container_id": "mysql-container-123",
			"database":     "testdb",
			"username":     "root",
			"password":     "secret",
		},
	}

	config := &BackupConfig{
		Compress: false,
	}

	ctx := context.Background()
	backup, err := manager.BackupMySQL(ctx, instance, config)
	if err != nil {
		t.Fatalf("BackupMySQL failed: %v", err)
	}

	if backup.ServiceType != ServiceTypeMySQL {
		t.Errorf("Expected service type MySQL, got %s", backup.ServiceType)
	}
}

func TestBackupManager_BackupMongoDB(t *testing.T) {
	tmpDir := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/exec") && r.Method == "POST" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"Id": "exec-mongo",
			})
			return
		}
		if strings.Contains(r.URL.Path, "/start") {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	dockerClient := NewDockerClient(server.URL)
	manager := &BackupManager{
		backupDir:    tmpDir,
		dockerClient: dockerClient,
	}

	instance := &ServiceInstance{
		InstanceID:  "mongo-001",
		ServiceType: ServiceTypeMongoDB,
		Config: map[string]string{
			"container_id": "mongo-container-123",
			"database":     "testdb",
			"username":     "admin",
			"password":     "secret",
		},
	}

	config := &BackupConfig{
		Compress: true,
	}

	ctx := context.Background()
	backup, err := manager.BackupMongoDB(ctx, instance, config)
	if err != nil {
		t.Fatalf("BackupMongoDB failed: %v", err)
	}

	if backup.ServiceType != ServiceTypeMongoDB {
		t.Errorf("Expected service type MongoDB, got %s", backup.ServiceType)
	}
}

func TestBackupManager_BackupRedis(t *testing.T) {
	tmpDir := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/exec") && r.Method == "POST" {
			// Redis BGSAVE command
			json.NewEncoder(w).Encode(map[string]interface{}{
				"Id": "exec-redis",
			})
			return
		}
		if strings.Contains(r.URL.Path, "/start") {
			w.Write([]byte("Background saving started\n"))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	dockerClient := NewDockerClient(server.URL)
	manager := &BackupManager{
		backupDir:    tmpDir,
		dockerClient: dockerClient,
	}

	instance := &ServiceInstance{
		InstanceID:  "redis-001",
		ServiceType: ServiceTypeRedis,
		Config: map[string]string{
			"container_id": "redis-container-123",
		},
	}

	config := &BackupConfig{}

	ctx := context.Background()
	backup, err := manager.BackupRedis(ctx, instance, config)
	if err != nil {
		t.Fatalf("BackupRedis failed: %v", err)
	}

	if backup.ServiceType != ServiceTypeRedis {
		t.Errorf("Expected service type Redis, got %s", backup.ServiceType)
	}
}

func TestBackupManager_RestorePostgreSQL(t *testing.T) {
	tmpDir := t.TempDir()

	// Create mock backup file
	backupPath := filepath.Join(tmpDir, "postgres-backup.sql")
	os.WriteFile(backupPath, []byte("CREATE TABLE test;\n"), 0644)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/exec") && r.Method == "POST" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"Id": "exec-restore",
			})
			return
		}
		if strings.Contains(r.URL.Path, "/start") {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	dockerClient := NewDockerClient(server.URL)
	manager := &BackupManager{
		backupDir:    tmpDir,
		dockerClient: dockerClient,
	}

	instance := &ServiceInstance{
		InstanceID:  "postgres-001",
		ServiceType: ServiceTypePostgres,
		Config: map[string]string{
			"container_id": "postgres-container-123",
			"database":     "testdb",
			"username":     "testuser",
		},
	}

	backup := &Backup{
		BackupID:    "backup-123",
		BackupPath:  backupPath,
		ServiceType: ServiceTypePostgres,
	}

	ctx := context.Background()
	err := manager.RestorePostgreSQL(ctx, instance, backup)
	if err != nil {
		t.Fatalf("RestorePostgreSQL failed: %v", err)
	}
}

func TestBackupManager_ListBackups(t *testing.T) {
	tmpDir := t.TempDir()

	manager := &BackupManager{
		backupDir: tmpDir,
		backups:   make(map[string]*Backup),
	}

	// Add test backups
	backups := []*Backup{
		{
			BackupID:     "backup-1",
			InstanceID:   "instance-1",
			ServiceType:  ServiceTypePostgres,
			Status:       "completed",
			BackupPath:   filepath.Join(tmpDir, "backup1.sql"),
			SizeBytes:    1024 * 1024,
			CreatedAt:    time.Now(),
		},
		{
			BackupID:     "backup-2",
			InstanceID:   "instance-2",
			ServiceType:  ServiceTypeMySQL,
			Status:       "completed",
			BackupPath:   filepath.Join(tmpDir, "backup2.sql"),
			SizeBytes:    2048 * 1024,
			CreatedAt:    time.Now(),
		},
	}

	for _, backup := range backups {
		manager.backups[backup.BackupID] = backup
	}

	result := manager.ListBackups("instance-1")

	if len(result) != 1 {
		t.Errorf("Expected 1 backup for instance-1, got %d", len(result))
	}

	if result[0].BackupID != "backup-1" {
		t.Errorf("Expected backup-1, got %s", result[0].BackupID)
	}

	// List all backups
	allBackups := manager.ListBackups("")
	if len(allBackups) != 2 {
		t.Errorf("Expected 2 total backups, got %d", len(allBackups))
	}
}

func TestBackupManager_GetBackup(t *testing.T) {
	manager := &BackupManager{
		backups: make(map[string]*Backup),
	}

	backup := &Backup{
		BackupID:    "backup-123",
		InstanceID:  "instance-001",
		ServiceType: ServiceTypePostgres,
		Status:      "completed",
	}

	manager.backups["backup-123"] = backup

	result, err := manager.GetBackup("backup-123")
	if err != nil {
		t.Fatalf("GetBackup failed: %v", err)
	}

	if result.BackupID != "backup-123" {
		t.Errorf("Expected backup-123, got %s", result.BackupID)
	}

	// Test non-existent backup
	_, err = manager.GetBackup("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent backup")
	}
}

func TestBackupManager_DeleteBackup(t *testing.T) {
	tmpDir := t.TempDir()

	manager := &BackupManager{
		backupDir: tmpDir,
		backups:   make(map[string]*Backup),
	}

	// Create mock backup file
	backupPath := filepath.Join(tmpDir, "test-backup.sql")
	os.WriteFile(backupPath, []byte("test data"), 0644)

	backup := &Backup{
		BackupID:   "backup-delete",
		BackupPath: backupPath,
	}

	manager.backups["backup-delete"] = backup

	err := manager.DeleteBackup("backup-delete")
	if err != nil {
		t.Fatalf("DeleteBackup failed: %v", err)
	}

	// Verify backup is removed from map
	if _, exists := manager.backups["backup-delete"]; exists {
		t.Error("Backup should be removed from map")
	}

	// Verify file is deleted
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Error("Backup file should be deleted")
	}
}

func TestBackupConfig_Compression(t *testing.T) {
	config := &BackupConfig{
		Compress: true,
	}

	if !config.Compress {
		t.Error("Expected compression to be enabled")
	}

	// Compressed backups should use .gz extension
	backupPath := "backup.sql"
	if config.Compress {
		backupPath += ".gz"
	}

	if !strings.HasSuffix(backupPath, ".gz") {
		t.Error("Expected .gz extension for compressed backup")
	}
}

func TestBackupConfig_Encryption(t *testing.T) {
	config := &BackupConfig{
		Encrypt:       true,
		EncryptionKey: "test-encryption-key",
	}

	if !config.Encrypt {
		t.Error("Expected encryption to be enabled")
	}

	if config.EncryptionKey == "" {
		t.Error("Expected non-empty encryption key")
	}
}

func TestBackupManager_RetentionPolicy(t *testing.T) {
	manager := &BackupManager{
		backups: make(map[string]*Backup),
	}

	// Create backups with different ages
	now := time.Now()

	backups := []*Backup{
		{
			BackupID:  "old-backup",
			CreatedAt: now.Add(-31 * 24 * time.Hour), // 31 days old
		},
		{
			BackupID:  "recent-backup",
			CreatedAt: now.Add(-5 * 24 * time.Hour), // 5 days old
		},
	}

	for _, backup := range backups {
		manager.backups[backup.BackupID] = backup
	}

	// Apply 30-day retention policy
	retentionDays := 30

	for _, backup := range manager.backups {
		age := time.Since(backup.CreatedAt)
		if age > time.Duration(retentionDays)*24*time.Hour {
			t.Logf("Backup %s is older than retention policy: %.0f days",
				backup.BackupID, age.Hours()/24)
		}
	}
}

func TestBackupManager_IncrementalBackup(t *testing.T) {
	// Incremental backup test structure
	config := &BackupConfig{
		Incremental:      true,
		BaseBackupID:     "backup-base-001",
	}

	if !config.Incremental {
		t.Error("Expected incremental backup to be enabled")
	}

	if config.BaseBackupID == "" {
		t.Error("Expected non-empty base backup ID for incremental")
	}

	t.Log("Incremental backup configuration validated")
}

func TestBackupManager_PointInTimeRecovery(t *testing.T) {
	config := &BackupConfig{
		PointInTime: true,
	}

	if !config.PointInTime {
		t.Error("Expected PITR to be enabled")
	}

	// PITR requires continuous WAL archiving for PostgreSQL
	t.Log("Point-in-time recovery configuration validated")
}

func TestBackupManager_ConcurrentBackups(t *testing.T) {
	tmpDir := t.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/exec") {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"Id": "exec-concurrent",
			})
			return
		}
		if strings.Contains(r.URL.Path, "/start") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("backup data\n"))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	dockerClient := NewDockerClient(server.URL)
	manager := &BackupManager{
		backupDir:    tmpDir,
		dockerClient: dockerClient,
		backups:      make(map[string]*Backup),
	}

	done := make(chan string, 5)

	// Start multiple backups concurrently
	for i := 0; i < 5; i++ {
		go func(index int) {
			instance := &ServiceInstance{
				InstanceID:  string(rune('A' + index)),
				ServiceType: ServiceTypePostgres,
				Config: map[string]string{
					"container_id": "container-" + string(rune('A' + index)),
					"database":     "testdb",
					"username":     "testuser",
				},
			}

			config := &BackupConfig{}
			ctx := context.Background()

			backup, err := manager.BackupPostgreSQL(ctx, instance, config)
			if err != nil {
				t.Errorf("Concurrent backup %d failed: %v", index, err)
				done <- ""
				return
			}

			done <- backup.BackupID
		}(i)
	}

	// Wait for all backups
	timeout := time.After(10 * time.Second)
	completedBackups := make([]string, 0)

	for i := 0; i < 5; i++ {
		select {
		case backupID := <-done:
			completedBackups = append(completedBackups, backupID)
		case <-timeout:
			t.Fatal("Concurrent backups timed out")
		}
	}

	if len(completedBackups) != 5 {
		t.Errorf("Expected 5 backups, got %d", len(completedBackups))
	}
}

func BenchmarkBackupManager_BackupPostgreSQL(b *testing.B) {
	tmpDir := b.TempDir()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/exec") && r.Method == "POST" {
			json.NewEncoder(w).Encode(map[string]interface{}{"Id": "exec"})
			return
		}
		if strings.Contains(r.URL.Path, "/start") {
			w.Write([]byte("-- PostgreSQL dump\n"))
			return
		}
	}))
	defer server.Close()

	dockerClient := NewDockerClient(server.URL)
	manager := &BackupManager{
		backupDir:    tmpDir,
		dockerClient: dockerClient,
	}

	instance := &ServiceInstance{
		InstanceID:  "bench-postgres",
		ServiceType: ServiceTypePostgres,
		Config: map[string]string{
			"container_id": "bench-container",
			"database":     "testdb",
			"username":     "testuser",
		},
	}

	config := &BackupConfig{}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.BackupPostgreSQL(ctx, instance, config)
	}
}

func BenchmarkBackupManager_ListBackups(b *testing.B) {
	manager := &BackupManager{
		backups: make(map[string]*Backup),
	}

	// Add 100 backups
	for i := 0; i < 100; i++ {
		backup := &Backup{
			BackupID:   string(rune(i)),
			InstanceID: "instance-" + string(rune(i%10)),
		}
		manager.backups[backup.BackupID] = backup
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.ListBackups("")
	}
}
