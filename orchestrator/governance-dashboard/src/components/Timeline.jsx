import { GATE_ORDER, GATE_LABELS, gateIndex, isTerminal, blocksRemaining, estimateTime } from '../utils/gates';
import { shortGate } from '../utils/format';

/**
 * Horizontal stepper showing the proposal's gate progression.
 *
 * Props:
 *  - currentGate: raw gate state string (e.g. "EXECUTION_GATE_SHOCK_ABSORBER")
 *  - earliestExecHeight: number | string
 *  - currentHeight: number
 *  - statusNotes: string (optional)
 */
export default function Timeline({ currentGate, earliestExecHeight, currentHeight, statusNotes }) {
  if (!currentGate) {
    return (
      <section className="panel panel-timeline">
        <h3>Execution Timeline</h3>
        <p className="muted">No timeline data available.</p>
      </section>
    );
  }

  const activeIdx = gateIndex(currentGate);
  const blocks = blocksRemaining(currentHeight, earliestExecHeight);
  const timeEst = estimateTime(blocks);
  const terminal = isTerminal(currentGate);
  const extended = statusNotes && /extend/i.test(statusNotes);

  return (
    <section className="panel panel-timeline">
      <h3>
        Execution Timeline
        {extended && <span className="badge badge-extended">Extended</span>}
      </h3>

      <div className="stepper">
        {GATE_ORDER.map((gate, idx) => {
          // Skip ABORTED in the linear stepper unless it's the current state
          if (gate === 'ABORTED' && shortGate(currentGate) !== 'ABORTED') return null;

          let stepClass = 'step';
          if (idx < activeIdx) stepClass += ' step-done';
          else if (idx === activeIdx) stepClass += terminal ? ' step-terminal' : ' step-active';
          else stepClass += ' step-future';

          return (
            <div key={gate} className={stepClass}>
              <div className="step-dot" />
              <div className="step-label">{GATE_LABELS[gate]}</div>
            </div>
          );
        })}
      </div>

      {blocks != null && !terminal && (
        <div className="timeline-info">
          <span className="timeline-blocks">
            {blocks === 0
              ? 'Ready for execution'
              : `~${blocks.toLocaleString()} blocks remaining`}
          </span>
          {timeEst && <span className="timeline-time">(≈ {timeEst})</span>}
          {currentHeight > 0 && (
            <span className="timeline-height">
              Current height: {currentHeight.toLocaleString()}
            </span>
          )}
        </div>
      )}
    </section>
  );
}
