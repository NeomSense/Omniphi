/**
 * Validator Nodes Page
 */

import { useEffect, useState } from 'react';
import { formatDistanceToNow } from 'date-fns';
import {
  Search,
  RefreshCw,
  Play,
  Square,
  ScrollText,
  ExternalLink,
  CheckCircle,
  XCircle,
  AlertTriangle,
  Server,
  Cpu,
  Database,
  HardDrive,
  Wifi,
  Activity,
  ChevronLeft,
  ChevronRight,
} from 'lucide-react';
import { api } from '../services/api';
import type { ValidatorNode, NodeStatus } from '../types';

const statusConfig: Record<NodeStatus, { color: string; icon: typeof Activity }> = {
  running: { color: 'badge-success', icon: CheckCircle },
  stopped: { color: 'badge-neutral', icon: Square },
  error: { color: 'badge-error', icon: XCircle },
  starting: { color: 'badge-info', icon: RefreshCw },
  stopping: { color: 'badge-warning', icon: AlertTriangle },
};

export function NodesPage() {
  const [nodes, setNodes] = useState<ValidatorNode[]>([]);
  const [loading, setLoading] = useState(true);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [providerFilter, setProviderFilter] = useState('');
  const [actionLoading, setActionLoading] = useState<string | null>(null);

  const pageSize = 12;

  const fetchNodes = async () => {
    setLoading(true);
    const result = await api.nodes.list({
      page,
      pageSize,
      status: statusFilter || undefined,
      provider: providerFilter || undefined,
    });
    if (result.success && result.data) {
      setNodes(result.data.items);
      setTotal(result.data.total);
    }
    setLoading(false);
  };

  useEffect(() => {
    fetchNodes();
    const interval = setInterval(fetchNodes, 30000);
    return () => clearInterval(interval);
  }, [page, statusFilter, providerFilter]);

  const handleRestart = async (id: string) => {
    if (confirm('Are you sure you want to restart this node?')) {
      setActionLoading(id);
      await api.nodes.restart(id);
      await fetchNodes();
      setActionLoading(null);
    }
  };

  const handleStop = async (id: string) => {
    if (confirm('Are you sure you want to stop this node?')) {
      setActionLoading(id);
      await api.nodes.stop(id);
      await fetchNodes();
      setActionLoading(null);
    }
  };

  const formatUptime = (seconds: number) => {
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    if (days > 0) return `${days}d ${hours}h`;
    const mins = Math.floor((seconds % 3600) / 60);
    return `${hours}h ${mins}m`;
  };

  const totalPages = Math.ceil(total / pageSize);

  return (
    <div className="p-8">
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-white">Validator Nodes</h1>
          <p className="text-dark-400 mt-1">Monitor and manage running validator instances</p>
        </div>
        <button onClick={fetchNodes} className="btn btn-secondary">
          <RefreshCw className={`w-4 h-4 mr-2 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </button>
      </div>

      {/* Filters */}
      <div className="card mb-6">
        <div className="flex flex-wrap items-center gap-4">
          <div className="flex-1 min-w-[200px]">
            <div className="relative">
              <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-dark-400" />
              <input
                type="text"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Search by node ID..."
                className="input pl-10"
              />
            </div>
          </div>
          <select
            value={statusFilter}
            onChange={(e) => {
              setStatusFilter(e.target.value);
              setPage(1);
            }}
            className="select w-36"
          >
            <option value="">All Status</option>
            <option value="running">Running</option>
            <option value="stopped">Stopped</option>
            <option value="error">Error</option>
          </select>
          <select
            value={providerFilter}
            onChange={(e) => {
              setProviderFilter(e.target.value);
              setPage(1);
            }}
            className="select w-36"
          >
            <option value="">All Providers</option>
            <option value="aws">AWS</option>
            <option value="gcp">GCP</option>
            <option value="digitalocean">DigitalOcean</option>
            <option value="local">Local</option>
          </select>
        </div>
      </div>

      {/* Nodes Grid */}
      {loading ? (
        <div className="flex items-center justify-center py-12">
          <RefreshCw className="w-8 h-8 text-omniphi-500 animate-spin" />
        </div>
      ) : nodes.length === 0 ? (
        <div className="card text-center py-12">
          <Server className="w-12 h-12 text-dark-600 mx-auto mb-4" />
          <h3 className="text-lg font-medium text-white mb-2">No nodes found</h3>
          <p className="text-dark-400">No validator nodes match your filters</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {nodes.map((node) => {
            const StatusIcon = statusConfig[node.status].icon;
            const isLoading = actionLoading === node.id;

            return (
              <div key={node.id} className="card hover:border-dark-600 transition-colors">
                {/* Header */}
                <div className="flex items-center justify-between mb-4">
                  <div className="flex items-center space-x-3">
                    <div className={`w-10 h-10 rounded-lg flex items-center justify-center ${
                      node.status === 'running' ? 'bg-green-900/30' :
                      node.status === 'error' ? 'bg-red-900/30' : 'bg-dark-800'
                    }`}>
                      <Server className={`w-5 h-5 ${
                        node.status === 'running' ? 'text-green-400' :
                        node.status === 'error' ? 'text-red-400' : 'text-dark-400'
                      }`} />
                    </div>
                    <div>
                      <p className="font-mono text-sm text-white">{node.id}</p>
                      <p className="text-xs text-dark-400 capitalize">{node.provider}</p>
                    </div>
                  </div>
                  <span className={`badge ${statusConfig[node.status].color}`}>
                    <StatusIcon className={`w-3 h-3 mr-1 ${node.status === 'starting' ? 'animate-spin' : ''}`} />
                    {node.status}
                  </span>
                </div>

                {/* Metrics */}
                <div className="space-y-3 mb-4">
                  <MetricBar icon={<Cpu className="w-4 h-4" />} label="CPU" value={node.cpu_percent} />
                  <MetricBar icon={<Database className="w-4 h-4" />} label="RAM" value={node.ram_percent} detail={node.ram_used} />
                  <MetricBar icon={<HardDrive className="w-4 h-4" />} label="Disk" value={node.disk_percent} detail={node.disk_used} />
                </div>

                {/* Node Info */}
                <div className="grid grid-cols-2 gap-3 mb-4 text-sm">
                  <div className="bg-dark-800 p-2 rounded">
                    <p className="text-dark-400 text-xs">Block Height</p>
                    <p className="text-white font-mono">{node.block_height.toLocaleString()}</p>
                  </div>
                  <div className="bg-dark-800 p-2 rounded">
                    <p className="text-dark-400 text-xs">Peers</p>
                    <p className="text-white flex items-center">
                      <Wifi className="w-3 h-3 mr-1 text-green-400" />
                      {node.peers}
                    </p>
                  </div>
                  <div className="bg-dark-800 p-2 rounded">
                    <p className="text-dark-400 text-xs">Uptime</p>
                    <p className="text-white">{formatUptime(node.uptime)}</p>
                  </div>
                  <div className="bg-dark-800 p-2 rounded">
                    <p className="text-dark-400 text-xs">Syncing</p>
                    <p className={node.syncing ? 'text-yellow-400' : 'text-green-400'}>
                      {node.syncing ? 'Yes' : 'No'}
                    </p>
                  </div>
                </div>

                {/* Endpoints */}
                <div className="space-y-2 mb-4 text-xs">
                  <div className="flex items-center justify-between p-2 bg-dark-800 rounded">
                    <span className="text-dark-400">RPC</span>
                    <span className="font-mono text-dark-200 truncate max-w-[180px]">{node.rpc_endpoint}</span>
                  </div>
                  <div className="flex items-center justify-between p-2 bg-dark-800 rounded">
                    <span className="text-dark-400">P2P</span>
                    <span className="font-mono text-dark-200 truncate max-w-[180px]">{node.p2p_endpoint}</span>
                  </div>
                </div>

                {/* Actions */}
                <div className="flex items-center space-x-2 pt-4 border-t border-dark-700">
                  {node.status === 'running' ? (
                    <>
                      <button
                        onClick={() => handleRestart(node.id)}
                        disabled={isLoading}
                        className="btn btn-secondary btn-sm flex-1"
                      >
                        {isLoading ? (
                          <RefreshCw className="w-4 h-4 animate-spin" />
                        ) : (
                          <>
                            <RefreshCw className="w-4 h-4 mr-1" />
                            Restart
                          </>
                        )}
                      </button>
                      <button
                        onClick={() => handleStop(node.id)}
                        disabled={isLoading}
                        className="btn btn-danger btn-sm flex-1"
                      >
                        <Square className="w-4 h-4 mr-1" />
                        Stop
                      </button>
                    </>
                  ) : node.status === 'stopped' ? (
                    <button
                      onClick={() => handleRestart(node.id)}
                      disabled={isLoading}
                      className="btn btn-success btn-sm flex-1"
                    >
                      <Play className="w-4 h-4 mr-1" />
                      Start
                    </button>
                  ) : null}
                  <button className="btn btn-ghost btn-sm" title="View Logs">
                    <ScrollText className="w-4 h-4" />
                  </button>
                  {node.metrics_endpoint && (
                    <a
                      href={node.metrics_endpoint}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="btn btn-ghost btn-sm"
                      title="Open Metrics"
                    >
                      <ExternalLink className="w-4 h-4" />
                    </a>
                  )}
                </div>

                {/* Last Health Check */}
                <p className="text-xs text-dark-500 mt-3 text-center">
                  Last check: {formatDistanceToNow(new Date(node.last_health_check), { addSuffix: true })}
                </p>
              </div>
            );
          })}
        </div>
      )}

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between mt-6">
          <p className="text-sm text-dark-400">
            Showing {(page - 1) * pageSize + 1} to {Math.min(page * pageSize, total)} of {total}
          </p>
          <div className="flex items-center space-x-2">
            <button
              onClick={() => setPage(p => Math.max(1, p - 1))}
              disabled={page === 1}
              className="btn btn-secondary btn-sm"
            >
              <ChevronLeft className="w-4 h-4" />
            </button>
            <span className="text-sm text-dark-300">
              Page {page} of {totalPages}
            </span>
            <button
              onClick={() => setPage(p => Math.min(totalPages, p + 1))}
              disabled={page === totalPages}
              className="btn btn-secondary btn-sm"
            >
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

function MetricBar({
  icon,
  label,
  value,
  detail,
}: {
  icon: React.ReactNode;
  label: string;
  value: number;
  detail?: string;
}) {
  const getColor = (v: number) => {
    if (v > 90) return 'bg-red-500';
    if (v > 75) return 'bg-yellow-500';
    return 'bg-green-500';
  };

  return (
    <div>
      <div className="flex items-center justify-between mb-1">
        <div className="flex items-center space-x-2 text-dark-400">
          {icon}
          <span className="text-xs">{label}</span>
        </div>
        <span className="text-xs text-dark-300">
          {value.toFixed(1)}%
          {detail && <span className="text-dark-500 ml-1">({detail})</span>}
        </span>
      </div>
      <div className="w-full bg-dark-700 rounded-full h-1.5">
        <div
          className={`h-1.5 rounded-full transition-all duration-300 ${getColor(value)}`}
          style={{ width: `${Math.min(value, 100)}%` }}
        />
      </div>
    </div>
  );
}

export default NodesPage;
