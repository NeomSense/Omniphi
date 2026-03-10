package keeper

import (
	"context"
	"encoding/json"

	"pos/x/guard/types"
)

// ============================================================================
// Layer 2: AI Model Storage Functions
// ============================================================================

// SetAIModelMetadata stores AI model metadata
func (k Keeper) SetAIModelMetadata(ctx context.Context, metadata types.AIModelMetadata) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	return store.Set(types.AIModelMetadataKey, bz)
}

// GetAIModelMetadata retrieves AI model metadata
func (k Keeper) GetAIModelMetadata(ctx context.Context) (types.AIModelMetadata, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.AIModelMetadataKey)
	if err != nil || bz == nil {
		return types.AIModelMetadata{}, false
	}

	var metadata types.AIModelMetadata
	if err := json.Unmarshal(bz, &metadata); err != nil {
		return types.AIModelMetadata{}, false
	}

	return metadata, true
}

// SetLinearModel stores the linear scoring model
func (k Keeper) SetLinearModel(ctx context.Context, model types.LinearScoringModel) error {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := json.Marshal(model)
	if err != nil {
		return err
	}
	return store.Set(types.LogisticModelKey, bz)
}

// GetLinearModel retrieves the linear scoring model
func (k Keeper) GetLinearModel(ctx context.Context) (types.LinearScoringModel, bool) {
	store := k.storeService.OpenKVStore(ctx)
	bz, err := store.Get(types.LogisticModelKey)
	if err != nil || bz == nil {
		return types.LinearScoringModel{}, false
	}

	var model types.LinearScoringModel
	if err := json.Unmarshal(bz, &model); err != nil {
		return types.LinearScoringModel{}, false
	}

	return model, true
}

// ============================================================================
// Layer 3: Advisory Link Storage Functions
// ============================================================================

// SetAdvisoryLink stores an advisory link for a proposal
func (k Keeper) SetAdvisoryLink(ctx context.Context, link types.AdvisoryLink) error {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetAdvisoryLinkKey(link.ProposalId)

	bz, err := json.Marshal(link)
	if err != nil {
		return err
	}

	return store.Set(key, bz)
}

// GetAdvisoryLink retrieves advisory link for a proposal
func (k Keeper) GetAdvisoryLink(ctx context.Context, proposalID uint64) (types.AdvisoryLink, bool) {
	store := k.storeService.OpenKVStore(ctx)
	key := types.GetAdvisoryLinkKey(proposalID)

	bz, err := store.Get(key)
	if err != nil || bz == nil {
		return types.AdvisoryLink{}, false
	}

	var link types.AdvisoryLink
	if err := json.Unmarshal(bz, &link); err != nil {
		return types.AdvisoryLink{}, false
	}

	return link, true
}

// GetAllAdvisoryLinks retrieves all advisory links
func (k Keeper) GetAllAdvisoryLinks(ctx context.Context) []types.AdvisoryLink {
	store := k.storeService.OpenKVStore(ctx)
	iterator, err := store.Iterator(types.AdvisoryLinkPrefix, types.PrefixEnd(types.AdvisoryLinkPrefix))
	if err != nil {
		k.logger.Error("failed to create advisory links iterator", "error", err)
		return nil
	}
	defer iterator.Close()

	var links []types.AdvisoryLink
	for ; iterator.Valid(); iterator.Next() {
		var link types.AdvisoryLink
		if err := json.Unmarshal(iterator.Value(), &link); err != nil {
			k.logger.Error("failed to unmarshal advisory link", "error", err)
			continue
		}
		links = append(links, link)
	}

	return links
}
