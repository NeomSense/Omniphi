import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { formatDistanceToNow } from 'date-fns';
import {
  getLatestBlock,
  getRecentBlocks,
  getValidators,
  getNodeInfo,
  getSupply,
  getStakingPool,
  formatAmount,
  truncateHash,
} from '@/lib/api';

export default function Home() {
  const { data: latestBlock } = useQuery({
    queryKey: ['latestBlock'],
    queryFn: getLatestBlock,
    refetchInterval: 5000,
  });

  const { data: blocks = [] } = useQuery({
    queryKey: ['recentBlocks'],
    queryFn: () => getRecentBlocks(6),
    refetchInterval: 10000,
  });

  const { data: validators = [] } = useQuery({
    queryKey: ['validators'],
    queryFn: getValidators,
  });

  const { data: nodeInfo } = useQuery({
    queryKey: ['nodeInfo'],
    queryFn: getNodeInfo,
  });

  const { data: supply } = useQuery({
    queryKey: ['supply'],
    queryFn: getSupply,
  });

  const { data: pool } = useQuery({
    queryKey: ['stakingPool'],
    queryFn: getStakingPool,
  });

  const totalSupply = supply?.supply?.find((s) => s.denom === 'omniphi')?.amount || '0';
  const bondedTokens = pool?.pool?.bonded_tokens || '0';
  const stakingRatio = totalSupply !== '0' ? (BigInt(bondedTokens) * 100n / BigInt(totalSupply)) : 0n;

  return (
    <div className="space-y-8">
      {/* Network Stats */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <StatCard
          label="Block Height"
          value={latestBlock?.block.header.height || '-'}
          icon="cube"
        />
        <StatCard
          label="Validators"
          value={validators.length.toString()}
          icon="users"
        />
        <StatCard
          label="Total Supply"
          value={`${formatAmount(totalSupply)} OMNI`}
          icon="coins"
        />
        <StatCard
          label="Staking Ratio"
          value={`${stakingRatio.toString()}%`}
          icon="chart"
        />
      </div>

      {/* Network Info */}
      {nodeInfo && (
        <div className="card p-6">
          <h2 className="text-lg font-semibold mb-4">Network Information</h2>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 text-sm">
            <div>
              <p className="text-dark-400">Chain ID</p>
              <p className="font-mono text-omniphi-400">{nodeInfo.default_node_info.network}</p>
            </div>
            <div>
              <p className="text-dark-400">Node Version</p>
              <p className="font-mono">{nodeInfo.default_node_info.version}</p>
            </div>
            <div>
              <p className="text-dark-400">App Version</p>
              <p className="font-mono">{nodeInfo.application_version.version}</p>
            </div>
            <div>
              <p className="text-dark-400">Moniker</p>
              <p className="font-mono">{nodeInfo.default_node_info.moniker}</p>
            </div>
          </div>
        </div>
      )}

      {/* Recent Blocks */}
      <div className="card">
        <div className="p-4 border-b border-dark-700 flex items-center justify-between">
          <h2 className="text-lg font-semibold">Recent Blocks</h2>
          <Link to="/blocks" className="text-sm text-omniphi-400 hover:text-omniphi-300">
            View All →
          </Link>
        </div>
        <div className="divide-y divide-dark-700">
          {blocks.map((block) => (
            <div key={block.block.header.height} className="p-4 table-row">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-4">
                  <div className="w-10 h-10 bg-omniphi-500/20 rounded-lg flex items-center justify-center">
                    <svg className="w-5 h-5 text-omniphi-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
                    </svg>
                  </div>
                  <div>
                    <Link to={`/block/${block.block.header.height}`} className="font-mono text-omniphi-400 hover:text-omniphi-300">
                      #{block.block.header.height}
                    </Link>
                    <p className="text-sm text-dark-400">
                      {block.block.data.txs?.length || 0} transactions
                    </p>
                  </div>
                </div>
                <div className="text-right">
                  <p className="font-mono text-sm text-dark-300">
                    {truncateHash(block.block_id.hash)}
                  </p>
                  <p className="text-sm text-dark-500">
                    {formatDistanceToNow(new Date(block.block.header.time), { addSuffix: true })}
                  </p>
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>

      {/* Validators */}
      <div className="card">
        <div className="p-4 border-b border-dark-700 flex items-center justify-between">
          <h2 className="text-lg font-semibold">Active Validators</h2>
          <Link to="/validators" className="text-sm text-omniphi-400 hover:text-omniphi-300">
            View All →
          </Link>
        </div>
        <div className="divide-y divide-dark-700">
          {validators.slice(0, 5).map((validator, index) => (
            <div key={validator.operator_address} className="p-4 table-row">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-4">
                  <div className="w-8 h-8 bg-dark-700 rounded-full flex items-center justify-center text-sm font-bold">
                    {index + 1}
                  </div>
                  <div>
                    <Link
                      to={`/validator/${validator.operator_address}`}
                      className="font-medium text-dark-100 hover:text-omniphi-400"
                    >
                      {validator.description.moniker}
                    </Link>
                    <p className="text-sm text-dark-500">
                      {(parseFloat(validator.commission.commission_rates.rate) * 100).toFixed(1)}% commission
                    </p>
                  </div>
                </div>
                <div className="text-right">
                  <p className="font-mono text-dark-200">
                    {formatAmount(validator.tokens)} OMNI
                  </p>
                  <p className="text-sm text-dark-500">Voting Power</p>
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

function StatCard({ label, value, icon }: { label: string; value: string; icon: string }) {
  const iconMap: Record<string, React.ReactNode> = {
    cube: (
      <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
      </svg>
    ),
    users: (
      <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197m13.5-9a2.5 2.5 0 11-5 0 2.5 2.5 0 015 0z" />
      </svg>
    ),
    coins: (
      <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
    ),
    chart: (
      <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
      </svg>
    ),
  };

  return (
    <div className="card p-4">
      <div className="flex items-center gap-3">
        <div className="w-12 h-12 bg-omniphi-500/20 rounded-xl flex items-center justify-center text-omniphi-400">
          {iconMap[icon]}
        </div>
        <div>
          <p className="text-sm text-dark-400">{label}</p>
          <p className="text-xl font-bold text-dark-100">{value}</p>
        </div>
      </div>
    </div>
  );
}
