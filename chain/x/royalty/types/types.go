package types

import (
	"context"
	"encoding/json"

	"cosmossdk.io/math"
)

// TokenStatus represents the lifecycle state of a royalty token
type TokenStatus string

const (
	TokenStatusActive       TokenStatus = "ACTIVE"
	TokenStatusFrozen       TokenStatus = "FROZEN"       // fraud-related freeze
	TokenStatusClawedBack   TokenStatus = "CLAWED_BACK"  // royalties clawed back
	TokenStatusExpired      TokenStatus = "EXPIRED"       // all royalties paid out
)

// RoyaltyToken represents a tokenized royalty stream backed by a PoC contribution
type RoyaltyToken struct {
	// TokenID is the unique identifier
	TokenID uint64 `json:"token_id"`

	// ClaimID is the PoC contribution this token is backed by
	ClaimID uint64 `json:"claim_id"`

	// Owner is the current owner's bech32 address
	Owner string `json:"owner"`

	// OriginalCreator is the contributor who originally created the token
	OriginalCreator string `json:"original_creator"`

	// RoyaltyShare is the percentage of contribution rewards this token represents [0, 1]
	RoyaltyShare math.LegacyDec `json:"royalty_share"`

	// Status is the token lifecycle state
	Status TokenStatus `json:"status"`

	// CreatedAtHeight is the block height when the token was minted
	CreatedAtHeight int64 `json:"created_at_height"`

	// IsFractionalized indicates if this token has been split into fractions
	IsFractionalized bool `json:"is_fractionalized"`

	// FractionCount is the number of fractions (0 if not fractionalized)
	FractionCount uint32 `json:"fraction_count"`

	// TotalPayouts is the cumulative royalties paid to this token's holders
	TotalPayouts math.Int `json:"total_payouts"`

	// Metadata is optional JSON metadata (IPFS URI, description, etc.)
	Metadata string `json:"metadata"`
}

// NewRoyaltyToken creates a new active royalty token
func NewRoyaltyToken(tokenID, claimID uint64, owner string, share math.LegacyDec, height int64) RoyaltyToken {
	return RoyaltyToken{
		TokenID:         tokenID,
		ClaimID:         claimID,
		Owner:           owner,
		OriginalCreator: owner,
		RoyaltyShare:    share,
		Status:          TokenStatusActive,
		CreatedAtHeight: height,
		IsFractionalized: false,
		FractionCount:   0,
		TotalPayouts:    math.ZeroInt(),
		Metadata:        "",
	}
}

func (rt RoyaltyToken) Marshal() ([]byte, error) { return json.Marshal(rt) }
func (rt *RoyaltyToken) Unmarshal(bz []byte) error { return json.Unmarshal(bz, rt) }

// FractionalToken represents a fraction of a royalty token
type FractionalToken struct {
	ParentTokenID uint64         `json:"parent_token_id"`
	FractionIndex uint32         `json:"fraction_index"` // 0-based index
	Owner         string         `json:"owner"`
	Share         math.LegacyDec `json:"share"` // fraction of parent's royalty share
	TotalPayouts  math.Int       `json:"total_payouts"`
}

// RoyaltyPayment records a royalty distribution event
type RoyaltyPayment struct {
	TokenID     uint64   `json:"token_id"`
	Epoch       int64    `json:"epoch"`
	Amount      math.Int `json:"amount"`
	BlockHeight int64    `json:"block_height"`
	ClaimID     uint64   `json:"claim_id"`
}

// AccumulatedRoyalty tracks pending royalties for a token
type AccumulatedRoyalty struct {
	TokenID uint64   `json:"token_id"`
	Amount  math.Int `json:"amount"`
}

// Listing represents a marketplace listing for a royalty token
type Listing struct {
	TokenID   uint64   `json:"token_id"`
	Seller    string   `json:"seller"`
	AskPrice  math.Int `json:"ask_price"`
	Denom     string   `json:"denom"`
	ListedAt  int64    `json:"listed_at"` // block height
}

// ============================================================================
// Messages
// ============================================================================

type MsgServer interface {
	UpdateParams(ctx context.Context, msg *MsgUpdateParams) (*MsgUpdateParamsResponse, error)
	TokenizeRoyalty(ctx context.Context, msg *MsgTokenizeRoyalty) (*MsgTokenizeRoyaltyResponse, error)
	TransferToken(ctx context.Context, msg *MsgTransferToken) (*MsgTransferTokenResponse, error)
	ClaimRoyalties(ctx context.Context, msg *MsgClaimRoyalties) (*MsgClaimRoyaltiesResponse, error)
	FractionalizeToken(ctx context.Context, msg *MsgFractionalizeToken) (*MsgFractionalizeTokenResponse, error)
	ListToken(ctx context.Context, msg *MsgListToken) (*MsgListTokenResponse, error)
	BuyToken(ctx context.Context, msg *MsgBuyToken) (*MsgBuyTokenResponse, error)
	DelistToken(ctx context.Context, msg *MsgDelistToken) (*MsgDelistTokenResponse, error)
}

