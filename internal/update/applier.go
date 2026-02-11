package update

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Applier applies software updates by replacing the current binary
type Applier struct {
	currentBinaryPath string
	backupDir         string
}

// NewApplier creates a new update applier
func NewApplier(currentBinaryPath, backupDir string) *Applier {
	return &Applier{
		currentBinaryPath: currentBinaryPath,
		backupDir:         backupDir,
	}
}

// ApplyUpdate replaces the current binary with the new one
func (a *Applier) ApplyUpdate(newBinary []byte) error {
	// Create backup directory if it doesn't exist
	if err := os.MkdirAll(a.backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Create backup of current binary
	backupPath, err := a.createBackup()
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Write new binary to temporary location
	tempPath := a.currentBinaryPath + ".new"
	if err := os.WriteFile(tempPath, newBinary, 0755); err != nil {
		return fmt.Errorf("failed to write new binary: %w", err)
	}

	// Atomic rename on Unix-like systems
	// On Windows, this may require additional steps
	if err := a.atomicReplace(tempPath); err != nil {
		// Restore from backup on failure
		if restoreErr := a.restoreBackup(backupPath); restoreErr != nil {
			return fmt.Errorf("update failed and restore failed: %w (restore error: %v)", err, restoreErr)
		}
		return fmt.Errorf("update failed: %w", err)
	}

	return nil
}

// createBackup creates a backup of the current binary
func (a *Applier) createBackup() (string, error) {
	timestamp := time.Now().Format("20060102-150405")
	backupName := fmt.Sprintf("fedaaa-%s.backup", timestamp)
	backupPath := filepath.Join(a.backupDir, backupName)

	// Open current binary
	src, err := os.Open(a.currentBinaryPath)
	if err != nil {
		return "", fmt.Errorf("failed to open current binary: %w", err)
	}
	defer src.Close()

	// Create backup file
	dst, err := os.OpenFile(backupPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create backup file: %w", err)
	}
	defer dst.Close()

	// Copy binary to backup
	if _, err := io.Copy(dst, src); err != nil {
		return "", fmt.Errorf("failed to copy binary to backup: %w", err)
	}

	return backupPath, nil
}

// atomicReplace atomically replaces the current binary with the new one
func (a *Applier) atomicReplace(newPath string) error {
	if runtime.GOOS == "windows" {
		// Windows doesn't support atomic rename of running executables
		// Need to use a different strategy:
		// 1. Rename current binary to .old
		// 2. Rename new binary to current
		// 3. Delete .old on next startup
		oldPath := a.currentBinaryPath + ".old"

		// Remove any existing .old file
		os.Remove(oldPath)

		// Rename current to .old
		if err := os.Rename(a.currentBinaryPath, oldPath); err != nil {
			return fmt.Errorf("failed to rename current binary: %w", err)
		}

		// Rename new to current
		if err := os.Rename(newPath, a.currentBinaryPath); err != nil {
			// Try to restore
			os.Rename(oldPath, a.currentBinaryPath)
			return fmt.Errorf("failed to rename new binary: %w", err)
		}

		// Schedule cleanup of .old file
		// (This would be done on next startup)
		return nil
	}

	// Unix-like systems: atomic rename
	if err := os.Rename(newPath, a.currentBinaryPath); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	return nil
}

// restoreBackup restores the binary from a backup
func (a *Applier) restoreBackup(backupPath string) error {
	// Open backup file
	src, err := os.Open(backupPath)
	if err != nil {
		return fmt.Errorf("failed to open backup: %w", err)
	}
	defer src.Close()

	// Create/truncate current binary
	dst, err := os.OpenFile(a.currentBinaryPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("failed to open current binary: %w", err)
	}
	defer dst.Close()

	// Copy backup to current
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to restore from backup: %w", err)
	}

	return nil
}

// ListBackups lists all available backup files
func (a *Applier) ListBackups() ([]BackupInfo, error) {
	entries, err := os.ReadDir(a.backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BackupInfo{}, nil
		}
		return nil, fmt.Errorf("failed to read backup directory: %w", err)
	}

	var backups []BackupInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if !strings.HasSuffix(entry.Name(), ".backup") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		backups = append(backups, BackupInfo{
			Filename:  entry.Name(),
			Path:      filepath.Join(a.backupDir, entry.Name()),
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
		})
	}

	return backups, nil
}

// CleanOldBackups removes backups older than the specified duration
func (a *Applier) CleanOldBackups(maxAge time.Duration) error {
	backups, err := a.ListBackups()
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-maxAge)

	for _, backup := range backups {
		if backup.CreatedAt.Before(cutoff) {
			if err := os.Remove(backup.Path); err != nil {
				return fmt.Errorf("failed to remove old backup %s: %w", backup.Filename, err)
			}
		}
	}

	return nil
}

// BackupInfo represents information about a backup file
type BackupInfo struct {
	Filename  string
	Path      string
	Size      int64
	CreatedAt time.Time
}

// RestartProcess restarts the current process with the same arguments
func RestartProcess() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Get current process arguments
	args := os.Args[1:]

	// Start new process
	process, err := os.StartProcess(executable, append([]string{executable}, args...), &os.ProcAttr{
		Files: []*os.File{os.Stdin, os.Stdout, os.Stderr},
	})
	if err != nil {
		return fmt.Errorf("failed to start new process: %w", err)
	}

	// Release the process so it continues after we exit
	if err := process.Release(); err != nil {
		return fmt.Errorf("failed to release new process: %w", err)
	}

	// Exit current process
	os.Exit(0)

	return nil
}

// VerifyBinaryIntegrity checks if the binary is executable and valid
func VerifyBinaryIntegrity(binaryPath string) error {
	// Check if file exists
	info, err := os.Stat(binaryPath)
	if err != nil {
		return fmt.Errorf("binary not found: %w", err)
	}

	// Check if it's a regular file
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file")
	}

	// Check if it's executable (Unix-like systems)
	if runtime.GOOS != "windows" {
		if info.Mode()&0111 == 0 {
			return fmt.Errorf("binary is not executable")
		}
	}

	// Check minimum file size (should be at least 1MB for a Go binary)
	if info.Size() < 1024*1024 {
		return fmt.Errorf("binary size suspiciously small: %d bytes", info.Size())
	}

	return nil
}
