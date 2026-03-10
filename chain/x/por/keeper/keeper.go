package keeper

import (
	"context"
	"encoding/json"
	"fmt"

	"cosmossdk.io/core/store"
	"cosmossdk.io/log"
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"

	"pos/x/por/types"
)

// Keeper manages the PoR module state
type Keeper struct {
	cdc          codec.BinaryCodec
	storeService store.KVStoreService
	tStoreKey    storetypes.StoreKey // transient store for per-block state
	logger       log.Logger
	authority    string // governance module address

	// Required dependencies
	stakingKeeper types.StakingKeeper
	bankKeeper    types.BankKeeper
	accountKeeper types.AccountKeeper

	// Optional dependencies (nil-safe)
	slashingKeeper types.SlashingKeeper
	pocKeeper      types.PocKeeper
}

// NewKeeper creates a new PoR Keeper instance
func NewKeeper(
	cdc codec.BinaryCodec,
	storeService store.KVStoreService,
	tStoreKey storetypes.StoreKey,
	logger log.Logger,
	authority string,
	stakingKeeper types.StakingKeeper,
	bankKeeper types.BankKeeper,
	accountKeeper types.AccountKeeper,
) Keeper {
	// Validate authority address at construction time
	if _, err := sdk.AccAddressFromBech32(authority); err != nil {
		panic(fmt.Sprintf("invalid x/por authority address: %s", authority))
	}

	return Keeper{
		cdc:           cdc,
		storeService:  storeService,
		tStoreKey:     tStoreKey,
		logger:        logger,
		authority:     authority,
		stakingKeeper: stakingKeeper,
		bankKeeper:    bankKeeper,
		accountKeeper: accountKeeper,
	}
}

// SetSlashingKeeper sets the optional slashing keeper (for post-init wiring)
func (k *Keeper) SetSlashingKeeper(sk types.SlashingKeeper) {
	k.slashingKeeper = sk
}

// SetPocKeeper sets the optional PoC keeper for reward integration
func (k *Keeper) SetPocKeeper(pk types.PocKeeper) {
	k.pocKeeper = pk
}

// GetAuthority returns the module's authority address
func (k Keeper) GetAuthority() string {
	return k.authority
}

// Logger returns the module logger
func (k Keeper) Logger() log.Logger {
	return k.logger.With("module", fmt.Sprintf("x/%s", types.ModuleName))
}

// ============================================================================
// App CRUD Operations
// ============================================================================

// SetApp stores an app in the KV store
func (k Keeper) SetApp(ctx context.Context, app types.App) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(app)
	if err != nil {
		return fmt.Errorf("failed to marshal app: %w", err)
	}
	return kvStore.Set(types.GetAppKey(app.AppId), bz)
}

// GetApp retrieves an app by ID
func (k Keeper) GetApp(ctx context.Context, appID uint64) (types.App, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetAppKey(appID))
	if err != nil || bz == nil {
		return types.App{}, false
	}
	var app types.App
	if err := json.Unmarshal(bz, &app); err != nil {
		return types.App{}, false
	}
	return app, true
}

// GetNextAppID returns the next auto-increment app ID and increments the counter
func (k Keeper) GetNextAppID(ctx context.Context) (uint64, error) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.KeyNextAppID)
	if err != nil || bz == nil {
		// Default to 1 if not set
		if err := kvStore.Set(types.KeyNextAppID, sdk.Uint64ToBigEndian(2)); err != nil {
			return 0, err
		}
		return 1, nil
	}
	id := sdk.BigEndianToUint64(bz)
	if err := kvStore.Set(types.KeyNextAppID, sdk.Uint64ToBigEndian(id+1)); err != nil {
		return 0, err
	}
	return id, nil
}

// IterateApps iterates over all apps. Callback returns true to stop iteration.
func (k Keeper) IterateApps(ctx context.Context, cb func(app types.App) bool) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.KeyPrefixApp, storetypes.PrefixEndBytes(types.KeyPrefixApp))
	if err != nil {
		return err
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var app types.App
		if err := json.Unmarshal(iter.Value(), &app); err != nil {
			continue
		}
		if cb(app) {
			break
		}
	}
	return nil
}

// GetAllApps returns all registered apps
func (k Keeper) GetAllApps(ctx context.Context) []types.App {
	apps := []types.App{}
	_ = k.IterateApps(ctx, func(app types.App) bool {
		apps = append(apps, app)
		return false
	})
	return apps
}

