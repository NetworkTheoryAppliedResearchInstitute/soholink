package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// BackupManager handles backup and restore operations for managed services.
type BackupManager struct {
	// Base directory for storing backups
	backupDir string

	// Docker client for executing backup commands
	dockerClient *DockerClient
}

// BackupConfig configures a backup operation.
type BackupConfig struct {
	// ServiceInstance to backup
	Instance *ServiceInstance

	// Backup name/identifier
	Name string

	// Whether to compress the backup
	Compress bool

	// S3 bucket for offsite storage (optional)
	S3Bucket string

	// Retention period in days
	RetentionDays int
}

// Backup represents a completed backup.
type Backup struct {
	// Backup ID
	ID string

	// Service instance ID
	InstanceID string

	// Service type
	ServiceType ServiceType

	// Backup file path
	Path string

	// Backup size in bytes
	Size int64

	// Creation timestamp
	CreatedAt time.Time

	// Backup metadata
	Metadata map[string]string
}

// NewBackupManager creates a new backup manager.
func NewBackupManager(backupDir string, dockerEndpoint string) (*BackupManager, error) {
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	return &BackupManager{
		backupDir:    backupDir,
		dockerClient: NewDockerClient(dockerEndpoint),
	}, nil
}

// BackupPostgreSQL creates a backup of a PostgreSQL database.
func (bm *BackupManager) BackupPostgreSQL(ctx context.Context, instance *ServiceInstance, config *BackupConfig) (*Backup, error) {
	containerID, ok := instance.Config["container_id"]
	if !ok {
		return nil, fmt.Errorf("container_id not found")
	}

	// Create backup directory for this instance
	instanceBackupDir := filepath.Join(bm.backupDir, instance.InstanceID)
	if err := os.MkdirAll(instanceBackupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create instance backup directory: %w", err)
	}

	// Generate backup filename
	timestamp := time.Now().Format("20060102-150405")
	backupName := config.Name
	if backupName == "" {
		backupName = timestamp
	}
	backupFile := filepath.Join(instanceBackupDir, fmt.Sprintf("%s.sql", backupName))
	if config.Compress {
		backupFile += ".gz"
	}

	// Execute pg_dump inside container
	dumpCmd := fmt.Sprintf("pg_dump -U %s -d %s",
		instance.Credentials.Username,
		instance.Credentials.Database,
	)

	if config.Compress {
		dumpCmd += " | gzip"
	}

	execConfig := map[string]interface{}{
		"Cmd": []string{
			"sh", "-c", dumpCmd,
		},
		"AttachStdout": true,
		"AttachStderr": true,
	}

	execID, err := bm.dockerClient.CreateExec(ctx, containerID, execConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create exec: %w", err)
	}

	output, err := bm.dockerClient.StartExec(ctx, execID)
	if err != nil {
		return nil, fmt.Errorf("failed to execute backup: %w", err)
	}

	// Write backup to file
	if err := os.WriteFile(backupFile, []byte(output), 0644); err != nil {
		return nil, fmt.Errorf("failed to write backup file: %w", err)
	}

	// Get file size
	info, err := os.Stat(backupFile)
	if err != nil {
		return nil, fmt.Errorf("failed to stat backup file: %w", err)
	}

	backup := &Backup{
		ID:          fmt.Sprintf("backup-%s-%s", instance.InstanceID, backupName),
		InstanceID:  instance.InstanceID,
		ServiceType: instance.ServiceType,
		Path:        backupFile,
		Size:        info.Size(),
		CreatedAt:   time.Now(),
		Metadata: map[string]string{
			"database":   instance.Credentials.Database,
			"compressed": fmt.Sprintf("%t", config.Compress),
		},
	}

	return backup, nil
}

