/**
 * Omniphi Explorer API Client
 * REST API interactions with the Cosmos chain
 */

const API_BASE = import.meta.env.VITE_API_URL || 'http://46.202.179.182:1318';
const RPC_BASE = import.meta.env.VITE_RPC_URL || 'http://46.202.179.182:26657';

async function fetchApi<T>(endpoint: string, base = API_BASE): Promise<T> {
  const response = await fetch(`${base}${endpoint}`);
  if (!response.ok) {
    throw new Error(`API Error: ${response.status}`);
  }
  return response.json();
}

// Block types
export interface Block {
  block_id: {
    hash: string;
  };
  block: {
    header: {
      version: { block: string };
      chain_id: string;
      height: string;
      time: string;
      last_block_id: { hash: string };
      proposer_address: string;
    };
    data: {
      txs: string[];
    };
  };
}

export interface BlocksResponse {
  blocks: Block[];
}

// Transaction types
export interface Transaction {
  txhash: string;
  height: string;
  index: number;
  tx_result: {
    code: number;
    data: string;
    log: string;
    gas_wanted: string;
    gas_used: string;
    events: Array<{
      type: string;
      attributes: Array<{
        key: string;
        value: string;
      }>;
    }>;
  };
  tx: string;
}

export interface TxSearchResponse {
  txs: Transaction[];
  total_count: string;
}

// Validator types
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

// API functions
export async function getLatestBlock(): Promise<Block> {
  return fetchApi<Block>('/cosmos/base/tendermint/v1beta1/blocks/latest');
}

export async function getBlock(height: string): Promise<Block> {
  return fetchApi<Block>(`/cosmos/base/tendermint/v1beta1/blocks/${height}`);
}

export async function getRecentBlocks(count = 10): Promise<Block[]> {
  const latest = await getLatestBlock();
  const latestHeight = parseInt(latest.block.header.height);

  const blocks: Block[] = [latest];

  for (let i = 1; i < count && latestHeight - i > 0; i++) {
    try {
      const block = await getBlock((latestHeight - i).toString());
      blocks.push(block);
    } catch {
      break;
    }
  }

  return blocks;
}

export async function getValidators(): Promise<Validator[]> {
  const response = await fetchApi<{ validators: Validator[] }>(
    '/cosmos/staking/v1beta1/validators?status=BOND_STATUS_BONDED'
  );
  return response.validators;
}

export async function getValidator(address: string): Promise<Validator> {
  const response = await fetchApi<{ validator: Validator }>(
    `/cosmos/staking/v1beta1/validators/${address}`
  );
  return response.validator;
}

export async function getTx(hash: string): Promise<Transaction> {
  const response = await fetchApi<{ tx_response: Transaction }>(
    `/cosmos/tx/v1beta1/txs/${hash}`
  );
  return response.tx_response;
}

export async function searchTxs(query: string, page = 1, limit = 20): Promise<TxSearchResponse> {
  const response = await fetchApi<TxSearchResponse>(
    `/tx_search?query="${encodeURIComponent(query)}"&page=${page}&per_page=${limit}`,
    RPC_BASE
  );
  return response;
}

export async function getAccountBalance(address: string): Promise<{ denom: string; amount: string }[]> {
  const response = await fetchApi<{ balances: { denom: string; amount: string }[] }>(
    `/cosmos/bank/v1beta1/balances/${address}`
  );
  return response.balances;
}

export async function getAccountDelegations(address: string) {
  const response = await fetchApi<{ delegation_responses: unknown[] }>(
    `/cosmos/staking/v1beta1/delegations/${address}`
  );
  return response.delegation_responses;
}

export async function getNodeInfo() {
  return fetchApi<{
    default_node_info: {
      network: string;
      version: string;
      moniker: string;
    };
    application_version: {
      name: string;
      version: string;
    };
  }>('/cosmos/base/tendermint/v1beta1/node_info');
}

export async function getSupply() {
  return fetchApi<{
    supply: { denom: string; amount: string }[];
  }>('/cosmos/bank/v1beta1/supply');
}

export async function getStakingPool() {
  return fetchApi<{
    pool: {
      not_bonded_tokens: string;
      bonded_tokens: string;
    };
  }>('/cosmos/staking/v1beta1/pool');
}

// Utility functions
export function formatAmount(amount: string | number, decimals = 6): string {
  const value = typeof amount === 'string' ? BigInt(amount) : BigInt(amount);
  const divisor = BigInt(10 ** decimals);
  const whole = value / divisor;
  const fraction = value % divisor;

  if (fraction === 0n) {
    return whole.toLocaleString();
  }

  const fractionStr = fraction.toString().padStart(decimals, '0').slice(0, 2);
  return `${whole.toLocaleString()}.${fractionStr}`;
}

export function truncateHash(hash: string, start = 8, end = 6): string {
  if (hash.length <= start + end + 3) return hash;
  return `${hash.slice(0, start)}...${hash.slice(-end)}`;
}

export function truncateAddress(address: string, start = 12, end = 6): string {
  if (address.length <= start + end + 3) return address;
  return `${address.slice(0, start)}...${address.slice(-end)}`;
}
