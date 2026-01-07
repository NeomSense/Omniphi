/**
 * Omniphi Chain Configuration
 * Industry-standard chain configuration for Cosmos SDK chains
 */

export const CHAIN_ID = 'omniphi-testnet-2';
export const CHAIN_NAME = 'Omniphi Testnet';

// Token configuration
export const DENOM = 'omniphi';
export const DISPLAY_DENOM = 'OMNI';
export const DECIMALS = 6;
export const COIN_TYPE = 60; // Ethereum compatible

// Address prefixes
export const BECH32_PREFIX = 'omni';
export const BECH32_PREFIX_VALOPER = 'omnivaloper';
export const BECH32_PREFIX_VALCONS = 'omnivalcons';

// RPC endpoints
export const RPC_ENDPOINTS = {
  primary: 'http://46.202.179.182:26657',
  fallback: 'http://localhost:26657',
} as const;

export const REST_ENDPOINTS = {
  primary: 'http://46.202.179.182:1318', // nginx CORS proxy
  fallback: 'http://localhost:1317',
} as const;

// Gas configuration
export const GAS_PRICE = '0.025';
export const DEFAULT_GAS_ADJUSTMENT = 1.5;

// Staking configuration
export const UNBONDING_DAYS = 21;
export const MIN_DELEGATION = '1000000'; // 1 OMNI in base units

// Governance configuration
export const GOV_DEPOSIT_PERIOD_HOURS = 24;
export const GOV_VOTING_PERIOD_HOURS = 48;
export const GOV_MIN_DEPOSIT = '100000000'; // 100 OMNI
export const GOV_QUORUM = 0.25; // 25%
export const GOV_THRESHOLD = 0.5; // 50%
export const GOV_VETO_THRESHOLD = 0.334; // 33.4%

// Chain registry format for wallet integration
export const CHAIN_INFO = {
  chainId: CHAIN_ID,
  chainName: CHAIN_NAME,
  rpc: RPC_ENDPOINTS.primary,
  rest: REST_ENDPOINTS.primary,
  bip44: {
    coinType: COIN_TYPE,
  },
  bech32Config: {
    bech32PrefixAccAddr: BECH32_PREFIX,
    bech32PrefixAccPub: `${BECH32_PREFIX}pub`,
    bech32PrefixValAddr: BECH32_PREFIX_VALOPER,
    bech32PrefixValPub: `${BECH32_PREFIX_VALOPER}pub`,
    bech32PrefixConsAddr: BECH32_PREFIX_VALCONS,
    bech32PrefixConsPub: `${BECH32_PREFIX_VALCONS}pub`,
  },
  currencies: [
    {
      coinDenom: DISPLAY_DENOM,
      coinMinimalDenom: DENOM,
      coinDecimals: DECIMALS,
      coinGeckoId: 'omniphi', // placeholder
    },
  ],
  feeCurrencies: [
    {
      coinDenom: DISPLAY_DENOM,
      coinMinimalDenom: DENOM,
      coinDecimals: DECIMALS,
      coinGeckoId: 'omniphi',
      gasPriceStep: {
        low: 0.01,
        average: 0.025,
        high: 0.05,
      },
    },
  ],
  stakeCurrency: {
    coinDenom: DISPLAY_DENOM,
    coinMinimalDenom: DENOM,
    coinDecimals: DECIMALS,
    coinGeckoId: 'omniphi',
  },
  features: ['ibc-transfer', 'ibc-go'],
} as const;

// Proposal types
export const PROPOSAL_TYPES = {
  TEXT: 'text',
  PARAMETER_CHANGE: 'parameter_change',
  SOFTWARE_UPGRADE: 'software_upgrade',
  COMMUNITY_POOL_SPEND: 'community_pool_spend',
} as const;

// Vote options
export const VOTE_OPTIONS = {
  YES: 1,
  ABSTAIN: 2,
  NO: 3,
  NO_WITH_VETO: 4,
} as const;

export const VOTE_OPTION_LABELS: Record<number, string> = {
  [VOTE_OPTIONS.YES]: 'Yes',
  [VOTE_OPTIONS.ABSTAIN]: 'Abstain',
  [VOTE_OPTIONS.NO]: 'No',
  [VOTE_OPTIONS.NO_WITH_VETO]: 'No with Veto',
};

// Proposal status
export const PROPOSAL_STATUS = {
  UNSPECIFIED: 0,
  DEPOSIT_PERIOD: 1,
  VOTING_PERIOD: 2,
  PASSED: 3,
  REJECTED: 4,
  FAILED: 5,
} as const;

export const PROPOSAL_STATUS_LABELS: Record<number, string> = {
  [PROPOSAL_STATUS.UNSPECIFIED]: 'Unspecified',
  [PROPOSAL_STATUS.DEPOSIT_PERIOD]: 'Deposit Period',
  [PROPOSAL_STATUS.VOTING_PERIOD]: 'Voting',
  [PROPOSAL_STATUS.PASSED]: 'Passed',
  [PROPOSAL_STATUS.REJECTED]: 'Rejected',
  [PROPOSAL_STATUS.FAILED]: 'Failed',
};

// Explorer links (for future explorer integration)
export const EXPLORER_URL = 'https://explorer.omniphi.network';

// Local storage keys
export const STORAGE_KEYS = {
  WALLET: 'omniphi_wallet',
  SETTINGS: 'omniphi_settings',
  RECENT_ADDRESSES: 'omniphi_recent_addresses',
} as const;
