package app

import (
	feemarketmodulev1 "pos/proto/pos/feemarket/module/v1"
	guardmodulev1 "pos/proto/pos/guard/module/v1"
	pocmodulev1 "pos/proto/pos/poc/module/v1"
	pormodulev1 "pos/proto/pos/por/module/v1"
	repgovmodulev1 "pos/proto/pos/repgov/module/v1"
	rewardmultmodulev1 "pos/proto/pos/rewardmult/module/v1"
	royaltymodulev1 "pos/proto/pos/royalty/module/v1"
	timelockmodulev1 "pos/proto/pos/timelock/module/v1"
	tokenomicsmodulev1 "pos/proto/pos/tokenomics/module/v1"
	ucimodulev1 "pos/proto/pos/uci/module/v1"
	_ "pos/x/feemarket/module"
	feemarketmoduletypes "pos/x/feemarket/types"
	_ "pos/x/guard/module"
	guardmoduletypes "pos/x/guard/types"
	_ "pos/x/poc/module"
	pocmoduletypes "pos/x/poc/types"
	_ "pos/x/por/module"
	pormoduletypes "pos/x/por/types"
	_ "pos/x/poseq/module"
	poseqmoduletypes "pos/x/poseq/types"
	_ "pos/x/repgov/module"
	repgovmoduletypes "pos/x/repgov/types"
	_ "pos/x/rewardmult/module"
	rewardmultmoduletypes "pos/x/rewardmult/types"
	_ "pos/x/royalty/module"
	royaltymoduletypes "pos/x/royalty/types"
	_ "pos/x/timelock/module"
	timelockmoduletypes "pos/x/timelock/types"
	_ "pos/x/tokenomics/module"
	tokenomicsmoduletypes "pos/x/tokenomics/types"
	_ "pos/x/uci/module"
	ucimoduletypes "pos/x/uci/types"
	"time"

	runtimev1alpha1 "cosmossdk.io/api/cosmos/app/runtime/v1alpha1"
	appv1alpha1 "cosmossdk.io/api/cosmos/app/v1alpha1"
	authmodulev1 "cosmossdk.io/api/cosmos/auth/module/v1"
	bankmodulev1 "cosmossdk.io/api/cosmos/bank/module/v1"
	circuitmodulev1 "cosmossdk.io/api/cosmos/circuit/module/v1"
	consensusmodulev1 "cosmossdk.io/api/cosmos/consensus/module/v1"
	distrmodulev1 "cosmossdk.io/api/cosmos/distribution/module/v1"
	epochsmodulev1 "cosmossdk.io/api/cosmos/epochs/module/v1"
	evidencemodulev1 "cosmossdk.io/api/cosmos/evidence/module/v1"
	feegrantmodulev1 "cosmossdk.io/api/cosmos/feegrant/module/v1"
	genutilmodulev1 "cosmossdk.io/api/cosmos/genutil/module/v1"
	govmodulev1 "cosmossdk.io/api/cosmos/gov/module/v1"
	groupmodulev1 "cosmossdk.io/api/cosmos/group/module/v1"
	mintmodulev1 "cosmossdk.io/api/cosmos/mint/module/v1"
	nftmodulev1 "cosmossdk.io/api/cosmos/nft/module/v1"
	paramsmodulev1 "cosmossdk.io/api/cosmos/params/module/v1"
	slashingmodulev1 "cosmossdk.io/api/cosmos/slashing/module/v1"
	stakingmodulev1 "cosmossdk.io/api/cosmos/staking/module/v1"
	txconfigv1 "cosmossdk.io/api/cosmos/tx/config/v1"
	upgrademodulev1 "cosmossdk.io/api/cosmos/upgrade/module/v1"
	vestingmodulev1 "cosmossdk.io/api/cosmos/vesting/module/v1"
	"cosmossdk.io/depinject/appconfig"
	_ "cosmossdk.io/x/circuit" // import for side-effects
	circuittypes "cosmossdk.io/x/circuit/types"
	_ "cosmossdk.io/x/evidence" // import for side-effects
	evidencetypes "cosmossdk.io/x/evidence/types"
	"cosmossdk.io/x/feegrant"
	_ "cosmossdk.io/x/feegrant/module" // import for side-effects
	"cosmossdk.io/x/nft"
	_ "cosmossdk.io/x/nft/module" // import for side-effects
	_ "cosmossdk.io/x/upgrade"    // import for side-effects
	upgradetypes "cosmossdk.io/x/upgrade/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	_ "github.com/cosmos/cosmos-sdk/x/auth/tx/config" // import for side-effects
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	_ "github.com/cosmos/cosmos-sdk/x/auth/vesting" // import for side-effects
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	_ "github.com/cosmos/cosmos-sdk/x/bank" // import for side-effects
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	_ "github.com/cosmos/cosmos-sdk/x/consensus" // import for side-effects
	consensustypes "github.com/cosmos/cosmos-sdk/x/consensus/types"
	_ "github.com/cosmos/cosmos-sdk/x/distribution" // import for side-effects
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	_ "github.com/cosmos/cosmos-sdk/x/epochs" // import for side-effects
	epochstypes "github.com/cosmos/cosmos-sdk/x/epochs/types"
	genutiltypes "github.com/cosmos/cosmos-sdk/x/genutil/types"
	_ "github.com/cosmos/cosmos-sdk/x/gov" // import for side-effects
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/cosmos-sdk/x/group"
	_ "github.com/cosmos/cosmos-sdk/x/group/module" // import for side-effects
	_ "github.com/cosmos/cosmos-sdk/x/mint"         // import for side-effects
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	_ "github.com/cosmos/cosmos-sdk/x/params" // import for side-effects
	paramstypes "github.com/cosmos/cosmos-sdk/x/params/types"
	_ "github.com/cosmos/cosmos-sdk/x/slashing" // import for side-effects
	slashingtypes "github.com/cosmos/cosmos-sdk/x/slashing/types"
	_ "github.com/cosmos/cosmos-sdk/x/staking" // import for side-effects
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	icatypes "github.com/cosmos/ibc-go/v10/modules/apps/27-interchain-accounts/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"
	"google.golang.org/protobuf/types/known/durationpb"
)