// ============================================================================
// VerifierSet CRUD Operations
// ============================================================================

// SetVerifierSet stores a verifier set
func (k Keeper) SetVerifierSet(ctx context.Context, vs types.VerifierSet) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(vs)
	if err != nil {
		return fmt.Errorf("failed to marshal verifier set: %w", err)
	}
	return kvStore.Set(types.GetVerifierSetKey(vs.VerifierSetId), bz)
}

// GetVerifierSet retrieves a verifier set by ID
func (k Keeper) GetVerifierSet(ctx context.Context, id uint64) (types.VerifierSet, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetVerifierSetKey(id))
	if err != nil || bz == nil {
		return types.VerifierSet{}, false
	}
	var vs types.VerifierSet
	if err := json.Unmarshal(bz, &vs); err != nil {
		return types.VerifierSet{}, false
	}
	return vs, true
}

// GetNextVerifierSetID returns and increments the next verifier set ID
func (k Keeper) GetNextVerifierSetID(ctx context.Context) (uint64, error) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.KeyNextVerifierSetID)
	if err != nil || bz == nil {
		if err := kvStore.Set(types.KeyNextVerifierSetID, sdk.Uint64ToBigEndian(2)); err != nil {
			return 0, err
		}
		return 1, nil
	}
	id := sdk.BigEndianToUint64(bz)
	if err := kvStore.Set(types.KeyNextVerifierSetID, sdk.Uint64ToBigEndian(id+1)); err != nil {
		return 0, err
	}
	return id, nil
}

// GetAllVerifierSets returns all verifier sets
func (k Keeper) GetAllVerifierSets(ctx context.Context) []types.VerifierSet {
	sets := []types.VerifierSet{}
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.KeyPrefixVerifierSet, storetypes.PrefixEndBytes(types.KeyPrefixVerifierSet))
	if err != nil {
		return sets
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var vs types.VerifierSet
		if err := json.Unmarshal(iter.Value(), &vs); err != nil {
			continue
		}
		sets = append(sets, vs)
	}
	return sets
}

// ============================================================================
// BatchCommitment CRUD Operations
// ============================================================================

// SetBatch stores a batch commitment and maintains all indexes
func (k Keeper) SetBatch(ctx context.Context, batch types.BatchCommitment) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("failed to marshal batch: %w", err)
	}

	// Store the batch
	if err := kvStore.Set(types.GetBatchKey(batch.BatchId), bz); err != nil {
		return err
	}

	// Maintain epoch index
	epochKey := types.GetBatchByEpochKey(batch.Epoch, batch.BatchId)
	if err := kvStore.Set(epochKey, sdk.Uint64ToBigEndian(batch.BatchId)); err != nil {
		return err
	}

	// Maintain app index
	appKey := types.GetBatchByAppKey(batch.AppId, batch.BatchId)
	if err := kvStore.Set(appKey, sdk.Uint64ToBigEndian(batch.BatchId)); err != nil {
		return err
	}

	// Maintain status index
	statusKey := types.GetBatchByStatusKey(batch.Status, batch.BatchId)
	return kvStore.Set(statusKey, sdk.Uint64ToBigEndian(batch.BatchId))
}

// UpdateBatchStatus transitions a batch to a new status and updates the status index
func (k Keeper) UpdateBatchStatus(ctx context.Context, batch *types.BatchCommitment, newStatus types.BatchStatus) error {
	kvStore := k.storeService.OpenKVStore(ctx)

	// Remove old status index
	oldStatusKey := types.GetBatchByStatusKey(batch.Status, batch.BatchId)
	if err := kvStore.Delete(oldStatusKey); err != nil {
		return fmt.Errorf("failed to remove old status index: %w", err)
	}

	// Update status
	batch.Status = newStatus

	// Add new status index
	newStatusKey := types.GetBatchByStatusKey(newStatus, batch.BatchId)
	if err := kvStore.Set(newStatusKey, sdk.Uint64ToBigEndian(batch.BatchId)); err != nil {
		return err
	}

	// Save updated batch
	bz, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("failed to marshal batch: %w", err)
	}
	return kvStore.Set(types.GetBatchKey(batch.BatchId), bz)
}

