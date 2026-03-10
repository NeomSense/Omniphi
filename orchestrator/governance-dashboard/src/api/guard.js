/**
 * x/guard module REST queries.
 *
 * The guard module exposes several endpoints per proposal.
 * This layer tries the preferred endpoint first and falls back
 * gracefully so the UI can show partial data.
 */
import { getJSON, guardUrl, postJSON } from './client';

// ---------- endpoint paths ----------
const PATHS = {
  guardStatus:     (id) => `/omniphi/guard/v1/guard_status/${id}`,
  riskReport:      (id) => `/omniphi/guard/v1/risk_report/${id}`,
  queuedExecution: (id) => `/omniphi/guard/v1/queued/${id}`,
  advisoryLink:    (id) => `/omniphi/guard/v1/advisory/${id}`,
  params:          '/omniphi/guard/v1/params',
};

/**
 * Try to fetch a URL; return null on 404 / not-found errors
 * so the UI can render partial panels.
 */
async function tryGet(url) {
  try {
    return await getJSON(url);
  } catch (err) {
    if (err.message?.includes('404') || err.message?.includes('not found')) {
      return null;
    }
    throw err;
  }
}

/**
 * Fetch full guard status for a proposal.
 * Tries the unified guard_status endpoint first, then falls back
 * to fetching risk_report + queued_execution separately.
 *
 * Returns { riskReport, queuedExecution } where either may be null.
 */
export async function fetchGuardStatus(proposalId) {
  // Try unified endpoint
  const status = await tryGet(guardUrl(PATHS.guardStatus(proposalId)));
  if (status) {
    return {
      riskReport: status.risk_report ?? status.report ?? status,
      queuedExecution: status.queued_execution ?? status.execution ?? null,
    };
  }

  // Fallback: fetch individually in parallel
  const [riskReport, queuedExecution] = await Promise.all([
    tryGet(guardUrl(PATHS.riskReport(proposalId))),
    tryGet(guardUrl(PATHS.queuedExecution(proposalId))),
  ]);

  return {
    riskReport: riskReport?.report ?? riskReport,
    queuedExecution: queuedExecution?.execution ?? queuedExecution,
  };
}

/**
 * Fetch advisory link for a proposal.
 * Returns the link object or null if none exists.
 */
export async function fetchAdvisoryLink(proposalId) {
  const data = await tryGet(guardUrl(PATHS.advisoryLink(proposalId)));
  return data?.link ?? data;
}

/**
 * Fetch guard module params.
 */
export async function fetchGuardParams() {
  const data = await getJSON(guardUrl(PATHS.params));
  return data?.params ?? data;
}

/**
 * Confirm execution via backend proxy.
 * This does NOT sign in the browser — it calls a backend endpoint
 * that holds the signing key.
 */
export async function confirmExecution(proposalId) {
  const base = import.meta.env.VITE_BACKEND_ACTIONS_URL;
  if (!base) {
    throw new Error('VITE_BACKEND_ACTIONS_URL is not configured');
  }
  return postJSON(`${base.replace(/\/+$/, '')}/confirm-execution`, {
    proposal_id: proposalId,
  });
}
