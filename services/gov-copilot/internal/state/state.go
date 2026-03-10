// Package state manages persistent state across service restarts.
package state

import (
	"encoding/json"
	"os"
	"sync"
)

// State tracks which proposals have been processed.
type State struct {
	LastSeenProposalID uint64            `json:"last_seen_proposal_id"`
	ProcessedIDs       map[uint64]string `json:"processed_ids"` // proposal_id -> report_hash

	mu   sync.Mutex
	path string
}

// Load reads state from disk, or returns empty state if not found.
func Load(path string) (*State, error) {
	s := &State{
		ProcessedIDs: make(map[uint64]string),
		path:         path,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	if s.ProcessedIDs == nil {
		s.ProcessedIDs = make(map[uint64]string)
	}
	s.path = path
	return s, nil
}

// MarkProcessed records a proposal as processed and saves to disk.
func (s *State) MarkProcessed(proposalID uint64, reportHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ProcessedIDs[proposalID] = reportHash
	if proposalID > s.LastSeenProposalID {
		s.LastSeenProposalID = proposalID
	}
	return s.save()
}

// IsProcessed returns true if this proposal has already been processed.
func (s *State) IsProcessed(proposalID uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.ProcessedIDs[proposalID]
	return ok
}

func (s *State) save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}

	// Atomic write: write temp then rename
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		os.Remove(tmp) // best-effort cleanup
		return err
	}
	return nil
}
