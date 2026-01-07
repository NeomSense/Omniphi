import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { formatDistanceToNow } from 'date-fns';
import { getRecentBlocks, truncateHash } from '@/lib/api';

export default function Blocks() {
  const { data: blocks = [], isLoading } = useQuery({
    queryKey: ['recentBlocks', 20],
    queryFn: () => getRecentBlocks(20),
    refetchInterval: 10000,
  });

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-bold">Blocks</h1>
        <p className="text-dark-400">
          Latest {blocks.length} blocks
        </p>
      </div>

      <div className="card overflow-hidden">
        <table className="w-full">
          <thead className="bg-dark-800">
            <tr className="text-left text-sm text-dark-400">
              <th className="px-4 py-3 font-medium">Height</th>
              <th className="px-4 py-3 font-medium">Hash</th>
              <th className="px-4 py-3 font-medium">Txs</th>
              <th className="px-4 py-3 font-medium">Proposer</th>
              <th className="px-4 py-3 font-medium text-right">Time</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-dark-700">
            {isLoading ? (
              <tr>
                <td colSpan={5} className="px-4 py-8 text-center text-dark-400">
                  Loading blocks...
                </td>
              </tr>
            ) : blocks.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-4 py-8 text-center text-dark-400">
                  No blocks found
                </td>
              </tr>
            ) : (
              blocks.map((block) => (
                <tr key={block.block.header.height} className="table-row">
                  <td className="px-4 py-3">
                    <Link
                      to={`/block/${block.block.header.height}`}
                      className="font-mono text-omniphi-400 hover:text-omniphi-300"
                    >
                      #{block.block.header.height}
                    </Link>
                  </td>
                  <td className="px-4 py-3">
                    <span className="font-mono text-sm text-dark-300">
                      {truncateHash(block.block_id.hash)}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <span className="px-2 py-1 bg-dark-700 rounded text-sm">
                      {block.block.data.txs?.length || 0}
                    </span>
                  </td>
                  <td className="px-4 py-3">
                    <span className="font-mono text-sm text-dark-400">
                      {truncateHash(block.block.header.proposer_address, 8, 4)}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-right text-sm text-dark-400">
                    {formatDistanceToNow(new Date(block.block.header.time), { addSuffix: true })}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
