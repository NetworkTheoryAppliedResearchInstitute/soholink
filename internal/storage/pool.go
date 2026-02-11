package storage

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/accounting"
	"github.com/NetworkTheoryAppliedResearchInstitute/soholink/internal/store"
)

// Pool manages shared storage for the federation.
type Pool struct {
	store      *store.Store
	accounting *accounting.Collector
	scanner    *ContentScanner
	baseDir    string
	maxFileSize int64
}

// NewPool creates a new storage pool.
func NewPool(s *store.Store, ac *accounting.Collector, scanner *ContentScanner, baseDir string, maxFileSize int64) *Pool {
	return &Pool{
		store:       s,
		accounting:  ac,
		scanner:     scanner,
		baseDir:     baseDir,
		maxFileSize: maxFileSize,
	}
}

// StoredFile represents a file in the shared storage pool.
type StoredFile struct {
	FileID      string
	OwnerDID    string
	FileName    string
	MimeType    string
	Size        int64
	ContentHash [32]byte
	StoragePath string
	Encrypted   bool
	CreatedAt   time.Time
}

// Upload stores a file, scanning it for malware first.
func (p *Pool) Upload(ctx context.Context, ownerDID string, fileName string, mimeType string, reader io.Reader) (*StoredFile, error) {
	// Create temp file
	tmpPath := filepath.Join(p.baseDir, "tmp", fmt.Sprintf("upload_%d", time.Now().UnixNano()))
	if err := os.MkdirAll(filepath.Dir(tmpPath), 0750); err != nil {
		return nil, fmt.Errorf("failed to create tmp dir: %w", err)
	}

	tmpFile, err := os.Create(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	// Write and compute hash simultaneously
	hasher := sha256.New()
	written, err := io.Copy(io.MultiWriter(tmpFile, hasher), io.LimitReader(reader, p.maxFileSize+1))
	tmpFile.Close()
	if err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("failed to write upload: %w", err)
	}

	if written > p.maxFileSize {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("file exceeds maximum size of %d bytes", p.maxFileSize)
	}

	// Content scanning
	if p.scanner != nil {
		result, err := p.scanner.Scan(ctx, tmpPath)
		if err != nil {
			os.Remove(tmpPath)
			return nil, fmt.Errorf("content scan failed: %w", err)
		}
		if result.IsMalware {
			os.Remove(tmpPath)
			log.Printf("[storage] malware blocked: %s from %s (sig: %s)", fileName, ownerDID, result.Signature)
			p.accounting.Record(&accounting.AccountingEvent{
				Timestamp: time.Now(),
				EventType: "storage_malware_blocked",
				UserDID:   ownerDID,
				Reason:    result.Signature,
			})
			return nil, fmt.Errorf("file rejected: malware detected (%s)", result.Signature)
		}
	}

	// Move to permanent location
	var contentHash [32]byte
	copy(contentHash[:], hasher.Sum(nil))
	hashStr := fmt.Sprintf("%x", contentHash)

	permDir := filepath.Join(p.baseDir, "files", hashStr[:2])
	if err := os.MkdirAll(permDir, 0750); err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("failed to create storage dir: %w", err)
	}

	permPath := filepath.Join(permDir, hashStr)
	if err := os.Rename(tmpPath, permPath); err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("failed to move file to permanent storage: %w", err)
	}

	file := &StoredFile{
		FileID:      hashStr[:16],
		OwnerDID:    ownerDID,
		FileName:    fileName,
		MimeType:    mimeType,
		Size:        written,
		ContentHash: contentHash,
		StoragePath: permPath,
		CreatedAt:   time.Now(),
	}

	p.accounting.Record(&accounting.AccountingEvent{
		Timestamp: time.Now(),
		EventType: "storage_file_uploaded",
		UserDID:   ownerDID,
		Resource:  fileName,
		Decision:  fmt.Sprintf("size:%d,hash:%s", written, hashStr[:16]),
	})

	return file, nil
}

// Download retrieves a file from storage by its content hash.
func (p *Pool) Download(ctx context.Context, hashStr string) (*os.File, error) {
	permPath := filepath.Join(p.baseDir, "files", hashStr[:2], hashStr)
	return os.Open(permPath)
}
