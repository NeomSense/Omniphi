import { useState, useEffect, useCallback } from 'react';
import { truncHash } from '../utils/format';
import { sha256Hex, normalizeHash } from '../utils/hash';
import { normalizeReportUri, fetchReportBytes } from '../utils/uri';
import JsonViewer from './JsonViewer';

/**
 * Advisory link panel with automatic SHA-256 hash verification.
 *
 * When an on-chain advisory link exists with a URI + report_hash,
 * this component fetches the report, hashes it in-browser via
 * Web Crypto, and shows whether the hash matches on-chain.
 *
 * Props:
 *  - advisory: advisory link object or null
 */
export default function AdvisoryPanel({ advisory }) {
  const [status, setStatus] = useState('idle');
  const [computedHash, setComputedHash] = useState(null);
  const [errorMessage, setErrorMessage] = useState(null);
  const [reportJson, setReportJson] = useState(null);
  const [fetchedAt, setFetchedAt] = useState(null);

  const uri = advisory?.uri || advisory?.report_uri;
  const onChainHash = advisory?.report_hash || advisory?.hash;
  const reporter = advisory?.reporter;
  const height = advisory?.height || advisory?.creation_height;

  const verify = useCallback(async () => {
    if (!uri || !onChainHash) return;

    setStatus('verifying');
    setComputedHash(null);
    setErrorMessage(null);
    setReportJson(null);

    const resolved = normalizeReportUri(uri);

    // Non-fetchable scheme
    if (!resolved.fetchUrl) {
      setStatus('unsupported');
      setErrorMessage(resolved.note);
      return;
    }

    try {
      const bytes = await fetchReportBytes(resolved.fetchUrl);
      const hex = await sha256Hex(bytes);
      setComputedHash(hex);
      setFetchedAt(new Date().toLocaleTimeString());

      // Try to parse as JSON for preview
      try {
        const text = new TextDecoder().decode(bytes);
        setReportJson(JSON.parse(text));
      } catch {
        // Not valid JSON — that's fine, hash still applies
      }

      const expected = normalizeHash(onChainHash);
      if (hex === expected) {
        setStatus('verified');
      } else {
        setStatus('mismatch');
      }
    } catch (err) {
      setStatus('unreachable');
      setErrorMessage(err.message || 'Failed to fetch report.');
    }
  }, [uri, onChainHash]);

  // Auto-verify on mount and when URI changes
  useEffect(() => {
    if (uri && onChainHash) {
      verify();
    }
  }, [verify]);

  // --- No advisory at all ---
  if (!advisory) {
    return (
      <section className="panel panel-advisory">
        <h3>Copilot Advisory</h3>
        <p className="muted">No advisory report has been submitted for this proposal.</p>
      </section>
    );
  }

  const resolved = normalizeReportUri(uri);
  const linkable = resolved.fetchUrl || (resolved.kind === 'http' || resolved.kind === 'https');

  return (
    <section className="panel panel-advisory">
      <h3>
        Copilot Advisory
        {onChainHash && <VerifyBadge status={status} />}
      </h3>

      <div className="kv-grid">
        <dt>Report URI</dt>
        <dd>
          {linkable && resolved.fetchUrl ? (
            <a href={resolved.fetchUrl} target="_blank" rel="noopener noreferrer" className="advisory-link">
              {uri}
            </a>
          ) : (
            <span>{uri || '—'}</span>
          )}
          {resolved.kind === 'ipfs' && (
            <span className="badge badge-ipfs">IPFS</span>
          )}
        </dd>

        <dt>On-chain Hash</dt>
        <dd title={onChainHash}>
          <code>{truncHash(onChainHash)}</code>
        </dd>

        {computedHash && (
          <>
            <dt>Computed Hash</dt>
            <dd title={computedHash}>
              <code className={status === 'mismatch' ? 'hash-mismatch' : 'hash-match'}>
                {truncHash(computedHash)}
              </code>
            </dd>
          </>
        )}

        <dt>Reporter</dt>
        <dd>
          <code className="address">{reporter || '—'}</code>
        </dd>

        {height && (
          <>
            <dt>Submitted Height</dt>
            <dd>{Number(height).toLocaleString()}</dd>
          </>
        )}

        {fetchedAt && (
          <>
            <dt>Verified At</dt>
            <dd>{fetchedAt}</dd>
          </>
        )}
      </div>

      {/* Status detail messages */}
      {status === 'mismatch' && (
        <div className="verify-alert verify-alert-danger">
          Hash mismatch — the fetched report does not match the on-chain hash.
          The report may have been modified after submission.
        </div>
      )}

      {status === 'unreachable' && (
        <div className="verify-alert verify-alert-warn">
          <strong>Could not fetch report.</strong> {errorMessage}
          <ul className="verify-causes">
            <li>CORS may not be enabled on the report server</li>
            <li>The URI may not be publicly accessible</li>
            <li>IPFS gateway may be down or slow</li>
          </ul>
        </div>
      )}

      {status === 'unsupported' && (
        <div className="verify-alert verify-alert-info">
          {errorMessage}
        </div>
      )}

      {/* Retry button */}
      {(status === 'unreachable' || status === 'mismatch') && (
        <button className="btn btn-sm" onClick={verify} style={{ marginTop: 8 }}>
          Retry verification
        </button>
      )}

      {/* No hash → can't verify, show manual instructions */}
      {uri && !onChainHash && (
        <div className="verify-alert verify-alert-info">
          No report hash was submitted on-chain. Automatic verification is not possible.
        </div>
      )}

      {/* Report preview */}
      {reportJson && (
        <JsonViewer label="Report Preview" data={reportJson} />
      )}

      <JsonViewer label="Raw Advisory Link" data={advisory} />
    </section>
  );
}

/**
 * Verification status badge.
 */
function VerifyBadge({ status }) {
  switch (status) {
    case 'verifying':
      return <span className="badge verify-badge verify-verifying">Verifying...</span>;
    case 'verified':
      return <span className="badge verify-badge verify-verified">Verified</span>;
    case 'mismatch':
      return <span className="badge verify-badge verify-mismatch">Mismatch</span>;
    case 'unreachable':
      return <span className="badge verify-badge verify-unreachable">Unreachable</span>;
    case 'unsupported':
      return <span className="badge verify-badge verify-unsupported">Unsupported</span>;
    default:
      return null;
  }
}
