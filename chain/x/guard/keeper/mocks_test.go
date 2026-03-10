package keeper_test

import (
	"context"
	"fmt"

	"cosmossdk.io/math"
	cmtprotocrypto "github.com/cometbft/cometbft/proto/tendermint/crypto"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"pos/x/guard/keeper"
)

// ============================================================================
// Mock GovKeeper
// ============================================================================

type MockGovKeeper struct {
	proposals map[uint64]govtypes.Proposal
}

func NewMockGovKeeper() *MockGovKeeper {
	return &MockGovKeeper{
		proposals: make(map[uint64]govtypes.Proposal),
	}
}

func (m *MockGovKeeper) SetProposal(proposal govtypes.Proposal) {
	m.proposals[proposal.Id] = proposal
}

func (m *MockGovKeeper) GetProposal(ctx context.Context, proposalID uint64) (govtypes.Proposal, error) {
	proposal, found := m.proposals[proposalID]
	if !found {
		return govtypes.Proposal{}, fmt.Errorf("proposal %d not found", proposalID)
	}
	return proposal, nil
}

func (m *MockGovKeeper) IterateProposals(ctx context.Context, cb func(proposal govtypes.Proposal) (stop bool)) error {
	for _, proposal := range m.proposals {
		if cb(proposal) {
			break
		}
	}
	return nil
}

func (m *MockGovKeeper) GetParams(ctx context.Context) (govtypes.Params, error) {
	return govtypes.DefaultParams(), nil
}

// ============================================================================
// Mock StakingKeeper
// ============================================================================

type MockStakingKeeper struct {
	totalBonded    math.Int
	validators     []stakingtypes.Validator
	powerReduction math.Int
	mockValidators []MockValidator
}

func NewMockStakingKeeper() *MockStakingKeeper {
	return &MockStakingKeeper{
		totalBonded:    math.NewInt(1000000000000),
		validators:     []stakingtypes.Validator{},
		powerReduction: math.NewInt(1000000),
		mockValidators: []MockValidator{},
	}
}

func (m *MockStakingKeeper) TotalBondedTokens(ctx context.Context) (math.Int, error) {
	return m.totalBonded, nil
}

func (m *MockStakingKeeper) GetAllValidators(ctx context.Context) ([]stakingtypes.Validator, error) {
	return m.validators, nil
}

func (m *MockStakingKeeper) Validator(ctx context.Context, addr sdk.ValAddress) (stakingtypes.ValidatorI, error) {
	return nil, nil
}

func (m *MockStakingKeeper) PowerReduction(ctx context.Context) math.Int {
	return m.powerReduction
}

func (m *MockStakingKeeper) IterateBondedValidatorsByPower(ctx context.Context, fn func(index int64, validator stakingtypes.ValidatorI) (stop bool)) error {
	for i, val := range m.mockValidators {
		v := val // capture
		if fn(int64(i), &v) {
			break
		}
	}
	return nil
}

// SetMockValidators sets mock validators for power iteration
func (m *MockStakingKeeper) SetMockValidators(vals []MockValidator) {
	m.mockValidators = vals
}

// MockValidator implements stakingtypes.ValidatorI for testing
type MockValidator struct {
	OperatorAddr string
	Power        int64
}

