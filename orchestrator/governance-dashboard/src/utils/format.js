/**
 * Formatting helpers for the governance dashboard.
 */

/**
 * Shorten a proposal status enum like PROPOSAL_STATUS_VOTING_PERIOD → VOTING.
 */
export function shortStatus(raw) {
  if (!raw) return '—';
  return raw
    .replace('PROPOSAL_STATUS_', '')
    .replace('_PERIOD', '')
    .replace(/_/g, ' ');
}

/**
 * Shorten a risk tier like RISK_TIER_HIGH → HIGH.
 */
export function shortTier(raw) {
  if (!raw) return '—';
  return raw.replace('RISK_TIER_', '');
}

/**
 * Shorten a gate state like EXECUTION_GATE_SHOCK_ABSORBER → SHOCK_ABSORBER.
 */
export function shortGate(raw) {
  if (!raw) return '—';
  return raw.replace('EXECUTION_GATE_', '');
}

/**
 * Format a number with commas.
 */
export function fmtNum(n) {
  if (n == null) return '—';
  return Number(n).toLocaleString();
}

/**
 * Format basis points as a percentage string.
 * 500 bps → "5.00%"
 */
export function bpsToPercent(bps) {
  if (bps == null) return '—';
  return (Number(bps) / 100).toFixed(2) + '%';
}

/**
 * Format a date string to a short locale representation.
 */
export function fmtDate(isoStr) {
  if (!isoStr) return '—';
  try {
    return new Date(isoStr).toLocaleString(undefined, {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  } catch {
    return isoStr;
  }
}

/**
 * Truncate a hash for display.  64-char → first 8 + … + last 8.
 */
export function truncHash(hash) {
  if (!hash || hash.length <= 20) return hash || '—';
  return `${hash.slice(0, 8)}…${hash.slice(-8)}`;
}