// GetBatch retrieves a batch by ID
func (k Keeper) GetBatch(ctx context.Context, batchID uint64) (types.BatchCommitment, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetBatchKey(batchID))
	if err != nil || bz == nil {
		return types.BatchCommitment{}, false
	}
	var batch types.BatchCommitment
	if err := json.Unmarshal(bz, &batch); err != nil {
		return types.BatchCommitment{}, false
	}
	return batch, true
}

// GetNextBatchID returns and increments the next batch ID
func (k Keeper) GetNextBatchID(ctx context.Context) (uint64, error) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.KeyNextBatchID)
	if err != nil || bz == nil {
		if err := kvStore.Set(types.KeyNextBatchID, sdk.Uint64ToBigEndian(2)); err != nil {
			return 0, err
		}
		return 1, nil
	}
	id := sdk.BigEndianToUint64(bz)
	if err := kvStore.Set(types.KeyNextBatchID, sdk.Uint64ToBigEndian(id+1)); err != nil {
		return 0, err
	}
	return id, nil
}

// GetBatchesByStatus returns all batches with a given status
func (k Keeper) GetBatchesByStatus(ctx context.Context, status types.BatchStatus) []types.BatchCommitment {
	batches := []types.BatchCommitment{}
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.GetBatchByStatusPrefix(status)
	iter, err := kvStore.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return batches
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		batchID := sdk.BigEndianToUint64(iter.Value())
		batch, found := k.GetBatch(ctx, batchID)
		if found {
			batches = append(batches, batch)
		}
	}
	return batches
}

// GetBatchesByEpoch returns all batches in a given epoch
func (k Keeper) GetBatchesByEpoch(ctx context.Context, epoch uint64) []types.BatchCommitment {
	batches := []types.BatchCommitment{}
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.GetBatchByEpochPrefix(epoch)
	iter, err := kvStore.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return batches
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		batchID := sdk.BigEndianToUint64(iter.Value())
		batch, found := k.GetBatch(ctx, batchID)
		if found {
			batches = append(batches, batch)
		}
	}
	return batches
}

// GetAllBatches returns all batch commitments
func (k Keeper) GetAllBatches(ctx context.Context) []types.BatchCommitment {
	batches := []types.BatchCommitment{}
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.KeyPrefixBatch, storetypes.PrefixEndBytes(types.KeyPrefixBatch))
	if err != nil {
		return batches
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var batch types.BatchCommitment
		if err := json.Unmarshal(iter.Value(), &batch); err != nil {
			continue
		}
		batches = append(batches, batch)
	}
	return batches
}

// ============================================================================
// Attestation CRUD Operations
// ============================================================================

// SetAttestation stores an attestation
func (k Keeper) SetAttestation(ctx context.Context, att types.Attestation) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(att)
	if err != nil {
		return fmt.Errorf("failed to marshal attestation: %w", err)
	}
	return kvStore.Set(types.GetAttestationKey(att.BatchId, att.VerifierAddress), bz)
}

// GetAttestation retrieves an attestation for a batch by verifier address
func (k Keeper) GetAttestation(ctx context.Context, batchID uint64, verifierAddr string) (types.Attestation, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetAttestationKey(batchID, verifierAddr))
	if err != nil || bz == nil {
		return types.Attestation{}, false
	}
	var att types.Attestation
	if err := json.Unmarshal(bz, &att); err != nil {
		return types.Attestation{}, false
	}
	return att, true
}

// GetAttestationsForBatch returns all attestations for a given batch
func (k Keeper) GetAttestationsForBatch(ctx context.Context, batchID uint64) []types.Attestation {
	attestations := []types.Attestation{}
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.GetAttestationsByBatchPrefix(batchID)
	iter, err := kvStore.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return attestations
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var att types.Attestation
		if err := json.Unmarshal(iter.Value(), &att); err != nil {
			continue
		}
		attestations = append(attestations, att)
	}
	return attestations
}

// GetAllAttestations returns all attestations
func (k Keeper) GetAllAttestations(ctx context.Context) []types.Attestation {
	attestations := []types.Attestation{}
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.KeyPrefixAttestation, storetypes.PrefixEndBytes(types.KeyPrefixAttestation))
	if err != nil {
		return attestations
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var att types.Attestation
		if err := json.Unmarshal(iter.Value(), &att); err != nil {
			continue
		}
		attestations = append(attestations, att)
	}
	return attestations
}