type MsgUpdateParams struct {
	Authority string `json:"authority"`
	Params    Params `json:"params"`
}

func (m *MsgUpdateParams) ValidateBasic() error {
	if m.Authority == "" {
		return ErrInvalidAuthority
	}
	return m.Params.Validate()
}

func (m *MsgUpdateParams) ProtoMessage()  {}
func (m *MsgUpdateParams) Reset()         { *m = MsgUpdateParams{} }
func (m *MsgUpdateParams) String() string { return "MsgUpdateParams" }

type MsgUpdateParamsResponse struct{}
func (m *MsgUpdateParamsResponse) ProtoMessage()  {}
func (m *MsgUpdateParamsResponse) Reset()         {}
func (m *MsgUpdateParamsResponse) String() string { return "MsgUpdateParamsResponse" }

type MsgTokenizeRoyalty struct {
	Creator      string         `json:"creator"`
	ClaimID      uint64         `json:"claim_id"`
	RoyaltyShare math.LegacyDec `json:"royalty_share"`
	Metadata     string         `json:"metadata"`
}

func (m *MsgTokenizeRoyalty) ValidateBasic() error {
	if m.Creator == "" {
		return ErrInvalidAddress
	}
	if m.ClaimID == 0 {
		return ErrClaimNotFound
	}
	if m.RoyaltyShare.IsNil() || m.RoyaltyShare.IsNegative() || m.RoyaltyShare.IsZero() {
		return ErrInvalidRoyaltyShare
	}
	return nil
}

func (m *MsgTokenizeRoyalty) ProtoMessage()  {}
func (m *MsgTokenizeRoyalty) Reset()         { *m = MsgTokenizeRoyalty{} }
func (m *MsgTokenizeRoyalty) String() string { return "MsgTokenizeRoyalty" }

type MsgTokenizeRoyaltyResponse struct {
	TokenID uint64 `json:"token_id"`
}
func (m *MsgTokenizeRoyaltyResponse) ProtoMessage()  {}
func (m *MsgTokenizeRoyaltyResponse) Reset()         {}
func (m *MsgTokenizeRoyaltyResponse) String() string { return "MsgTokenizeRoyaltyResponse" }

type MsgTransferToken struct {
	Sender    string `json:"sender"`
	Recipient string `json:"recipient"`
	TokenID   uint64 `json:"token_id"`
}

func (m *MsgTransferToken) ValidateBasic() error {
	if m.Sender == "" || m.Recipient == "" {
		return ErrInvalidAddress
	}
	if m.Sender == m.Recipient {
		return ErrInvalidAddress
	}
	return nil
}

func (m *MsgTransferToken) ProtoMessage()  {}
func (m *MsgTransferToken) Reset()         { *m = MsgTransferToken{} }
func (m *MsgTransferToken) String() string { return "MsgTransferToken" }

type MsgTransferTokenResponse struct{}
func (m *MsgTransferTokenResponse) ProtoMessage()  {}
func (m *MsgTransferTokenResponse) Reset()         {}
func (m *MsgTransferTokenResponse) String() string { return "MsgTransferTokenResponse" }

type MsgClaimRoyalties struct {
	Owner   string `json:"owner"`
	TokenID uint64 `json:"token_id"`
}

func (m *MsgClaimRoyalties) ValidateBasic() error {
	if m.Owner == "" {
		return ErrInvalidAddress
	}
	return nil
}

func (m *MsgClaimRoyalties) ProtoMessage()  {}
func (m *MsgClaimRoyalties) Reset()         { *m = MsgClaimRoyalties{} }
func (m *MsgClaimRoyalties) String() string { return "MsgClaimRoyalties" }

type MsgClaimRoyaltiesResponse struct {
	Amount math.Int `json:"amount"`
}
func (m *MsgClaimRoyaltiesResponse) ProtoMessage()  {}
func (m *MsgClaimRoyaltiesResponse) Reset()         {}
func (m *MsgClaimRoyaltiesResponse) String() string { return "MsgClaimRoyaltiesResponse" }

type MsgFractionalizeToken struct {
	Owner     string `json:"owner"`
	TokenID   uint64 `json:"token_id"`
	Fractions uint32 `json:"fractions"` // number of equal fractions
}

func (m *MsgFractionalizeToken) ValidateBasic() error {
	if m.Owner == "" {
		return ErrInvalidAddress
	}
	if m.Fractions < 2 {
		return ErrInvalidFractionCount
	}
	return nil
}

func (m *MsgFractionalizeToken) ProtoMessage()  {}
func (m *MsgFractionalizeToken) Reset()         { *m = MsgFractionalizeToken{} }
func (m *MsgFractionalizeToken) String() string { return "MsgFractionalizeToken" }

