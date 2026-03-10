import { Link } from 'react-router-dom';
import { shortStatus, shortTier, shortGate, fmtDate } from '../utils/format';
import { tierClass, gateClass } from '../utils/gates';

/**
 * A single row in the proposal list table.
 *
 * Props:
 *  - proposal: gov proposal object
 *  - guard: { riskReport, queuedExecution } | null
 */
export default function ProposalRow({ proposal, guard }) {
  const id = proposal.id;
  const title =
    proposal.title ||
    proposal.content?.title ||
    proposal.messages?.[0]?.content?.title ||
    `Proposal #${id}`;
  const status = shortStatus(proposal.status);
  const updated = fmtDate(proposal.voting_end_time || proposal.submit_time);

  const tierFinal = guard?.riskReport?.tier_final;
  const currentGate = guard?.queuedExecution?.gate_state;

  return (
    <tr className="proposal-row">
      <td className="col-id">
        <Link to={`/proposal/${id}`}>#{id}</Link>
      </td>
      <td className="col-title">
        <Link to={`/proposal/${id}`}>{title}</Link>
      </td>
      <td>
        <span className={`badge badge-status`}>{status}</span>
      </td>
      <td className="col-date">{updated}</td>
      <td>
        {tierFinal ? (
          <span className={`badge ${tierClass(tierFinal)}`}>
            {shortTier(tierFinal)}
          </span>
        ) : (
          <span className="muted">—</span>
        )}
      </td>
      <td>
        {currentGate ? (
          <span className={`badge ${gateClass(currentGate)}`}>
            {shortGate(currentGate)}
          </span>
        ) : (
          <span className="muted">—</span>
        )}
      </td>
    </tr>
  );
}
