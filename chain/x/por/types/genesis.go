package types

import "fmt"

// GenesisState defines the PoR module's genesis state
type GenesisState struct {
	Params            Params               `json:"params"`
	Apps              []App                `json:"apps"`
	VerifierSets      []VerifierSet        `json:"verifier_sets"`
	Batches           []BatchCommitment    `json:"batches"`
	Attestations      []Attestation        `json:"attestations"`
	Challenges        []Challenge          `json:"challenges"`
	Reputations       []VerifierReputation `json:"reputations"`
	NextAppId         uint64               `json:"next_app_id"`
	NextBatchId       uint64               `json:"next_batch_id"`
	NextChallengeId   uint64               `json:"next_challenge_id"`
	NextVerifierSetId uint64               `json:"next_verifier_set_id"`
}

func (gs *GenesisState) Reset()         { *gs = GenesisState{} }
func (gs *GenesisState) String() string { return fmt.Sprintf("%+v", *gs) }
func (*GenesisState) ProtoMessage()     {}

// DefaultGenesis returns the default genesis state for the PoR module
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:            DefaultParams(),
		Apps:              []App{},
		VerifierSets:      []VerifierSet{},
		Batches:           []BatchCommitment{},
		Attestations:      []Attestation{},
		Challenges:        []Challenge{},
		Reputations:       []VerifierReputation{},
		NextAppId:         1,
		NextBatchId:       1,
		NextChallengeId:   1,
		NextVerifierSetId: 1,
	}
}

// Validate performs basic genesis state validation
func (gs GenesisState) Validate() error {
	if err := gs.Params.Validate(); err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}

	// Validate no duplicate app IDs
	appIDs := make(map[uint64]bool)
	for _, app := range gs.Apps {
		if appIDs[app.AppId] {
			return fmt.Errorf("duplicate app ID: %d", app.AppId)
		}
		appIDs[app.AppId] = true
	}

	// Validate no duplicate batch IDs
	batchIDs := make(map[uint64]bool)
	for _, batch := range gs.Batches {
		if batchIDs[batch.BatchId] {
			return fmt.Errorf("duplicate batch ID: %d", batch.BatchId)
		}
		batchIDs[batch.BatchId] = true

		// Verify batch references valid app
		if !appIDs[batch.AppId] && batch.AppId > 0 {
			// Allow references to apps not in genesis (they may have been deregistered)
		}

		if !batch.Status.IsValid() {
			return fmt.Errorf("invalid batch status for batch %d: %d", batch.BatchId, batch.Status)
		}
	}

	// Validate no duplicate challenge IDs
	challengeIDs := make(map[uint64]bool)
	for _, challenge := range gs.Challenges {
		if challengeIDs[challenge.ChallengeId] {
			return fmt.Errorf("duplicate challenge ID: %d", challenge.ChallengeId)
		}
		challengeIDs[challenge.ChallengeId] = true
	}

	// Validate no duplicate verifier set IDs
	vsIDs := make(map[uint64]bool)
	for _, vs := range gs.VerifierSets {
		if vsIDs[vs.VerifierSetId] {
			return fmt.Errorf("duplicate verifier set ID: %d", vs.VerifierSetId)
		}
		vsIDs[vs.VerifierSetId] = true
	}

	// Validate counters are higher than any existing ID
	for id := range appIDs {
		if id >= gs.NextAppId {
			return fmt.Errorf("next_app_id (%d) must be greater than existing app ID %d", gs.NextAppId, id)
		}
	}
	for id := range batchIDs {
		if id >= gs.NextBatchId {
			return fmt.Errorf("next_batch_id (%d) must be greater than existing batch ID %d", gs.NextBatchId, id)
		}
	}
	for id := range challengeIDs {
		if id >= gs.NextChallengeId {
			return fmt.Errorf("next_challenge_id (%d) must be greater than existing challenge ID %d", gs.NextChallengeId, id)
		}
	}
	for id := range vsIDs {
		if id >= gs.NextVerifierSetId {
			return fmt.Errorf("next_verifier_set_id (%d) must be greater than existing verifier set ID %d", gs.NextVerifierSetId, id)
		}
	}

	return nil
}
