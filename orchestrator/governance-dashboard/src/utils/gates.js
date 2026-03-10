/**
 * Gate state definitions for the x/guard execution pipeline.
 *
 * The guard module moves proposals through a series of gates
 * before they can be executed on-chain.
 */

export const GATE_ORDER = [
  'VISIBILITY',
  'SHOCK_ABSORBER',
  'CONDITIONAL_EXECUTION',
  'READY',
  'EXECUTED',
  'ABORTED',
];

/**
 * Friendly labels for each gate.
 */
export const GATE_LABELS = {
  VISIBILITY:             'Visibility',
  SHOCK_ABSORBER:         'Shock Absorber',
  CONDITIONAL_EXECUTION:  'Conditional',
  READY:                  'Ready',
  EXECUTED:               'Executed',
  ABORTED:                'Aborted',
};

/**
 * Get the index of a gate in the ordered pipeline.
 * Normalizes enum names (strips EXECUTION_GATE_ prefix).
 */
export function gateIndex(raw) {
  const name = (raw || '').replace('EXECUTION_GATE_', '');
  const idx = GATE_ORDER.indexOf(name);
  return idx >= 0 ? idx : -1;
}

/**
 * Is this a terminal gate (EXECUTED or ABORTED)?
 */
export function isTerminal(raw) {
  const name = (raw || '').replace('EXECUTION_GATE_', '');
  return name === 'EXECUTED' || name === 'ABORTED';
}

/**
 * Estimate blocks remaining until earliest_exec_height.
 * Returns null if data is insufficient.
 */
export function blocksRemaining(currentHeight, earliestExecHeight) {
  if (!currentHeight || !earliestExecHeight) return null;
  const diff = Number(earliestExecHeight) - Number(currentHeight);
  return diff > 0 ? diff : 0;
}

/**
 * Estimate time remaining from block count.
 * Assumes ~6 seconds per block (configurable).
 */
export function estimateTime(blocks, blockTimeSec = 6) {
  if (blocks == null || blocks <= 0) return null;
  const totalSec = blocks * blockTimeSec;
  if (totalSec < 60) return `${totalSec}s`;
  if (totalSec < 3600) return `${Math.ceil(totalSec / 60)}m`;
  const h = Math.floor(totalSec / 3600);
  const m = Math.ceil((totalSec % 3600) / 60);
  return m > 0 ? `${h}h ${m}m` : `${h}h`;
}

/**
 * CSS class suffix for a tier badge.
 */
export function tierClass(tier) {
  const t = (tier || '').replace('RISK_TIER_', '').toUpperCase();
  switch (t) {
    case 'LOW':      return 'tier-low';
    case 'MED':
    case 'MEDIUM':   return 'tier-med';
    case 'HIGH':     return 'tier-high';
    case 'CRITICAL': return 'tier-crit';
    default:         return 'tier-unknown';
  }
}

/**
 * CSS class suffix for a gate badge.
 */
export function gateClass(gate) {
  const g = (gate || '').replace('EXECUTION_GATE_', '').toUpperCase();
  switch (g) {
    case 'VISIBILITY':            return 'gate-vis';
    case 'SHOCK_ABSORBER':        return 'gate-shock';
    case 'CONDITIONAL_EXECUTION': return 'gate-cond';
    case 'READY':                 return 'gate-ready';
    case 'EXECUTED':              return 'gate-exec';
    case 'ABORTED':               return 'gate-abort';
    default:                      return 'gate-unknown';
  }
}
