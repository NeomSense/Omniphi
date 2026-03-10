import ProposalRow from './ProposalRow';

/**
 * Tabular list of proposals with guard data.
 *
 * Props:
 *  - proposals: array of gov proposal objects
 *  - guardMap: { [proposalId]: { riskReport, queuedExecution } }
 */
export default function ProposalList({ proposals, guardMap }) {
  if (!proposals || proposals.length === 0) {
    return <p className="empty">No proposals found.</p>;
  }

  return (
    <div className="table-wrap">
      <table className="proposal-table">
        <thead>
          <tr>
            <th>ID</th>
            <th>Title</th>
            <th>Status</th>
            <th>Updated</th>
            <th>Tier</th>
            <th>Gate</th>
          </tr>
        </thead>
        <tbody>
          {proposals.map((p) => (
            <ProposalRow
              key={p.id}
              proposal={p}
              guard={guardMap?.[p.id] ?? null}
            />
          ))}
        </tbody>
      </table>
    </div>
  );
}
