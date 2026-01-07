import { useQuery } from '@tanstack/react-query';
import { useParams, Link } from 'react-router-dom';
import { format } from 'date-fns';
import { getBlock, truncateHash } from '@/lib/api';

export default function BlockDetail() {
  const { height } = useParams<{ height: string }>();

  const { data: block, isLoading, error } = useQuery({
    queryKey: ['block', height],
    queryFn: () => getBlock(height!),
    enabled: !!height,
  });

  if (isLoading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <div className="w-8 h-8 border-4 border-omniphi-500 border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  if (error || !block) {
    return (
      <div className="card p-8 text-center">
        <h2 className="text-xl font-semibold text-red-400 mb-2">Block Not Found</h2>
        <p className="text-dark-400">Block #{height} could not be found.</p>
        <Link to="/blocks" className="btn-primary inline-block mt-4">
          Back to Blocks
        </Link>
      </div>
    );
  }

  const txCount = block.block.data.txs?.length || 0;

  return (
    <div className="space-y-6">
      <div className="flex items-center gap-4">
        <Link to="/blocks" className="text-dark-400 hover:text-dark-200">
          ← Blocks
        </Link>
        <h1 className="text-2xl font-bold">Block #{height}</h1>
      </div>

      <div className="card">
        <div className="p-4 border-b border-dark-700">
          <h2 className="font-semibold">Block Overview</h2>
        </div>
        <div className="p-4 space-y-4">
          <DetailRow label="Height" value={block.block.header.height} mono />
          <DetailRow label="Hash" value={block.block_id.hash} mono copyable />
          <DetailRow label="Chain ID" value={block.block.header.chain_id} />
          <DetailRow
            label="Time"
            value={format(new Date(block.block.header.time), 'PPpp')}
          />
          <DetailRow label="Transactions" value={txCount.toString()} />
          <DetailRow
            label="Proposer"
            value={block.block.header.proposer_address}
            mono
          />
          <DetailRow
            label="Previous Block"
            value={
              <Link
                to={`/block/${parseInt(height!) - 1}`}
                className="text-omniphi-400 hover:text-omniphi-300 font-mono"
              >
                {truncateHash(block.block.header.last_block_id.hash)}
              </Link>
            }
          />
        </div>
      </div>

      {/* Transactions */}
      <div className="card">
        <div className="p-4 border-b border-dark-700">
          <h2 className="font-semibold">Transactions ({txCount})</h2>
        </div>
        {txCount === 0 ? (
          <div className="p-8 text-center text-dark-400">
            No transactions in this block
          </div>
        ) : (
          <div className="divide-y divide-dark-700">
            {block.block.data.txs?.map((tx, index) => (
              <div key={index} className="p-4 table-row">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className="w-8 h-8 bg-omniphi-500/20 rounded-lg flex items-center justify-center text-sm font-mono text-omniphi-400">
                      {index + 1}
                    </div>
                    <span className="font-mono text-sm text-dark-300">
                      {truncateHash(tx, 16, 8)}
                    </span>
                  </div>
                  <span className="px-2 py-1 status-success rounded text-xs">
                    Success
                  </span>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Navigation */}
      <div className="flex items-center justify-between">
        <Link
          to={`/block/${parseInt(height!) - 1}`}
          className="btn-primary"
        >
          ← Previous Block
        </Link>
        <Link
          to={`/block/${parseInt(height!) + 1}`}
          className="btn-primary"
        >
          Next Block →
        </Link>
      </div>
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
    <div className="flex flex-col sm:flex-row sm:items-center gap-1 sm:gap-4">
      <span className="text-dark-400 sm:w-32 flex-shrink-0">{label}</span>
      <div className="flex items-center gap-2 overflow-hidden">
        <span
          className={`${mono ? 'font-mono text-sm' : ''} text-dark-100 break-all`}
        >
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