// ============================================================================
// Challenge CRUD Operations
// ============================================================================

// SetChallenge stores a challenge and maintains the batch index
func (k Keeper) SetChallenge(ctx context.Context, challenge types.Challenge) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(challenge)
	if err != nil {
		return fmt.Errorf("failed to marshal challenge: %w", err)
	}

	// Store the challenge
	if err := kvStore.Set(types.GetChallengeKey(challenge.ChallengeId), bz); err != nil {
		return err
	}

	// Maintain batch index
	batchIdxKey := types.GetChallengeByBatchKey(challenge.BatchId, challenge.ChallengeId)
	return kvStore.Set(batchIdxKey, sdk.Uint64ToBigEndian(challenge.ChallengeId))
}

// GetChallenge retrieves a challenge by ID
func (k Keeper) GetChallenge(ctx context.Context, challengeID uint64) (types.Challenge, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetChallengeKey(challengeID))
	if err != nil || bz == nil {
		return types.Challenge{}, false
	}
	var challenge types.Challenge
	if err := json.Unmarshal(bz, &challenge); err != nil {
		return types.Challenge{}, false
	}
	return challenge, true
}

// GetNextChallengeID returns and increments the next challenge ID
func (k Keeper) GetNextChallengeID(ctx context.Context) (uint64, error) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.KeyNextChallengeID)
	if err != nil || bz == nil {
		if err := kvStore.Set(types.KeyNextChallengeID, sdk.Uint64ToBigEndian(2)); err != nil {
			return 0, err
		}
		return 1, nil
	}
	id := sdk.BigEndianToUint64(bz)
	if err := kvStore.Set(types.KeyNextChallengeID, sdk.Uint64ToBigEndian(id+1)); err != nil {
		return 0, err
	}
	return id, nil
}

// GetChallengesForBatch returns all challenges for a given batch
func (k Keeper) GetChallengesForBatch(ctx context.Context, batchID uint64) []types.Challenge {
	challenges := []types.Challenge{}
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.GetChallengeByBatchPrefix(batchID)
	iter, err := kvStore.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return challenges
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		challengeID := sdk.BigEndianToUint64(iter.Value())
		challenge, found := k.GetChallenge(ctx, challengeID)
		if found {
			challenges = append(challenges, challenge)
		}
	}
	return challenges
}

// HasOpenChallenges returns true if any OPEN challenges exist for a batch
func (k Keeper) HasOpenChallenges(ctx context.Context, batchID uint64) bool {
	challenges := k.GetChallengesForBatch(ctx, batchID)
	for _, c := range challenges {
		if c.Status == types.ChallengeStatusOpen {
			return true
		}
	}
	return false
}

// GetAllChallenges returns all challenges
func (k Keeper) GetAllChallenges(ctx context.Context) []types.Challenge {
	challenges := []types.Challenge{}
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.KeyPrefixChallenge, storetypes.PrefixEndBytes(types.KeyPrefixChallenge))
	if err != nil {
		return challenges
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var challenge types.Challenge
		if err := json.Unmarshal(iter.Value(), &challenge); err != nil {
			continue
		}
		challenges = append(challenges, challenge)
	}
	return challenges
}

// ============================================================================
// VerifierReputation CRUD Operations
// ============================================================================

// SetVerifierReputation stores a verifier reputation record
func (k Keeper) SetVerifierReputation(ctx context.Context, rep types.VerifierReputation) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(rep)
	if err != nil {
		return fmt.Errorf("failed to marshal verifier reputation: %w", err)
	}
	return kvStore.Set(types.GetVerifierReputationKey(rep.Address), bz)
}

// GetVerifierReputation retrieves a verifier reputation record
func (k Keeper) GetVerifierReputation(ctx context.Context, addr string) (types.VerifierReputation, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetVerifierReputationKey(addr))
	if err != nil || bz == nil {
		return types.VerifierReputation{}, false
	}
	var rep types.VerifierReputation
	if err := json.Unmarshal(bz, &rep); err != nil {
		return types.VerifierReputation{}, false
	}
	return rep, true
}

// GetOrCreateVerifierReputation retrieves or creates a default verifier reputation
func (k Keeper) GetOrCreateVerifierReputation(ctx context.Context, addr string) types.VerifierReputation {
	rep, found := k.GetVerifierReputation(ctx, addr)
	if !found {
		return types.NewVerifierReputation(addr)
	}
	return rep
}

