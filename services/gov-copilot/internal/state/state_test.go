package state_test

import (
	"os"
	"path/filepath"
	"testing"

	"gov-copilot/internal/state"
)

func TestState_LoadNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s, err := state.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if s.LastSeenProposalID != 0 {
		t.Errorf("expected 0, got %d", s.LastSeenProposalID)
	}
	if s.IsProcessed(1) {
		t.Error("proposal 1 should not be processed yet")
	}
}

func TestState_MarkAndPersist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s, err := state.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// Mark proposals
	if err := s.MarkProcessed(5, "hash5"); err != nil {
		t.Fatalf("MarkProcessed: %v", err)
	}
	if err := s.MarkProcessed(10, "hash10"); err != nil {
		t.Fatalf("MarkProcessed: %v", err)
	}

	if !s.IsProcessed(5) {
		t.Error("proposal 5 should be processed")
	}
	if !s.IsProcessed(10) {
		t.Error("proposal 10 should be processed")
	}
	if s.IsProcessed(7) {
		t.Error("proposal 7 should not be processed")
	}
	if s.LastSeenProposalID != 10 {
		t.Errorf("last_seen: got %d, want 10", s.LastSeenProposalID)
	}

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("state file should exist on disk")
	}

	// Reload from disk
	s2, err := state.Load(path)
	if err != nil {
		t.Fatalf("Load (reload): %v", err)
	}

	if s2.LastSeenProposalID != 10 {
		t.Errorf("reloaded last_seen: got %d, want 10", s2.LastSeenProposalID)
	}
	if !s2.IsProcessed(5) {
		t.Error("reloaded: proposal 5 should be processed")
	}
	if !s2.IsProcessed(10) {
		t.Error("reloaded: proposal 10 should be processed")
	}
}

func TestState_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s, _ := state.Load(path)
	s.MarkProcessed(1, "h1")

	// Temp file should not remain
	tmpPath := path + ".tmp"
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temp file should be cleaned up after atomic write")
	}
}
