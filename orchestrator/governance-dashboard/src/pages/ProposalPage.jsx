import { useCallback, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { fetchProposalById, fetchLatestHeight } from '../api/gov';
import { fetchGuardStatus, fetchAdvisoryLink, confirmExecution } from '../api/guard';
import { usePolling } from '../utils/polling';
import { shortStatus, fmtDate } from '../utils/format';
import GuardPanel from '../components/GuardPanel';
import Timeline from '../components/Timeline';
import AdvisoryPanel from '../components/AdvisoryPanel';
import JsonViewer from '../components/JsonViewer';
import Loading from '../components/Loading';
import ErrorBanner from '../components/ErrorBanner';

export default function ProposalPage() {
  const { id } = useParams();

  const fetchAll = useCallback(async () => {
    const [proposal, guard, advisory, height] = await Promise.all([
      fetchProposalById(id),
      fetchGuardStatus(id).catch(() => ({ riskReport: null, queuedExecution: null })),
      fetchAdvisoryLink(id).catch(() => null),
      fetchLatestHeight().catch(() => 0),
    ]);
    return { proposal, guard, advisory, height };
  }, [id]);

  const { data, error, loading, refresh } = usePolling(fetchAll, [id]);

  if (loading && !data) return <Loading message={`Loading proposal #${id}…`} />;

  const proposal = data?.proposal;
  const guard = data?.guard;
  const advisory = data?.advisory;
  const height = data?.height || 0;

  if (!proposal) {
    return (
      <div className="page-proposal">
        <Link to="/" className="back-link">← Back</Link>
        <ErrorBanner error={error || new Error(`Proposal #${id} not found`)} onRetry={refresh} />
      </div>
    );
  }

  const title =
    proposal.title ||
    proposal.content?.title ||
    proposal.messages?.[0]?.content?.title ||
    `Proposal #${id}`;

  const qe = guard?.queuedExecution;
  const showConfirm =
    qe &&
    (qe.gate_state || '').includes('READY') &&
    qe.requires_second_confirm &&
    !qe.second_confirm_received;

  return (
    <div className="page-proposal">
      <Link to="/" className="back-link">← Back to proposals</Link>

      <ErrorBanner error={error} onRetry={refresh} />

      <div className="proposal-header">
        <h2>
          <span className="proposal-id">#{id}</span> {title}
        </h2>
        <div className="proposal-meta">
          <span className={`badge badge-status`}>{shortStatus(proposal.status)}</span>
          <span className="meta-item">Submitted: {fmtDate(proposal.submit_time)}</span>
          {proposal.voting_end_time && (
            <span className="meta-item">Voting ends: {fmtDate(proposal.voting_end_time)}</span>
          )}
        </div>
      </div>

      {proposal.summary && (
        <section className="panel">
          <h3>Summary</h3>
          <p className="proposal-summary">{proposal.summary}</p>
        </section>
      )}

      {proposal.messages && proposal.messages.length > 0 && (
        <section className="panel">
          <h3>Messages ({proposal.messages.length})</h3>
          <JsonViewer label="Proposal Messages" data={proposal.messages} />
        </section>
      )}

      <Timeline
        currentGate={qe?.gate_state}
        earliestExecHeight={qe?.earliest_exec_height}
        currentHeight={height}
        statusNotes={qe?.status_notes}
      />

      <div className="detail-grid">
        <GuardPanel
          riskReport={guard?.riskReport}
          queuedExecution={guard?.queuedExecution}
        />
        <AdvisoryPanel advisory={advisory} />
      </div>

      {showConfirm && <ConfirmButton proposalId={id} onDone={refresh} />}

      <JsonViewer label="Raw Proposal (gov)" data={proposal} />
    </div>
  );
}

function ConfirmButton({ proposalId, onDone }) {
  const [state, setState] = useState('idle'); // idle | loading | success | error | no-backend
  const [msg, setMsg] = useState('');
  const [txHash, setTxHash] = useState('');
  const backendUrl = import.meta.env.VITE_BACKEND_ACTIONS_URL;

  async function handleConfirm() {
    if (!backendUrl) {
      setState('no-backend');
      return;
    }
    setState('loading');
    setMsg('');
    setTxHash('');
    try {
      const resp = await confirmExecution(proposalId);
      if (resp.result === 'already_confirmed') {
        setState('success');
        setMsg(resp.message || 'Already confirmed.');
      } else if (resp.result === 'submitted') {
        setState('success');
        setMsg(resp.message || 'Execution confirmed.');
        setTxHash(resp.tx?.txhash || '');
      } else {
        setState('error');
        setMsg(resp.message || 'Unexpected response.');
      }
      onDone?.();
    } catch (err) {
      setState('error');
      // Try to parse JSON error body from proxy
      try {
        const parsed = JSON.parse(err.message);
        setMsg(parsed.message || err.message);
      } catch {
        setMsg(err.message || 'Confirm execution failed.');
      }
    }
  }

  return (
    <section className="panel panel-confirm">
      <h3>Second Confirmation Required</h3>
      <p>
        This CRITICAL proposal is at the READY gate and requires a second
        confirmation before it can be executed.
      </p>

      {state === 'no-backend' && (
        <div className="error-banner">
          Actions proxy not configured. Set <code>VITE_BACKEND_ACTIONS_URL</code> in
          your <code>.env</code> file (e.g. <code>http://localhost:8090</code>).
        </div>
      )}

      {state === 'success' && (
        <div className="success-banner">
          {msg}
          {txHash && (
            <div className="tx-hash">TxHash: <code>{txHash}</code></div>
          )}
        </div>
      )}
      {state === 'error' && <div className="error-banner">{msg}</div>}

      <button
        className="btn btn-confirm"
        onClick={handleConfirm}
        disabled={state === 'loading' || state === 'success'}
      >
        {state === 'loading' ? 'Confirming…' : 'Confirm Execution'}
      </button>
    </section>
  );
}