type MsgFractionalizeTokenResponse struct{}
func (m *MsgFractionalizeTokenResponse) ProtoMessage()  {}
func (m *MsgFractionalizeTokenResponse) Reset()         {}
func (m *MsgFractionalizeTokenResponse) String() string { return "MsgFractionalizeTokenResponse" }

type MsgListToken struct {
	Seller   string   `json:"seller"`
	TokenID  uint64   `json:"token_id"`
	AskPrice math.Int `json:"ask_price"`
	Denom    string   `json:"denom"`
}

func (m *MsgListToken) ValidateBasic() error {
	if m.Seller == "" {
		return ErrInvalidAddress
	}
	if m.AskPrice.IsNil() || !m.AskPrice.IsPositive() {
		return ErrInsufficientFunds
	}
	return nil
}

func (m *MsgListToken) ProtoMessage()  {}
func (m *MsgListToken) Reset()         { *m = MsgListToken{} }
func (m *MsgListToken) String() string { return "MsgListToken" }

type MsgListTokenResponse struct{}
func (m *MsgListTokenResponse) ProtoMessage()  {}
func (m *MsgListTokenResponse) Reset()         {}
func (m *MsgListTokenResponse) String() string { return "MsgListTokenResponse" }

type MsgBuyToken struct {
	Buyer   string `json:"buyer"`
	TokenID uint64 `json:"token_id"`
}

func (m *MsgBuyToken) ValidateBasic() error {
	if m.Buyer == "" {
		return ErrInvalidAddress
	}
	return nil
}

func (m *MsgBuyToken) ProtoMessage()  {}
func (m *MsgBuyToken) Reset()         { *m = MsgBuyToken{} }
func (m *MsgBuyToken) String() string { return "MsgBuyToken" }

type MsgBuyTokenResponse struct{}
func (m *MsgBuyTokenResponse) ProtoMessage()  {}
func (m *MsgBuyTokenResponse) Reset()         {}
func (m *MsgBuyTokenResponse) String() string { return "MsgBuyTokenResponse" }

type MsgDelistToken struct {
	Seller  string `json:"seller"`
	TokenID uint64 `json:"token_id"`
}

func (m *MsgDelistToken) ValidateBasic() error {
	if m.Seller == "" {
		return ErrInvalidAddress
	}
	return nil
}

func (m *MsgDelistToken) ProtoMessage()  {}
func (m *MsgDelistToken) Reset()         { *m = MsgDelistToken{} }
func (m *MsgDelistToken) String() string { return "MsgDelistToken" }

type MsgDelistTokenResponse struct{}
func (m *MsgDelistTokenResponse) ProtoMessage()  {}
func (m *MsgDelistTokenResponse) Reset()         {}
func (m *MsgDelistTokenResponse) String() string { return "MsgDelistTokenResponse" }

// ============================================================================
// Queries
// ============================================================================

type QueryServer interface {
	Params(ctx context.Context, req *QueryParamsRequest) (*QueryParamsResponse, error)
	RoyaltyToken(ctx context.Context, req *QueryRoyaltyTokenRequest) (*QueryRoyaltyTokenResponse, error)
	TokensByOwner(ctx context.Context, req *QueryTokensByOwnerRequest) (*QueryTokensByOwnerResponse, error)
	TokensByClaim(ctx context.Context, req *QueryTokensByClaimRequest) (*QueryTokensByClaimResponse, error)
	AccumulatedRoyalties(ctx context.Context, req *QueryAccumulatedRoyaltiesRequest) (*QueryAccumulatedRoyaltiesResponse, error)
}

type QueryParamsRequest struct{}
type QueryParamsResponse struct {
	Params Params `json:"params"`
}

type QueryRoyaltyTokenRequest struct{ TokenID uint64 }
type QueryRoyaltyTokenResponse struct{ Token RoyaltyToken `json:"token"` }

type QueryTokensByOwnerRequest struct{ Owner string }
type QueryTokensByOwnerResponse struct{ Tokens []RoyaltyToken `json:"tokens"` }

type QueryTokensByClaimRequest struct{ ClaimID uint64 }
type QueryTokensByClaimResponse struct{ Tokens []RoyaltyToken `json:"tokens"` }

type QueryAccumulatedRoyaltiesRequest struct{ TokenID uint64 }
type QueryAccumulatedRoyaltiesResponse struct{ Amount math.Int `json:"amount"` }

// ============================================================================
// Genesis
// ============================================================================

type GenesisState struct {
	Params Params         `json:"params"`
	Tokens []RoyaltyToken `json:"tokens"`
	NextTokenID uint64    `json:"next_token_id"`
}

func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:      DefaultParams(),
		Tokens:      []RoyaltyToken{},
		NextTokenID: 1,
	}
}

func (gs GenesisState) Validate() error {
	return gs.Params.Validate()
}

// ============================================================================
// Service Registration Stubs
// ============================================================================

func RegisterMsgServer(s interface{}, srv MsgServer)    {}
func RegisterQueryServer(s interface{}, srv QueryServer) {}