// RestorePostgreSQL restores a PostgreSQL database from backup.
func (bm *BackupManager) RestorePostgreSQL(ctx context.Context, instance *ServiceInstance, backupPath string) error {
	containerID, ok := instance.Config["container_id"]
	if !ok {
		return fmt.Errorf("container_id not found")
	}

	// Read backup file
	backupData, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup file: %w", err)
	}

	// Determine if compressed
	compressed := filepath.Ext(backupPath) == ".gz"

	// Execute psql to restore
	restoreCmd := fmt.Sprintf("psql -U %s -d %s",
		instance.Credentials.Username,
		instance.Credentials.Database,
	)

	if compressed {
		restoreCmd = "gunzip | " + restoreCmd
	}

	execConfig := map[string]interface{}{
		"Cmd": []string{
			"sh", "-c", restoreCmd,
		},
		"AttachStdin":  true,
		"AttachStdout": true,
		"AttachStderr": true,
		"Stdin":        string(backupData),
	}

	execID, err := bm.dockerClient.CreateExec(ctx, containerID, execConfig)
	if err != nil {
		return fmt.Errorf("failed to create exec: %w", err)
	}

	_, err = bm.dockerClient.StartExec(ctx, execID)
	if err != nil {
		return fmt.Errorf("failed to execute restore: %w", err)
	}

	return nil
}

