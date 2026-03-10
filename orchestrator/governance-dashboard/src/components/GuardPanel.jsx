import { shortTier, shortGate, fmtNum, bpsToPercent } from '../utils/format';
import { tierClass, gateClass } from '../utils/gates';
import JsonViewer from './JsonViewer';

/**
 * Guard module data panel for a single proposal.
 *
 * Props:
 *  - riskReport: risk report object or null
 *  - queuedExecution: queued execution object or null
 */
export default function GuardPanel({ riskReport, queuedExecution }) {
  if (!riskReport && !queuedExecution) {
    return (
      <section className="panel panel-guard">
        <h3>Guard Status</h3>
        <p className="muted">No guard data available for this proposal.</p>
      </section>
    );
  }

  return (
    <section className="panel panel-guard">
      <h3>Guard Status</h3>

      {riskReport && (
        <div className="guard-risk">
          <h4>Risk Assessment</h4>
          <div className="kv-grid">
            <Row label="Rules Tier">
              <span className={`badge ${tierClass(riskReport.tier_rules)}`}>
                {shortTier(riskReport.tier_rules)}
              </span>
            </Row>
            <Row label="AI Tier">
              <span className={`badge ${tierClass(riskReport.tier_ai)}`}>
                {shortTier(riskReport.tier_ai)}
              </span>
            </Row>
            <Row label="Final Tier">
              <span className={`badge ${tierClass(riskReport.tier_final)}`}>
                {shortTier(riskReport.tier_final)}
              </span>
            </Row>
            <Row label="Risk Score">
              {riskReport.risk_score ?? riskReport.ai_score ?? '—'}
            </Row>
            <Row label="Treasury BPS">
              {bpsToPercent(riskReport.treasury_bps)}
            </Row>
            <Row label="Churn BPS">
              {bpsToPercent(riskReport.churn_bps)}
            </Row>
            <Row label="Delay Blocks">
              {fmtNum(riskReport.delay_blocks ?? riskReport.computed_delay_blocks)}
            </Row>
            <Row label="Threshold">
              {bpsToPercent(riskReport.threshold_bps ?? riskReport.computed_threshold_bps)}
            </Row>
          </div>
          <JsonViewer label="Raw Risk Report" data={riskReport} />
        </div>
      )}

      {queuedExecution && (
        <div className="guard-exec">
          <h4>Execution Queue</h4>
          <div className="kv-grid">
            <Row label="Current Gate">
              <span className={`badge ${gateClass(queuedExecution.gate_state)}`}>
                {shortGate(queuedExecution.gate_state)}
              </span>
            </Row>
            <Row label="Queued Height">
              {fmtNum(queuedExecution.queued_height)}
            </Row>
            <Row label="Earliest Exec">
              {fmtNum(queuedExecution.earliest_exec_height)}
            </Row>
            <Row label="Threshold BPS">
              {bpsToPercent(queuedExecution.threshold_bps)}
            </Row>
            <Row label="2nd Confirm Required">
              {queuedExecution.requires_second_confirm ? 'Yes' : 'No'}
            </Row>
            <Row label="2nd Confirm Received">
              {queuedExecution.second_confirm_received ? 'Yes' : 'No'}
            </Row>
            {queuedExecution.status_notes && (
              <Row label="Notes">{queuedExecution.status_notes}</Row>
            )}
          </div>
          <JsonViewer label="Raw Queued Execution" data={queuedExecution} />
        </div>
      )}
    </section>
  );
}

function Row({ label, children }) {
  return (
    <>
      <dt>{label}</dt>
      <dd>{children}</dd>
    </>
  );
}
