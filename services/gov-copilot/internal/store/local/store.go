// Package local writes advisory reports to the local filesystem.
package local

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"gov-copilot/internal/report"
)

// Store writes reports as JSON files with atomic writes.
type Store struct {
	dir string
}

// NewStore creates a local file store, ensuring the directory exists.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create report dir: %w", err)
	}
	return &Store{dir: dir}, nil
}

// SaveReport writes the report to disk and returns (filePath, sha256hex).
func (s *Store) SaveReport(r *report.Report) (string, string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("marshal report: %w", err)
	}

	hash := sha256.Sum256(data)
	hexHash := fmt.Sprintf("%x", hash)

	filename := fmt.Sprintf("%d.json", r.ProposalID)
	outPath := filepath.Join(s.dir, filename)

	// Atomic write: temp file then rename
	tmpPath := outPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return "", "", fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmpPath, outPath); err != nil {
		os.Remove(tmpPath) // best-effort cleanup
		return "", "", fmt.Errorf("rename: %w", err)
	}

	absPath, err := filepath.Abs(outPath)
	if err != nil {
		absPath = outPath
	}

	return absPath, hexHash, nil
}
