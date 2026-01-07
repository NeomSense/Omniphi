/**
 * Omniphi API Client
 * REST API interactions with the Cosmos chain
 */

import { REST_ENDPOINTS, DENOM } from './constants';

const API_BASE = REST_ENDPOINTS.primary;

/**
 * Generic fetch wrapper with error handling
 */
async function fetchApi<T>(endpoint: string): Promise<T> {
  const response = await fetch(`${API_BASE}${endpoint}`);

  if (!response.ok) {
    const error = await response.text().catch(() => 'Unknown error');
    throw new Error(`API Error (${response.status}): ${error}`);
  }

  return response.json();
}

// ============================================================================
// Account & Balance APIs
// ============================================================================

export interface Balance {
  denom: string;
  amount: string;
}

export interface AccountBalance {
  balances: Balance[];
}

export async function getBalance(address: string): Promise<string> {
  try {
    const data = await fetchApi<AccountBalance>(
      `/cosmos/bank/v1beta1/balances/${address}`
    );
    const omniBalance = data.balances.find((b) => b.denom === DENOM);
    return omniBalance?.amount || '0';
  } catch {
    return '0';
  }
}

export async function getAllBalances(address: string): Promise<Balance[]> {
  try {
    const data = await fetchApi<AccountBalance>(
      `/cosmos/bank/v1beta1/balances/${address}`
    );
    return data.balances;
  } catch {
    return [];
  }
}

// ============================================================================
// Staking APIs
// ============================================================================

export interface Validator {
  operator_address: string;
  consensus_pubkey: {
    '@type': string;
    key: string;
  };
  jailed: boolean;
  status: string;
  tokens: string;
  delegator_shares: string;
  description: {
    moniker: string;
    identity: string;
    website: string;
    security_contact: string;
    details: string;
  };
  unbonding_height: string;
  unbonding_time: string;
  commission: {
    commission_rates: {
      rate: string;
      max_rate: string;
      max_change_rate: string;
    };
    update_time: string;
  };
  min_self_delegation: string;
}

export interface ValidatorsResponse {
  validators: Validator[];
  pagination: {
    next_key: string | null;
    total: string;
  };
}

export async function getValidators(status = 'BOND_STATUS_BONDED'): Promise<Validator[]> {
  try {
    const data = await fetchApi<ValidatorsResponse>(
      `/cosmos/staking/v1beta1/validators?status=${status}`
    );
    return data.validators;
  } catch {
    return [];
  }
}

export interface Delegation {
  delegation: {
    delegator_address: string;
    validator_address: string;
    shares: string;
  };
  balance: {
    denom: string;
    amount: string;
  };
}

export interface DelegationsResponse {
  delegation_responses: Delegation[];
}

export async function getDelegations(address: string): Promise<Delegation[]> {
  try {
    const data = await fetchApi<DelegationsResponse>(
      `/cosmos/staking/v1beta1/delegations/${address}`
    );
    return data.delegation_responses;
  } catch {
    return [];
  }
}

export interface StakingReward {
  validator_address: string;
  reward: Balance[];
}

export interface RewardsResponse {
  rewards: StakingReward[];
  total: Balance[];
}

export async function getStakingRewards(address: string): Promise<RewardsResponse> {
  try {
    return await fetchApi<RewardsResponse>(
      `/cosmos/distribution/v1beta1/delegators/${address}/rewards`
    );
  } catch {
    return { rewards: [], total: [] };
  }
}

// ============================================================================
// Governance APIs
// ============================================================================

export interface Proposal {
  id: string;
  messages: Array<{
    '@type': string;
    content?: {
      '@type': string;
      title: string;
      description: string;
    };
  }>;
  status: string;
  final_tally_result: {
    yes_count: string;
    abstain_count: string;
    no_count: string;
    no_with_veto_count: string;
  };
  submit_time: string;
  deposit_end_time: string;
  total_deposit: Balance[];
  voting_start_time: string;
  voting_end_time: string;
  metadata: string;
  title: string;
  summary: string;
  proposer: string;
}

export interface ProposalsResponse {
  proposals: Proposal[];
  pagination: {
    next_key: string | null;
    total: string;
  };
}