func (v *MockValidator) GetOperator() string                                    { return v.OperatorAddr }
func (v *MockValidator) GetConsensusPower(_ math.Int) int64                     { return v.Power }
func (v *MockValidator) IsJailed() bool                                         { return false }
func (v *MockValidator) GetMoniker() string                                     { return v.OperatorAddr }
func (v *MockValidator) GetStatus() stakingtypes.BondStatus                     { return stakingtypes.Bonded }
func (v *MockValidator) IsBonded() bool                                         { return true }
func (v *MockValidator) IsUnbonded() bool                                       { return false }
func (v *MockValidator) IsUnbonding() bool                                      { return false }
func (v *MockValidator) GetConsAddr() ([]byte, error)                           { return nil, nil }
func (v *MockValidator) ConsPubKey() (cryptotypes.PubKey, error)                { return nil, nil }
func (v *MockValidator) TmConsPublicKey() (cmtprotocrypto.PublicKey, error)     { return cmtprotocrypto.PublicKey{}, nil }
func (v *MockValidator) GetTokens() math.Int                         { return math.NewInt(v.Power * 1000000) }
func (v *MockValidator) GetBondedTokens() math.Int                   { return math.NewInt(v.Power * 1000000) }
func (v *MockValidator) GetCommission() math.LegacyDec               { return math.LegacyZeroDec() }
func (v *MockValidator) GetMinSelfDelegation() math.Int              { return math.OneInt() }
func (v *MockValidator) GetDelegatorShares() math.LegacyDec          { return math.LegacyNewDec(v.Power) }
func (v *MockValidator) TokensFromShares(s math.LegacyDec) math.LegacyDec          { return s }
func (v *MockValidator) TokensFromSharesTruncated(s math.LegacyDec) math.LegacyDec { return s }
func (v *MockValidator) TokensFromSharesRoundUp(s math.LegacyDec) math.LegacyDec   { return s }
func (v *MockValidator) SharesFromTokens(t math.Int) (math.LegacyDec, error) {
	return math.LegacyNewDecFromInt(t), nil
}
func (v *MockValidator) SharesFromTokensTruncated(t math.Int) (math.LegacyDec, error) {
	return math.LegacyNewDecFromInt(t), nil
}

// ============================================================================
// Mock BankKeeper
// ============================================================================

type MockBankKeeper struct {
	balances map[string]sdk.Coins
}

func NewMockBankKeeper() *MockBankKeeper {
	return &MockBankKeeper{
		balances: make(map[string]sdk.Coins),
	}
}

func (m *MockBankKeeper) SetBalance(addr sdk.AccAddress, coins sdk.Coins) {
	m.balances[addr.String()] = coins
}

func (m *MockBankKeeper) GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	coins, found := m.balances[addr.String()]
	if !found {
		return sdk.NewCoin(denom, math.ZeroInt())
	}
	return sdk.NewCoin(denom, coins.AmountOf(denom))
}

func (m *MockBankKeeper) GetAllBalances(ctx context.Context, addr sdk.AccAddress) sdk.Coins {
	coins, found := m.balances[addr.String()]
	if !found {
		return sdk.NewCoins()
	}
	return coins
}

func (m *MockBankKeeper) SpendableCoins(ctx context.Context, addr sdk.AccAddress) sdk.Coins {
	return m.GetAllBalances(ctx, addr)
}

// ============================================================================
// Mock DistrKeeper
// ============================================================================

type MockDistrKeeper struct {
	feePool distrtypes.FeePool
}

func NewMockDistrKeeper() *MockDistrKeeper {
	return &MockDistrKeeper{
		feePool: distrtypes.FeePool{
			CommunityPool: sdk.DecCoins{},
		},
	}
}

func (m *MockDistrKeeper) SetCommunityPool(coins sdk.DecCoins) {
	m.feePool.CommunityPool = coins
}

func (m *MockDistrKeeper) GetFeePool(ctx context.Context) (distrtypes.FeePool, error) {
	return m.feePool, nil
}

// ============================================================================
// Mock MessageRouter
// ============================================================================

type MockMessageRouter struct {
	handlers map[string]keeper.MsgServiceHandler
}

func NewMockMessageRouter() *MockMessageRouter {
	return &MockMessageRouter{
		handlers: make(map[string]keeper.MsgServiceHandler),
	}
}

func (m *MockMessageRouter) RegisterHandler(msgType string, handler keeper.MsgServiceHandler) {
	m.handlers[msgType] = handler
}

func (m *MockMessageRouter) Handler(msg sdk.Msg) keeper.MsgServiceHandler {
	msgName := sdk.MsgTypeURL(msg)
	if handler, ok := m.handlers[msgName]; ok {
		return handler
	}
	return nil
}
