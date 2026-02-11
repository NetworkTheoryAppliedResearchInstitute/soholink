package storage

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
)

// ScanResult holds the result of a content scan.
type ScanResult struct {
	IsMalware bool
	Signature string // e.g. "Win.Trojan.Agent-12345"
}

// ContentScanner scans files for malware using ClamAV.
type ContentScanner struct {
	socketPath string
}

// NewContentScanner creates a scanner that communicates with clamd via Unix socket.
func NewContentScanner(socketPath string) *ContentScanner {
	return &ContentScanner{socketPath: socketPath}
}

// Scan sends a file to ClamAV for malware scanning.
func (s *ContentScanner) Scan(ctx context.Context, filePath string) (*ScanResult, error) {
	if s.socketPath == "" {
		// No scanner configured - skip scanning
		return &ScanResult{IsMalware: false}, nil
	}

	conn, err := net.Dial("unix", s.socketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to clamd: %w", err)
	}
	defer conn.Close()

	// Send SCAN command
	fmt.Fprintf(conn, "SCAN %s\n", filePath)

	// Read response
	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		return nil, fmt.Errorf("no response from clamd")
	}
	response := scanner.Text()

	if strings.Contains(response, "FOUND") {
		sig := extractSignature(response)
		return &ScanResult{
			IsMalware: true,
			Signature: sig,
		}, nil
	}

	return &ScanResult{IsMalware: false}, nil
}

// extractSignature pulls the malware signature name from a ClamAV response.
func extractSignature(response string) string {
	// ClamAV response format: "/path/to/file: Signature FOUND"
	parts := strings.SplitN(response, ": ", 2)
	if len(parts) < 2 {
		return "unknown"
	}
	sig := strings.TrimSuffix(parts[1], " FOUND")
	return sig
}