var (
	moduleAccPerms = []*authmodulev1.ModuleAccountPermission{
		{Account: authtypes.FeeCollectorName, Permissions: []string{authtypes.Burner}},
		{Account: distrtypes.ModuleName},
		{Account: minttypes.ModuleName, Permissions: []string{authtypes.Minter}},
		{Account: stakingtypes.BondedPoolName, Permissions: []string{authtypes.Burner, stakingtypes.ModuleName}},
		{Account: stakingtypes.NotBondedPoolName, Permissions: []string{authtypes.Burner, stakingtypes.ModuleName}},
		{Account: govtypes.ModuleName, Permissions: []string{authtypes.Burner}},
		{Account: nft.ModuleName},
		{Account: ibctransfertypes.ModuleName, Permissions: []string{authtypes.Minter, authtypes.Burner}},
		{Account: icatypes.ModuleName},

		{Account: feemarketmoduletypes.ModuleName, Permissions: []string{authtypes.Burner}},
		{Account: pocmoduletypes.ModuleName, Permissions: []string{authtypes.Minter, authtypes.Burner}},
		{Account: pormoduletypes.ModuleName, Permissions: []string{authtypes.Burner}},
		{Account: tokenomicsmoduletypes.ModuleName, Permissions: []string{authtypes.Minter, authtypes.Burner}},
		{Account: royaltymoduletypes.ModuleName, Permissions: []string{authtypes.Minter, authtypes.Burner}},
		{Account: ucimoduletypes.ModuleName, Permissions: []string{authtypes.Burner}},
	}

	// blocked account addresses
	blockAccAddrs = []string{
		authtypes.FeeCollectorName,
		distrtypes.ModuleName,
		minttypes.ModuleName,
		stakingtypes.BondedPoolName,
		stakingtypes.NotBondedPoolName,
		nft.ModuleName,
		// We allow the following module accounts to receive funds:
		// govtypes.ModuleName
	}

	// application configuration (used by depinject)
	appConfig = appconfig.Compose(&appv1alpha1.Config{
		Modules: []*appv1alpha1.ModuleConfig{
			{
				Name: runtime.ModuleName,
				Config: appconfig.WrapAny(&runtimev1alpha1.Module{
					AppName: Name,
					// NOTE: upgrade module is required to be prioritized
					PreBlockers: []string{
						upgradetypes.ModuleName,
						authtypes.ModuleName,
						// this line is used by starport scaffolding # stargate/app/preBlockers
					},
					// During begin block slashing happens after distr.BeginBlocker so that
					// there is nothing left over in the validator fee pool, so as to keep the
					// CanWithdrawInvariant invariant.
					// NOTE: staking module is required if HistoricalEntries param > 0
					BeginBlockers: []string{
						feemarketmoduletypes.ModuleName,  // Must run first to update base fee before distribution
						tokenomicsmoduletypes.ModuleName, // Must run before mint for inflation control
						minttypes.ModuleName,
						distrtypes.ModuleName,
						slashingtypes.ModuleName,
						evidencetypes.ModuleName,
						stakingtypes.ModuleName,
						epochstypes.ModuleName,
						// ibc modules
						ibcexported.ModuleName,
						// chain modules
						pocmoduletypes.ModuleName,
						guardmoduletypes.ModuleName,
						poseqmoduletypes.ModuleName,
						// this line is used by starport scaffolding # stargate/app/beginBlockers
					},
					EndBlockers: []string{
						feemarketmoduletypes.ModuleName, // Process fees first (burn + distribute)
						timelockmoduletypes.ModuleName,  // MUST run before gov to intercept proposals
						guardmoduletypes.ModuleName,     // Guard runs after timelock, before gov to queue passed proposals
						govtypes.ModuleName,
						stakingtypes.ModuleName,
						feegrant.ModuleName,
						group.ModuleName,
						// chain modules
						tokenomicsmoduletypes.ModuleName, // Process IBC acknowledgements
						rewardmultmoduletypes.ModuleName, // Compute PoS reward multipliers at epoch boundaries
						repgovmoduletypes.ModuleName,     // Recompute reputation-weighted governance scores
						pocmoduletypes.ModuleName,
						pormoduletypes.ModuleName,     // Finalize expired batches
						royaltymoduletypes.ModuleName, // Process royalty stream distributions
						ucimoduletypes.ModuleName,     // Process DePIN contribution interface
						poseqmoduletypes.ModuleName,   // Process PoSeq batch ingestion events
						// this line is used by starport scaffolding # stargate/app/endBlockers
					},
					// The following is mostly only needed when ModuleName != StoreKey name.
					OverrideStoreKeys: []*runtimev1alpha1.StoreKeyConfig{
						{
							ModuleName: authtypes.ModuleName,
							KvStoreKey: "acc",
						},
					},
					// NOTE: The genutils module must occur after staking so that pools are
					// properly initialized with tokens from genesis accounts.
					// NOTE: The genutils module must also occur after auth so that it can access the params from auth.
					InitGenesis: []string{
						consensustypes.ModuleName,
						authtypes.ModuleName,
						banktypes.ModuleName,
						feemarketmoduletypes.ModuleName,  // Initialize before distribution for fee processing
						tokenomicsmoduletypes.ModuleName, // Initialize tokenomics early (supply, params, vesting)
						distrtypes.ModuleName,
						stakingtypes.ModuleName,
						slashingtypes.ModuleName,
						govtypes.ModuleName,
						guardmoduletypes.ModuleName,    // Initialize after gov for guard integration
						timelockmoduletypes.ModuleName, // Initialize after gov for timelock integration
						minttypes.ModuleName,
						genutiltypes.ModuleName,
						evidencetypes.ModuleName,
						feegrant.ModuleName,
						vestingtypes.ModuleName,
						nft.ModuleName,
						group.ModuleName,
						upgradetypes.ModuleName,
						circuittypes.ModuleName,
						epochstypes.ModuleName,
						// ibc modules
						ibcexported.ModuleName,
						ibctransfertypes.ModuleName,
						icatypes.ModuleName,
						// chain modules
						pocmoduletypes.ModuleName,
						pormoduletypes.ModuleName,
						rewardmultmoduletypes.ModuleName, // Initialize after staking/slashing for keeper access
						repgovmoduletypes.ModuleName,     // Initialize after staking for reputation weights
						royaltymoduletypes.ModuleName,    // Initialize after poc for royalty token streams
						ucimoduletypes.ModuleName,        // Initialize after poc for DePIN contribution interface
						poseqmoduletypes.ModuleName,      // Initialize after IBC for PoSeq accountability layer
						// this line is used by starport scaffolding # stargate/app/initGenesis
					},
				}),
			},
			{
				Name: authtypes.ModuleName,
				Config: appconfig.WrapAny(&authmodulev1.Module{
					Bech32Prefix:                AccountAddressPrefix,
					ModuleAccountPermissions:    moduleAccPerms,
					EnableUnorderedTransactions: true,
					// By default modules authority is the governance module. This is configurable with the following:
					// Authority: "group", // A custom module authority can be set using a module name
					// Authority: "omni1cwwv22j5ca08ggdv9c2uky355k908694z577tv", // or a specific address
				}),
			},
			{
				Name:   vestingtypes.ModuleName,
				Config: appconfig.WrapAny(&vestingmodulev1.Module{}),
			},
			{
				Name: banktypes.ModuleName,
				Config: appconfig.WrapAny(&bankmodulev1.Module{
					BlockedModuleAccountsOverride: blockAccAddrs,
				}),
			},
			{
				Name:   stakingtypes.ModuleName,
				Config: appconfig.WrapAny(&stakingmodulev1.Module{}),
			},
			{
				Name:   slashingtypes.ModuleName,
				Config: appconfig.WrapAny(&slashingmodulev1.Module{}),
			},
			{
				Name:   "tx",
				Config: appconfig.WrapAny(&txconfigv1.Config{}),
			},
			{
				Name:   genutiltypes.ModuleName,
				Config: appconfig.WrapAny(&genutilmodulev1.Module{}),
			},
			{
				Name:   upgradetypes.ModuleName,
				Config: appconfig.WrapAny(&upgrademodulev1.Module{}),
			},
			{
				Name:   distrtypes.ModuleName,
				Config: appconfig.WrapAny(&distrmodulev1.Module{}),
			},
			{
				Name:   evidencetypes.ModuleName,
				Config: appconfig.WrapAny(&evidencemodulev1.Module{}),
			},
			{
				Name:   minttypes.ModuleName,
				Config: appconfig.WrapAny(&mintmodulev1.Module{}),
			},
			{
				Name: group.ModuleName,
				Config: appconfig.WrapAny(&groupmodulev1.Module{
					MaxExecutionPeriod: durationpb.New(time.Second * 1209600),
					MaxMetadataLen:     255,
				}),
			},
			{
				Name:   nft.ModuleName,
				Config: appconfig.WrapAny(&nftmodulev1.Module{}),
			},
			{
				Name:   feegrant.ModuleName,
				Config: appconfig.WrapAny(&feegrantmodulev1.Module{}),
			},
			{
				Name:   govtypes.ModuleName,
				Config: appconfig.WrapAny(&govmodulev1.Module{}),
			},
			{
				Name:   consensustypes.ModuleName,
				Config: appconfig.WrapAny(&consensusmodulev1.Module{}),
			},
			{
				Name:   circuittypes.ModuleName,
				Config: appconfig.WrapAny(&circuitmodulev1.Module{}),
			},
			{
				Name:   paramstypes.ModuleName,
				Config: appconfig.WrapAny(&paramsmodulev1.Module{}),
			},
			{
				Name:   epochstypes.ModuleName,
				Config: appconfig.WrapAny(&epochsmodulev1.Module{}),
			},
			{
				Name:   feemarketmoduletypes.ModuleName,
				Config: appconfig.WrapAny(&feemarketmodulev1.Module{}),
			},
			{
				Name:   pocmoduletypes.ModuleName,
				Config: appconfig.WrapAny(&pocmodulev1.Module{}),
			},
			{
				Name:   tokenomicsmoduletypes.ModuleName,
				Config: appconfig.WrapAny(&tokenomicsmodulev1.Module{}),
			},
			{
				Name:   timelockmoduletypes.ModuleName,
				Config: appconfig.WrapAny(&timelockmodulev1.Module{}),
			},
			{
				Name:   pormoduletypes.ModuleName,
				Config: appconfig.WrapAny(&pormodulev1.Module{}),
			},
			{
				Name:   rewardmultmoduletypes.ModuleName,
				Config: appconfig.WrapAny(&rewardmultmodulev1.Module{}),
			},
			{
				Name:   guardmoduletypes.ModuleName,
				Config: appconfig.WrapAny(&guardmodulev1.Module{
					// Authority defaults to gov module if not specified
				}),
			},
			{
				Name:   repgovmoduletypes.ModuleName,
				Config: appconfig.WrapAny(&repgovmodulev1.Module{}),
			},
			{
				Name:   royaltymoduletypes.ModuleName,
				Config: appconfig.WrapAny(&royaltymodulev1.Module{}),
			},
			{
				Name:   ucimoduletypes.ModuleName,
				Config: appconfig.WrapAny(&ucimodulev1.Module{}),
			},
			// this line is used by starport scaffolding # stargate/app/moduleConfig
		},
	})
)