// BackupMySQL creates a backup of a MySQL database.
func (bm *BackupManager) BackupMySQL(ctx context.Context, instance *ServiceInstance, config *BackupConfig) (*Backup, error) {
	containerID, ok := instance.Config["container_id"]
	if !ok {
		return nil, fmt.Errorf("container_id not found")
	}

	instanceBackupDir := filepath.Join(bm.backupDir, instance.InstanceID)
	if err := os.MkdirAll(instanceBackupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create instance backup directory: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	backupName := config.Name
	if backupName == "" {
		backupName = timestamp
	}
	backupFile := filepath.Join(instanceBackupDir, fmt.Sprintf("%s.sql", backupName))
	if config.Compress {
		backupFile += ".gz"
	}

	// Execute mysqldump
	dumpCmd := fmt.Sprintf("mysqldump -u%s -p%s %s",
		instance.Credentials.Username,
		instance.Credentials.Password,
		instance.Credentials.Database,
	)

	if config.Compress {
		dumpCmd += " | gzip"
	}

	execConfig := map[string]interface{}{
		"Cmd": []string{
			"sh", "-c", dumpCmd,
		},
		"AttachStdout": true,
		"AttachStderr": true,
	}

	execID, err := bm.dockerClient.CreateExec(ctx, containerID, execConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create exec: %w", err)
	}

	output, err := bm.dockerClient.StartExec(ctx, execID)
	if err != nil {
		return nil, fmt.Errorf("failed to execute backup: %w", err)
	}

	if err := os.WriteFile(backupFile, []byte(output), 0644); err != nil {
		return nil, fmt.Errorf("failed to write backup file: %w", err)
	}

	info, err := os.Stat(backupFile)
	if err != nil {
		return nil, fmt.Errorf("failed to stat backup file: %w", err)
	}

	backup := &Backup{
		ID:          fmt.Sprintf("backup-%s-%s", instance.InstanceID, backupName),
		InstanceID:  instance.InstanceID,
		ServiceType: instance.ServiceType,
		Path:        backupFile,
		Size:        info.Size(),
		CreatedAt:   time.Now(),
		Metadata: map[string]string{
			"database":   instance.Credentials.Database,
			"compressed": fmt.Sprintf("%t", config.Compress),
		},
	}

	return backup, nil
}

// BackupMongoDB creates a backup of a MongoDB database.
func (bm *BackupManager) BackupMongoDB(ctx context.Context, instance *ServiceInstance, config *BackupConfig) (*Backup, error) {
	containerID, ok := instance.Config["container_id"]
	if !ok {
		return nil, fmt.Errorf("container_id not found")
	}

	instanceBackupDir := filepath.Join(bm.backupDir, instance.InstanceID)
	if err := os.MkdirAll(instanceBackupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create instance backup directory: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	backupName := config.Name
	if backupName == "" {
		backupName = timestamp
	}
	backupDir := filepath.Join(instanceBackupDir, backupName)

	// Execute mongodump
	rootPassword := instance.Config["root_password"]
	dumpCmd := fmt.Sprintf("mongodump --uri=mongodb://root:%s@localhost:27017/%s --out=/tmp/backup",
		rootPassword,
		instance.Credentials.Database,
	)

	execConfig := map[string]interface{}{
		"Cmd": []string{
			"sh", "-c", dumpCmd,
		},
		"AttachStdout": true,
		"AttachStderr": true,
	}

	execID, err := bm.dockerClient.CreateExec(ctx, containerID, execConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create exec: %w", err)
	}

	_, err = bm.dockerClient.StartExec(ctx, execID)
	if err != nil {
		return nil, fmt.Errorf("failed to execute backup: %w", err)
	}

	// Copy backup from container
	// In production, use docker cp or volume mounts
	// For now, simulate with directory creation
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	backup := &Backup{
		ID:          fmt.Sprintf("backup-%s-%s", instance.InstanceID, backupName),
		InstanceID:  instance.InstanceID,
		ServiceType: instance.ServiceType,
		Path:        backupDir,
		Size:        0, // Would calculate from directory
		CreatedAt:   time.Now(),
		Metadata: map[string]string{
			"database": instance.Credentials.Database,
		},
	}

	return backup, nil
}

// BackupRedis creates a backup of Redis data.
func (bm *BackupManager) BackupRedis(ctx context.Context, instance *ServiceInstance, config *BackupConfig) (*Backup, error) {
	containerID, ok := instance.Config["container_id"]
	if !ok {
		return nil, fmt.Errorf("container_id not found")
	}

	instanceBackupDir := filepath.Join(bm.backupDir, instance.InstanceID)
	if err := os.MkdirAll(instanceBackupDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create instance backup directory: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	backupName := config.Name
	if backupName == "" {
		backupName = timestamp
	}
	backupFile := filepath.Join(instanceBackupDir, fmt.Sprintf("%s.rdb", backupName))

	// Trigger BGSAVE
	saveCmd := fmt.Sprintf("redis-cli -a %s BGSAVE", instance.Credentials.Password)

	execConfig := map[string]interface{}{
		"Cmd": []string{
			"sh", "-c", saveCmd,
		},
		"AttachStdout": true,
		"AttachStderr": true,
	}

	execID, err := bm.dockerClient.CreateExec(ctx, containerID, execConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create exec: %w", err)
	}

	_, err = bm.dockerClient.StartExec(ctx, execID)
	if err != nil {
		return nil, fmt.Errorf("failed to execute backup: %w", err)
	}

	// Wait for BGSAVE to complete
	time.Sleep(2 * time.Second)

	// Copy RDB file from container
	// In production, use docker cp
	// For now, simulate
	if err := os.WriteFile(backupFile, []byte{}, 0644); err != nil {
		return nil, fmt.Errorf("failed to create backup file: %w", err)
	}

	info, err := os.Stat(backupFile)
	if err != nil {
		return nil, fmt.Errorf("failed to stat backup file: %w", err)
	}

	backup := &Backup{
		ID:          fmt.Sprintf("backup-%s-%s", instance.InstanceID, backupName),
		InstanceID:  instance.InstanceID,
		ServiceType: instance.ServiceType,
		Path:        backupFile,
		Size:        info.Size(),
		CreatedAt:   time.Now(),
		Metadata: map[string]string{
			"type": "rdb",
		},
	}

	return backup, nil
}

// ListBackups lists all backups for a service instance.
func (bm *BackupManager) ListBackups(instanceID string) ([]*Backup, error) {
	instanceBackupDir := filepath.Join(bm.backupDir, instanceID)

	if _, err := os.Stat(instanceBackupDir); os.IsNotExist(err) {
		return []*Backup{}, nil
	}

	entries, err := os.ReadDir(instanceBackupDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	backups := make([]*Backup, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		path := filepath.Join(instanceBackupDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		backup := &Backup{
			ID:         fmt.Sprintf("backup-%s-%s", instanceID, entry.Name()),
			InstanceID: instanceID,
			Path:       path,
			Size:       info.Size(),
			CreatedAt:  info.ModTime(),
		}

		backups = append(backups, backup)
	}

	return backups, nil
}

// DeleteBackup deletes a backup file.
func (bm *BackupManager) DeleteBackup(backupPath string) error {
	if err := os.Remove(backupPath); err != nil {
		return fmt.Errorf("failed to delete backup: %w", err)
	}
	return nil
}

// CleanupOldBackups removes backups older than the retention period.
func (bm *BackupManager) CleanupOldBackups(instanceID string, retentionDays int) error {
	backups, err := bm.ListBackups(instanceID)
	if err != nil {
		return err
	}

	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	for _, backup := range backups {
		if backup.CreatedAt.Before(cutoff) {
			if err := bm.DeleteBackup(backup.Path); err != nil {
				// Log error but continue
				fmt.Printf("Failed to delete old backup %s: %v\n", backup.Path, err)
			}
		}
	}

	return nil
}
