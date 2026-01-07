import { useQuery } from '@tanstack/react-query';
import { useParams, Link } from 'react-router-dom';
import { getTx } from '@/lib/api';

export default function TxDetail() {
  const { hash } = useParams<{ hash: string }>();

  const { data: tx, isLoading, error } = useQuery({
    queryKey: ['tx', hash],
    queryFn: () => getTx(hash!),
    enabled: !!hash,
  });

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="w-8 h-8 border-4 border-omniphi-500 border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  if (error || !tx) {
    return (
      <div className="card p-8 text-center">
        <h2 className="text-xl font-semibold text-red-400 mb-2">Transaction Not Found</h2>
        <p className="text-dark-400">Transaction with hash {hash} could not be found.</p>
        <Link to="/blocks" className="btn-primary inline-block mt-4">
          Browse Blocks
        </Link>
      </div>
    );
  }

  const isSuccess = tx.tx_result?.code === 0;

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/blocks" className="text-dark-400 hover:text-dark-200">
          ← Blocks
        </Link>
        <h1 className="text-2xl font-bold">Transaction Details</h1>
      </div>

      <div className="card">
        <div className="p-4 border-b border-dark-700 flex items-center justify-between">
          <h2 className="font-semibold">Overview</h2>
          {isSuccess ? (
            <span className="px-3 py-1 status-success rounded-full text-sm">Success</span>
          ) : (
            <span className="px-3 py-1 status-error rounded-full text-sm">Failed</span>
          )}
        </div>
        <div className="p-4 space-y-4">
          <DetailRow label="Transaction Hash" value={tx.txhash} mono copyable />
          <DetailRow
            label="Block Height"
            value={
              <Link to={`/block/${tx.height}`} className="text-omniphi-400 hover:text-omniphi-300">
                #{tx.height}
              </Link>
            }
          />
          <DetailRow label="Gas Wanted" value={tx.tx_result?.gas_wanted || '0'} />
          <DetailRow label="Gas Used" value={tx.tx_result?.gas_used || '0'} />
          {tx.tx_result?.log && (
            <DetailRow label="Log" value={tx.tx_result.log} />
          )}
        </div>
      </div>

      {/* Events */}
      {tx.tx_result?.events && tx.tx_result.events.length > 0 && (
        <div className="card">
          <div className="p-4 border-b border-dark-700">
            <h2 className="font-semibold">Events ({tx.tx_result.events.length})</h2>
          </div>
          <div className="divide-y divide-dark-700">
            {tx.tx_result.events.map((event, index) => (
              <div key={index} className="p-4">
                <p className="font-medium text-omniphi-400 mb-2">{event.type}</p>
                <div className="space-y-1">
                  {event.attributes.map((attr, attrIndex) => (
                    <div key={attrIndex} className="flex gap-4 text-sm">
                      <span className="text-dark-400 w-32">{attr.key}</span>
                      <span className="text-dark-200 font-mono break-all">{attr.value}</span>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function DetailRow({
  label,
  value,
  mono = false,
  copyable = false,
}: {
  label: string;
  value: React.ReactNode;
  mono?: boolean;
  copyable?: boolean;
}) {
  const handleCopy = () => {
    if (typeof value === 'string') {
      navigator.clipboard.writeText(value);
    }
  };

  return (
    <div className="flex flex-col sm:flex-row sm:items-start gap-1 sm:gap-4">
      <span className="text-dark-400 sm:w-32 flex-shrink-0">{label}</span>
      <div className="flex items-start gap-2 overflow-hidden flex-1">
        <span className={`${mono ? 'font-mono text-sm' : ''} text-dark-100 break-all`}>
          {value}
        </span>
        {copyable && (
          <button
            onClick={handleCopy}
            className="p-1 text-dark-400 hover:text-omniphi-400 flex-shrink-0"
            title="Copy"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
            </svg>
          </button>
        )}
      </div>
    </div>
  );
}