// GetAllReputations returns all verifier reputation records
func (k Keeper) GetAllReputations(ctx context.Context) []types.VerifierReputation {
	reps := []types.VerifierReputation{}
	kvStore := k.storeService.OpenKVStore(ctx)
	iter, err := kvStore.Iterator(types.KeyPrefixVerifierReputation, storetypes.PrefixEndBytes(types.KeyPrefixVerifierReputation))
	if err != nil {
		return reps
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		var rep types.VerifierReputation
		if err := json.Unmarshal(iter.Value(), &rep); err != nil {
			continue
		}
		reps = append(reps, rep)
	}
	return reps
}

// ============================================================================
// Epoch Credit Tracking (F2/F6)
// ============================================================================

// GetEpochCreditsUsed returns the total credits used in a given epoch
func (k Keeper) GetEpochCreditsUsed(ctx context.Context, epoch uint64) math.Int {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetEpochCreditTrackerKey(epoch))
	if err != nil || bz == nil {
		return math.ZeroInt()
	}
	var credits math.Int
	if err := json.Unmarshal(bz, &credits); err != nil {
		return math.ZeroInt()
	}
	return credits
}

// IncrementEpochCredits adds credits to the epoch tracker and returns updated total
func (k Keeper) IncrementEpochCredits(ctx context.Context, epoch uint64, amount math.Int) (math.Int, error) {
	current := k.GetEpochCreditsUsed(ctx, epoch)
	updated := current.Add(amount)
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(updated)
	if err != nil {
		return math.ZeroInt(), fmt.Errorf("failed to marshal epoch credits: %w", err)
	}
	if err := kvStore.Set(types.GetEpochCreditTrackerKey(epoch), bz); err != nil {
		return math.ZeroInt(), err
	}
	return updated, nil
}

// ============================================================================
// Batch Leaf Hash Storage (F3)
// ============================================================================

// StoreBatchLeafHashes stores per-record leaf hashes for a batch and builds the reverse index
func (k Keeper) StoreBatchLeafHashes(ctx context.Context, batchID uint64, leafHashes [][]byte) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	for i, leafHash := range leafHashes {
		// Store leaf hash by index
		key := types.GetBatchLeafHashKey(batchID, uint64(i))
		if err := kvStore.Set(key, leafHash); err != nil {
			return fmt.Errorf("failed to store leaf hash %d: %w", i, err)
		}
		// Store reverse index: leaf_hash -> batch_id
		reverseKey := types.GetLeafHashToBatchKey(leafHash, batchID)
		if err := kvStore.Set(reverseKey, sdk.Uint64ToBigEndian(batchID)); err != nil {
			return fmt.Errorf("failed to store leaf-to-batch index %d: %w", i, err)
		}
	}
	return nil
}

// GetBatchesContainingLeaf returns all batch IDs that contain a given leaf hash
func (k Keeper) GetBatchesContainingLeaf(ctx context.Context, leafHash []byte) []uint64 {
	var batchIDs []uint64
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.GetLeafHashToBatchPrefix(leafHash)
	iter, err := kvStore.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return batchIDs
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		batchID := sdk.BigEndianToUint64(iter.Value())
		batchIDs = append(batchIDs, batchID)
	}
	return batchIDs
}

// HasLeafHashInBatch checks if a specific leaf hash exists in a batch
func (k Keeper) HasLeafHashInBatch(ctx context.Context, batchID uint64, leafHash []byte) bool {
	kvStore := k.storeService.OpenKVStore(ctx)
	prefix := types.GetBatchLeafHashPrefix(batchID)
	iter, err := kvStore.Iterator(prefix, storetypes.PrefixEndBytes(prefix))
	if err != nil {
		return false
	}
	defer iter.Close()

	for ; iter.Valid(); iter.Next() {
		if bytesEqual(iter.Value(), leafHash) {
			return true
		}
	}
	return false
}

// ============================================================================
// Challenge Rate Limiting (F4)
// ============================================================================

// GetAddressChallengeCount returns the number of challenges submitted by addr in the given epoch
func (k Keeper) GetAddressChallengeCount(ctx context.Context, addr string, epoch uint64) uint32 {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetChallengeRateLimitKey(addr, epoch))
	if err != nil || bz == nil || len(bz) < 4 {
		return 0
	}
	return uint32(bz[0])<<24 | uint32(bz[1])<<16 | uint32(bz[2])<<8 | uint32(bz[3])
}

