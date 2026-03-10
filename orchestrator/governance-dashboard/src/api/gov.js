/**
 * Cosmos SDK v0.53+ governance REST queries.
 *
 * Endpoint paths are centralized so they can be adjusted for
 * chains that use different route prefixes.
 */
import { getJSON, lcdUrl } from './client';

// ---------- endpoint paths (easy to swap) ----------
const PATHS = {
  proposals:   '/cosmos/gov/v1/proposals',
  proposal:    (id) => `/cosmos/gov/v1/proposals/${id}`,
  tally:       (id) => `/cosmos/gov/v1/proposals/${id}/tally`,
  latestBlock: '/cosmos/base/tendermint/v1beta1/blocks/latest',
};

/**
 * List proposals with optional pagination.
 * Returns { proposals, pagination }.
 */
export async function fetchProposals({ limit = 20, offset = 0, reverse = true } = {}) {
  const params = new URLSearchParams({
    'pagination.limit': String(limit),
    'pagination.offset': String(offset),
    'pagination.reverse': String(reverse),
  });
  return getJSON(lcdUrl(`${PATHS.proposals}?${params}`));
}

/**
 * Fetch a single proposal by ID.
 * Returns the proposal object.
 */
export async function fetchProposalById(id) {
  const data = await getJSON(lcdUrl(PATHS.proposal(id)));
  return data.proposal ?? data;
}

/**
 * Fetch tally for a proposal.
 */
export async function fetchTally(id) {
  const data = await getJSON(lcdUrl(PATHS.tally(id)));
  return data.tally ?? data;
}

/**
 * Get the latest block height.
 * Returns a number.
 */
export async function fetchLatestHeight() {
  const data = await getJSON(lcdUrl(PATHS.latestBlock));
  const h = data?.block?.header?.height ?? data?.block_id?.height;
  return Number(h) || 0;
}
