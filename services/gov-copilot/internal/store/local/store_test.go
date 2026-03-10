package local_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"gov-copilot/internal/report"
	"gov-copilot/internal/store/local"
)

func TestStore_SaveReport(t *testing.T) {
	dir := t.TempDir()
	store, err := local.NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}

	r := &report.Report{
		ProposalID: 42,
		ChainID:    "test-1",
		CreatedAt:  "2026-02-17T00:00:00Z",
		Reporter:   "test",
		AIProvider: "template",
		Summary:    "Test report",
	}

	filePath, hash, err := store.SaveReport(r)
	if err != nil {
		t.Fatalf("SaveReport: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("report file should exist at %s", filePath)
	}

	// Verify hash is 64 hex chars (SHA256)
	if len(hash) != 64 {
		t.Errorf("hash length: got %d, want 64", len(hash))
	}

	// Verify file contents are valid JSON that roundtrips
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}

	var decoded report.Report
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ProposalID != 42 {
		t.Errorf("proposal_id: got %d, want 42", decoded.ProposalID)
	}
	if decoded.Summary != "Test report" {
		t.Errorf("summary: got %q", decoded.Summary)
	}

	// Verify filename pattern
	expectedFile := filepath.Join(dir, "42.json")
	absExpected, _ := filepath.Abs(expectedFile)
	if filePath != absExpected {
		t.Errorf("path: got %q, want %q", filePath, absExpected)
	}
}

func TestStore_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	store, _ := local.NewStore(dir)

	r := &report.Report{ProposalID: 1, Summary: "test"}
	store.SaveReport(r)

	// Temp file should not remain
	tmpPath := filepath.Join(dir, "1.json.tmp")
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temp file should be cleaned up after atomic write")
	}
}

// TestStore_HashDeterminism verifies that saving the same report twice
// produces the same SHA-256 hash — critical for on-chain verification.
func TestStore_HashDeterminism(t *testing.T) {
	r := &report.Report{
		ProposalID:               99,
		ChainID:                  "omniphi-1",
		CreatedAt:                "2026-02-17T12:00:00Z",
		Reporter:                 "gov-copilot-v1",
		AIProvider:               "template",
		Summary:                  "Test determinism",
		KeyChanges:               []string{"change A", "change B"},
		WhatCouldGoWrong:         []string{"risk 1"},
		RecommendedSafetyActions: []string{"action 1", "action 2"},
		Risk: report.RiskSection{
			TierRules: "CRITICAL",
			TierAI:    "HIGH",
			TierFinal: "CRITICAL",
		},
		Timeline: report.TimelineSection{
			CurrentGate:        "GATE_STATE_SHOCK_ABSORBER",
			EarliestExecHeight: 100000,
			Notes:              "waiting",
		},
	}

	// Save to two different directories
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	store1, _ := local.NewStore(dir1)
	store2, _ := local.NewStore(dir2)

	_, hash1, err := store1.SaveReport(r)
	if err != nil {
		t.Fatalf("first save: %v", err)
	}

	_, hash2, err := store2.SaveReport(r)
	if err != nil {
		t.Fatalf("second save: %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("hash not deterministic:\n  first:  %s\n  second: %s", hash1, hash2)
	}

	// Also verify by independently re-marshalling and hashing
	data, _ := json.MarshalIndent(r, "", "  ")
	h := sha256.Sum256(data)
	independent := fmt.Sprintf("%x", h)

	if hash1 != independent {
		t.Errorf("hash doesn't match independent computation:\n  store:  %s\n  manual: %s", hash1, independent)
	}
}

// TestStore_HashMatchesBrowserVerification verifies that the stored file bytes
// produce the expected hash when re-read — simulating what the browser does.
func TestStore_HashMatchesBrowserVerification(t *testing.T) {
	dir := t.TempDir()
	store, _ := local.NewStore(dir)

	r := &report.Report{
		ProposalID: 7,
		ChainID:    "test-1",
		CreatedAt:  "2026-01-01T00:00:00Z",
		Reporter:   "test",
		AIProvider: "template",
		Summary:    "Browser verification test",
	}

	filePath, expectedHash, err := store.SaveReport(r)
	if err != nil {
		t.Fatalf("SaveReport: %v", err)
	}

	// Read file back (simulating browser fetch)
	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// Compute SHA-256 of raw bytes (simulating browser crypto.subtle.digest)
	h := sha256.Sum256(fileBytes)
	browserHash := fmt.Sprintf("%x", h)

	if browserHash != expectedHash {
		t.Errorf("browser-side hash mismatch:\n  on-chain: %s\n  browser:  %s", expectedHash, browserHash)
	}
}

// TestStore_OverwriteReport verifies that saving a report for the same
// proposal ID overwrites the previous file (not append).
func TestStore_OverwriteReport(t *testing.T) {
	dir := t.TempDir()
	store, _ := local.NewStore(dir)

	r1 := &report.Report{ProposalID: 1, Summary: "version 1"}
	r2 := &report.Report{ProposalID: 1, Summary: "version 2"}

	_, hash1, _ := store.SaveReport(r1)
	_, hash2, _ := store.SaveReport(r2)

	if hash1 == hash2 {
		t.Error("different reports should produce different hashes")
	}

	// Verify file contains v2
	data, _ := os.ReadFile(filepath.Join(dir, "1.json"))
	var decoded report.Report
	json.Unmarshal(data, &decoded)
	if decoded.Summary != "version 2" {
		t.Errorf("file should contain v2, got %q", decoded.Summary)
	}
}