export async function getProposals(status?: string): Promise<Proposal[]> {
  try {
    let endpoint = '/cosmos/gov/v1/proposals';
    if (status) {
      endpoint += `?proposal_status=${status}`;
    }
    const data = await fetchApi<ProposalsResponse>(endpoint);
    return data.proposals;
  } catch {
    return [];
  }
}

export async function getProposal(proposalId: string): Promise<Proposal | null> {
  try {
    const data = await fetchApi<{ proposal: Proposal }>(
      `/cosmos/gov/v1/proposals/${proposalId}`
    );
    return data.proposal;
  } catch {
    return null;
  }
}

export interface Vote {
  proposal_id: string;
  voter: string;
  options: Array<{
    option: string;
    weight: string;
  }>;
  metadata: string;
}

export interface VoteResponse {
  vote: Vote;
}

export async function getVote(
  proposalId: string,
  voter: string
): Promise<Vote | null> {
  try {
    const data = await fetchApi<VoteResponse>(
      `/cosmos/gov/v1/proposals/${proposalId}/votes/${voter}`
    );
    return data.vote;
  } catch {
    return null;
  }
}

export interface TallyResult {
  tally: {
    yes_count: string;
    abstain_count: string;
    no_count: string;
    no_with_veto_count: string;
  };
}

export async function getProposalTally(proposalId: string): Promise<TallyResult['tally'] | null> {
  try {
    const data = await fetchApi<TallyResult>(
      `/cosmos/gov/v1/proposals/${proposalId}/tally`
    );
    return data.tally;
  } catch {
    return null;
  }
}

// ============================================================================
// Tokenomics APIs (Custom Module)
// ============================================================================

export interface TokenomicsParams {
  total_supply_cap: string;
  current_total_supply: string;
  total_minted: string;
  total_burned: string;
  inflation_rate: string;
  inflation_min: string;
  inflation_max: string;
  emission_split_staking: string;
  emission_split_poc: string;
  emission_split_sequencer: string;
  emission_split_treasury: string;
}

export async function getTokenomicsParams(): Promise<TokenomicsParams | null> {
  try {
    const data = await fetchApi<{ params: TokenomicsParams }>(
      '/pos/tokenomics/v1/params'
    );
    return data.params;
  } catch {
    return null;
  }
}

// ============================================================================
// Transaction APIs
// ============================================================================

export interface TxResponse {
  tx_response: {
    height: string;
    txhash: string;
    codespace: string;
    code: number;
    data: string;
    raw_log: string;
    logs: Array<{
      msg_index: number;
      log: string;
      events: Array<{
        type: string;
        attributes: Array<{
          key: string;
          value: string;
        }>;
      }>;
    }>;
    info: string;
    gas_wanted: string;
    gas_used: string;
    tx: unknown;
    timestamp: string;
  };
}

export async function getTx(txHash: string): Promise<TxResponse['tx_response'] | null> {
  try {
    const data = await fetchApi<TxResponse>(
      `/cosmos/tx/v1beta1/txs/${txHash}`
    );
    return data.tx_response;
  } catch {
    return null;
  }
}

// ============================================================================
// Node Info
// ============================================================================

export interface NodeInfo {
  default_node_info: {
    protocol_version: {
      p2p: string;
      block: string;
      app: string;
    };
    default_node_id: string;
    listen_addr: string;
    network: string;
    version: string;
    channels: string;
    moniker: string;
  };
  application_version: {
    name: string;
    app_name: string;
    version: string;
    git_commit: string;
    build_tags: string;
    go_version: string;
  };
}

export async function getNodeInfo(): Promise<NodeInfo | null> {
  try {
    return await fetchApi<NodeInfo>('/cosmos/base/tendermint/v1beta1/node_info');
  } catch {
    return null;
  }
}

export interface BlockInfo {
  block_id: {
    hash: string;
  };
  block: {
    header: {
      chain_id: string;
      height: string;
      time: string;
    };
  };
}

export async function getLatestBlock(): Promise<BlockInfo | null> {
  try {
    return await fetchApi<BlockInfo>('/cosmos/base/tendermint/v1beta1/blocks/latest');
  } catch {
    return null;
  }
}
