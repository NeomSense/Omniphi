import { useCallback } from 'react';
import { fetchProposals } from '../api/gov';
import { fetchGuardStatus } from '../api/guard';
import { usePolling } from '../utils/polling';
import ProposalList from '../components/ProposalList';
import Loading from '../components/Loading';
import ErrorBanner from '../components/ErrorBanner';

/**
 * Home page — lists all proposals with guard tier/gate columns.
 */
export default function Home() {
  const fetchAll = useCallback(async () => {
    const data = await fetchProposals({ limit: 50, reverse: true });
    const proposals = data.proposals || [];

    // Fetch guard data for each proposal in parallel.
    // Failures are swallowed per-proposal so the list still renders.
    const guardEntries = await Promise.all(
      proposals.map(async (p) => {
        try {
          const guard = await fetchGuardStatus(p.id);
          return [p.id, guard];
        } catch {
          return [p.id, null];
        }
      }),
    );

    const guardMap = Object.fromEntries(guardEntries);
    return { proposals, guardMap };
  }, []);

  const { data, error, loading, refresh } = usePolling(fetchAll);

  if (loading && !data) return <Loading message="Fetching proposals…" />;

  return (
    <div className="page-home">
      <div className="page-header">
        <h2>Governance Proposals</h2>
        <button className="btn btn-sm" onClick={refresh}>
          Refresh
        </button>
      </div>

      <ErrorBanner error={error} onRetry={refresh} />

      <ProposalList
        proposals={data?.proposals}
        guardMap={data?.guardMap}
      />
    </div>
  );
}
