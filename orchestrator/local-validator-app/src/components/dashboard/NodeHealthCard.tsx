import { Card } from '../ui/Card';
import { NodeHealth } from '../../types/validator';

interface NodeHealthCardProps {
  health: NodeHealth | null;
}

export function NodeHealthCard({ health }: NodeHealthCardProps) {
  if (!health) {
    return <Card title="Node Health"><p className="text-gray-500">No data available</p></Card>;
  }

  const getHealthColor = (percent: number) => {
    if (percent > 90) return 'text-red-600';
    if (percent > 75) return 'text-yellow-600';
    return 'text-green-600';
  };

  return (
    <Card title="Node Health Metrics">
      <div className="space-y-4">
        {/* CPU */}
        <div>
          <div className="flex justify-between mb-2">
            <span className="text-sm font-medium text-gray-700">CPU Usage</span>
            <span className={`text-sm font-bold ${getHealthColor(health.cpu)}`}>
              {health.cpu.toFixed(1)}%
            </span>
          </div>
          <div className="w-full bg-gray-200 rounded-full h-2">
            <div
              className="bg-omniphi-600 h-2 rounded-full transition-all duration-300"
              style={{ width: `${Math.min(health.cpu, 100)}%` }}
            />
          </div>
        </div>

        {/* RAM */}
        <div>
          <div className="flex justify-between mb-2">
            <span className="text-sm font-medium text-gray-700">RAM Usage</span>
            <span className={`text-sm font-bold ${getHealthColor(health.ram_percent)}`}>
              {health.ram}
            </span>
          </div>
          <div className="w-full bg-gray-200 rounded-full h-2">
            <div
              className="bg-omniphi-600 h-2 rounded-full transition-all duration-300"
              style={{ width: `${Math.min(health.ram_percent, 100)}%` }}
            />
          </div>
        </div>

        {/* Disk */}
        <div>
          <div className="flex justify-between mb-2">
            <span className="text-sm font-medium text-gray-700">Disk Usage</span>
            <span className={`text-sm font-bold ${getHealthColor(health.disk_percent)}`}>
              {health.disk}
            </span>
          </div>
          <div className="w-full bg-gray-200 rounded-full h-2">
            <div
              className="bg-omniphi-600 h-2 rounded-full transition-all duration-300"
              style={{ width: `${Math.min(health.disk_percent, 100)}%` }}
            />
          </div>
        </div>

        {/* Network & Ports */}
        <div className="grid grid-cols-2 gap-4 pt-4 border-t">
          <div>
            <p className="text-xs text-gray-500">Network In</p>
            <p className="text-sm font-semibold text-gray-900">{health.net_in}</p>
          </div>
          <div>
            <p className="text-xs text-gray-500">Network Out</p>
            <p className="text-sm font-semibold text-gray-900">{health.net_out}</p>
          </div>
          <div>
            <p className="text-xs text-gray-500">RPC Port</p>
            <p className="text-sm font-semibold text-gray-900">{health.rpc_port}</p>
          </div>
          <div>
            <p className="text-xs text-gray-500">P2P Port</p>
            <p className="text-sm font-semibold text-gray-900">{health.p2p_port}</p>
          </div>
        </div>

        {/* Node ID */}
        <div className="pt-2">
          <p className="text-xs text-gray-500">Node ID</p>
          <p className="text-xs font-mono text-gray-700 break-all">{health.node_id}</p>
        </div>
      </div>
    </Card>
  );
}
