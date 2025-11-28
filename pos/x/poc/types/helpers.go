package types

import (
	"fmt"

	"cosmossdk.io/math"
)

// NewContribution creates a new Contribution instance
func NewContribution(
	id uint64,
	contributor string,
	ctype string,
	uri string,
	hash []byte,
	blockHeight int64,
	blockTime int64,
) Contribution {
	return Contribution{
		Id:           id,
		Contributor:  contributor,
		Ctype:        ctype,
		Uri:          uri,
		Hash:         hash,
		Endorsements: []Endorsement{},
		Verified:     false,
		BlockHeight:  blockHeight,
		BlockTime:    blockTime,
		Rewarded:     false,
	}
}

// NewEndorsement creates a new Endorsement instance
func NewEndorsement(
	valAddr string,
	decision bool,
	power math.Int,
	time int64,
) Endorsement {
	return Endorsement{
		ValAddr:  valAddr,
		Decision: decision,
		Power:    power,
		Time:     time,
	}
}

// AddEndorsement adds an endorsement to a contribution
func (c *Contribution) AddEndorsement(e Endorsement) {
	c.Endorsements = append(c.Endorsements, e)
}

// GetApprovalPower calculates the total voting power that approved this contribution
func (c *Contribution) GetApprovalPower() math.Int {
	totalApproval := math.ZeroInt()
	for _, endorsement := range c.Endorsements {
		if endorsement.Decision {
			totalApproval = totalApproval.Add(endorsement.Power)
		}
	}
	return totalApproval
}

// GetTotalPower calculates the total voting power that endorsed (approve or reject)
func (c *Contribution) GetTotalPower() math.Int {
	total := math.ZeroInt()
	for _, endorsement := range c.Endorsements {
		total = total.Add(endorsement.Power)
	}
	return total
}

// IsPositive returns true if credits value is greater than zero
func (c Credits) IsPositive() bool {
	return c.Amount.IsPositive()
}

// NewCredits creates a new Credits instance
func NewCredits(address string, amount math.Int) Credits {
	return Credits{
		Address: address,
		Amount:  amount,
	}
}

// DefaultGenesis returns the default genesis state
func DefaultGenesis() *GenesisState {
	return &GenesisState{
		Params:        DefaultParams(),
		Contributions: []Contribution{},
		Credits:       []Credits{},
	}
}

// Validate performs basic validation of genesis data
func (gs GenesisState) Validate() error {
	// Validate params
	if err := gs.Params.Validate(); err != nil {
		return err
	}

	// Validate contributions
	seenIDs := make(map[uint64]bool)
	for _, contrib := range gs.Contributions {
		if seenIDs[contrib.Id] {
			return fmt.Errorf("duplicate contribution ID: %d", contrib.Id)
		}
		seenIDs[contrib.Id] = true
	}

	return nil
}

// Equal compares two Tier instances for equality
func (t *Tier) Equal(other *Tier) bool {
	if t == nil && other == nil {
		return true
	}
	if t == nil || other == nil {
		return false
	}

	return t.Name == other.Name &&
		t.Cutoff.Equal(other.Cutoff)
}
