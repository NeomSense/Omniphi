import { Card } from '../ui/Card';
import { Badge } from '../ui/Badge';
import { StatCard } from '../ui/StatCard';
import { ValidatorStatus } from '../../types/validator';
import { formatDistance } from 'date-fns';

interface ValidatorStatusCardProps {
  status: ValidatorStatus | null;
  loading?: boolean;
}

export function ValidatorStatusCard({ status, loading }: ValidatorStatusCardProps) {
  if (loading || !status) {
    return (
      <Card title="Validator Status">
        <div className="animate-pulse space-y-4">
          <div className="h-24 bg-gray-200 rounded"></div>
          <div className="h-24 bg-gray-200 rounded"></div>
        </div>
      </Card>
    );
  }

  const getSyncBadge = () => {
    if (!status.running) return <Badge variant="error">Stopped</Badge>;
    if (status.syncing) return <Badge variant="warning">Syncing</Badge>;
    return <Badge variant="success">Synced</Badge>;
  };

  const getJailBadge = () => {
    if (status.jailed) return <Badge variant="error">Jailed</Badge>;
    if (status.is_active) return <Badge variant="success">Active</Badge>;
    return <Badge variant="warning">Inactive</Badge>;
  };

  return (
    <div className="space-y-4">
      {/* Hero Card */}
      <Card dark className="bg-gradient-omniphi">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-2xl font-bold text-white mb-1">
              {status.moniker}
            </h2>
            <p className="text-omniphi-200">
              {status.chain_id}
            </p>
          </div>
          <div className="flex flex-col items-end space-y-2">
            {getSyncBadge()}
            {getJailBadge()}
          </div>
        </div>

        <div className="mt-6 grid grid-cols-2 gap-4">
          <div>
            <p className="text-omniphi-200 text-sm">Block Height</p>
            <p className="text-4xl font-bold text-white">
              {status.block_height.toLocaleString()}
            </p>
          </div>
          <div>
            <p className="text-omniphi-200 text-sm">Peers</p>
            <p className="text-4xl font-bold text-white">
              {status.peers}
            </p>
          </div>
        </div>

        {status.uptime > 0 && (
          <div className="mt-4 pt-4 border-t border-omniphi-400">
            <p className="text-omniphi-200 text-sm">
              Uptime: {formatDistance(0, status.uptime * 1000)}
            </p>
          </div>
        )}
      </Card>

      {/* Stats Grid */}
      <div className="grid grid-cols-3 gap-4">
        <StatCard
          label="Missed Blocks"
          value={status.missed_blocks}
          variant={status.missed_blocks > 10 ? 'default' : 'gradient'}
        />

        <StatCard
          label="Network"
          value={status.network_id || 'N/A'}
          subValue="Network ID"
        />

        <StatCard
          label="Last Signature"
          value={status.last_signature
            ? formatDistance(new Date(status.last_signature), new Date(), { addSuffix: true })
            : 'Never'
          }
        />
      </div>
    </div>
  );
}