// IncrementAddressChallengeCount increments and returns the challenge count for addr in epoch
func (k Keeper) IncrementAddressChallengeCount(ctx context.Context, addr string, epoch uint64) (uint32, error) {
	count := k.GetAddressChallengeCount(ctx, addr, epoch)
	count++
	kvStore := k.storeService.OpenKVStore(ctx)
	countBytes := []byte{byte(count >> 24), byte(count >> 16), byte(count >> 8), byte(count)}
	if err := kvStore.Set(types.GetChallengeRateLimitKey(addr, epoch), countBytes); err != nil {
		return 0, err
	}
	return count, nil
}

// ============================================================================
// PoSeq Commitment Storage (F8)
// ============================================================================

// SetPoSeqCommitment stores a registered PoSeq commitment
func (k Keeper) SetPoSeqCommitment(ctx context.Context, commitment types.PoSeqCommitment) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(commitment)
	if err != nil {
		return fmt.Errorf("failed to marshal PoSeq commitment: %w", err)
	}
	return kvStore.Set(types.GetPoSeqCommitmentKey(commitment.CommitmentHash), bz)
}

// GetPoSeqCommitment retrieves a PoSeq commitment by its hash
func (k Keeper) GetPoSeqCommitment(ctx context.Context, commitmentHash []byte) (types.PoSeqCommitment, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.GetPoSeqCommitmentKey(commitmentHash))
	if err != nil || bz == nil {
		return types.PoSeqCommitment{}, false
	}
	var commitment types.PoSeqCommitment
	if err := json.Unmarshal(bz, &commitment); err != nil {
		return types.PoSeqCommitment{}, false
	}
	return commitment, true
}

// SetPoSeqSequencerSet stores the authorized sequencer set
func (k Keeper) SetPoSeqSequencerSet(ctx context.Context, set types.PoSeqSequencerSet) error {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(set)
	if err != nil {
		return fmt.Errorf("failed to marshal PoSeq sequencer set: %w", err)
	}
	return kvStore.Set(types.KeyPoSeqSequencerSet, bz)
}

// GetPoSeqSequencerSet retrieves the authorized sequencer set
func (k Keeper) GetPoSeqSequencerSet(ctx context.Context) (types.PoSeqSequencerSet, bool) {
	kvStore := k.storeService.OpenKVStore(ctx)
	bz, err := kvStore.Get(types.KeyPoSeqSequencerSet)
	if err != nil || bz == nil {
		return types.PoSeqSequencerSet{}, false
	}
	var set types.PoSeqSequencerSet
	if err := json.Unmarshal(bz, &set); err != nil {
		return types.PoSeqSequencerSet{}, false
	}
	return set, true
}

// ============================================================================
// Epoch Derivation (SECURITY: F2/F6 re-audit)
// ============================================================================

// GetCurrentEpoch derives the current epoch from chain state (block height).
// SECURITY: This MUST be used instead of user-supplied msg.Epoch values to prevent
// attackers from bypassing per-epoch credit caps and challenge rate limits.
func (k Keeper) GetCurrentEpoch(ctx context.Context) uint64 {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return uint64(sdkCtx.BlockHeight() / 100)
}

// ============================================================================
// Rate Limiting (Transient Store)
// ============================================================================

// CheckRateLimit checks if the per-block batch submission limit has been reached
func (k Keeper) CheckRateLimit(ctx context.Context) error {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	tStore := sdkCtx.TransientStore(k.tStoreKey)

	key := types.KeyPrefixBlockSubmissionCount
	bz := tStore.Get(key)

	var count uint32
	if bz != nil && len(bz) >= 4 {
		count = uint32(bz[0])<<24 | uint32(bz[1])<<16 | uint32(bz[2])<<8 | uint32(bz[3])
	}

	params := k.GetParams(ctx)
	if count >= params.MaxBatchesPerBlock {
		return types.ErrRateLimitExceeded
	}

	// Increment counter
	count++
	countBytes := []byte{byte(count >> 24), byte(count >> 16), byte(count >> 8), byte(count)}
	tStore.Set(key, countBytes)

	return nil
}
