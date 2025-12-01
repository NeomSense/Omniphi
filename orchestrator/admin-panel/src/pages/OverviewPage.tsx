/**
 * Overview / System Health Page
 */

import { useEffect, useState } from 'react';
import {
  Server,
  Activity,
  Clock,
  AlertTriangle,
  CheckCircle,
  XCircle,
  TrendingUp,
  Cpu,
  HardDrive,
  Database,
  RefreshCw,
} from 'lucide-react';
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
} from 'recharts';
import { api } from '../services/api';
import type { SystemHealth, RecentError } from '../types';
import { formatDistanceToNow } from 'date-fns';

export function OverviewPage() {
  const [health, setHealth] = useState<SystemHealth | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);

  const fetchHealth = async () => {
    const result = await api.health.getSystemHealth();
    if (result.success && result.data) {
      setHealth(result.data);
    }
    setLoading(false);
    setRefreshing(false);
  };

  useEffect(() => {
    fetchHealth();
    const interval = setInterval(fetchHealth, 30000);
    return () => clearInterval(interval);
  }, []);

  const handleRefresh = () => {
    setRefreshing(true);
    fetchHealth();
  };

  if (loading) {
    return (
      <div className="p-8 flex items-center justify-center h-screen">
        <div className="text-center">
          <RefreshCw className="w-8 h-8 text-omniphi-500 animate-spin mx-auto mb-4" />
          <p className="text-dark-400">Loading system health...</p>
        </div>
      </div>
    );
  }

  if (!health) {
    return (
      <div className="p-8">
        <div className="card text-center py-12">
          <XCircle className="w-12 h-12 text-red-500 mx-auto mb-4" />
          <h2 className="text-xl font-semibold text-white mb-2">Failed to load system health</h2>
          <p className="text-dark-400 mb-4">Unable to connect to the orchestrator API</p>
          <button onClick={handleRefresh} className="btn btn-primary">
            Retry
          </button>
        </div>
      </div>
    );
  }

  const formatUptime = (seconds: number) => {
    const days = Math.floor(seconds / 86400);
    const hours = Math.floor((seconds % 86400) / 3600);
    const mins = Math.floor((seconds % 3600) / 60);
    return `${days}d ${hours}h ${mins}m`;
  };

  const statusColor = {
    healthy: 'text-green-400',
    degraded: 'text-yellow-400',
    unhealthy: 'text-red-400',
  };

  const statusBg = {
    healthy: 'bg-green-900/20 border-green-700/50',
    degraded: 'bg-yellow-900/20 border-yellow-700/50',
    unhealthy: 'bg-red-900/20 border-red-700/50',
  };

  // Chart data for provisioning success
  const pieData = [
    { name: 'Success', value: health.success_rate, color: '#22c55e' },
    { name: 'Failed', value: 100 - health.success_rate, color: '#ef4444' },
  ];

  // Mock trend data
  const trendData = Array.from({ length: 24 }, (_, i) => ({
    hour: `${i}:00`,
    validators: Math.floor(health.active_validators * (0.9 + Math.random() * 0.1)),
    requests: Math.floor(Math.random() * 10),
  }));

  return (
    <div className="p-8">
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-white">System Overview</h1>
          <p className="text-dark-400 mt-1">Monitor orchestrator health and validator status</p>
        </div>
        <button
          onClick={handleRefresh}
          disabled={refreshing}
          className="btn btn-secondary"
        >
          <RefreshCw className={`w-4 h-4 mr-2 ${refreshing ? 'animate-spin' : ''}`} />
          Refresh
        </button>
      </div>

      {/* Orchestrator Status Banner */}
      <div className={`card mb-6 border ${statusBg[health.orchestrator_status]}`}>
        <div className="flex items-center justify-between">
          <div className="flex items-center space-x-4">
            <div className={`p-3 rounded-lg ${health.orchestrator_status === 'healthy' ? 'bg-green-500/20' : 'bg-yellow-500/20'}`}>
              {health.orchestrator_status === 'healthy' ? (
                <CheckCircle className="w-6 h-6 text-green-400" />
              ) : (
                <AlertTriangle className="w-6 h-6 text-yellow-400" />
              )}
            </div>
            <div>
              <h2 className="text-lg font-semibold text-white capitalize">
                Orchestrator {health.orchestrator_status}
              </h2>
              <p className="text-dark-400 text-sm">
                Version {health.orchestrator_version} • Uptime: {formatUptime(health.orchestrator_uptime)}
              </p>
            </div>
          </div>
          <div className="text-right">
            <p className={`text-2xl font-bold ${statusColor[health.orchestrator_status]}`}>
              {health.success_rate.toFixed(1)}%
            </p>
            <p className="text-dark-400 text-sm">Success Rate</p>
          </div>
        </div>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
        <div className="stat-card">
          <div className="flex items-center justify-between">
            <Server className="w-5 h-5 text-omniphi-400" />
            <span className="badge badge-success">Active</span>
          </div>
          <p className="stat-value">{health.active_validators}</p>
          <p className="stat-label">Active Validators</p>
          <p className="stat-change stat-change-up">
            <TrendingUp className="w-3 h-3 inline mr-1" />
            of {health.total_validators} total
          </p>
        </div>

        <div className="stat-card">
          <div className="flex items-center justify-between">
            <Clock className="w-5 h-5 text-blue-400" />
            <span className="badge badge-info">{health.pending_requests} pending</span>
          </div>
          <p className="stat-value">{health.avg_provisioning_time}s</p>
          <p className="stat-label">Avg Provisioning Time</p>
          <p className="stat-change text-dark-500">Target: &lt;300s</p>
        </div>

        <div className="stat-card">
          <div className="flex items-center justify-between">
            <AlertTriangle className="w-5 h-5 text-yellow-400" />
            {health.provisioning_failures > 0 && (
              <span className="badge badge-warning">{health.provisioning_failures} failures</span>
            )}
          </div>
          <p className="stat-value">{health.provisioning_failures}</p>
          <p className="stat-label">Recent Failures</p>
          <p className="stat-change stat-change-down">
            Last 24 hours
          </p>
        </div>

        <div className="stat-card">
          <div className="flex items-center justify-between">
            <Activity className="w-5 h-5 text-green-400" />
          </div>
          <p className="stat-value">{health.pending_requests}</p>
          <p className="stat-label">Pending Requests</p>
          <p className="stat-change text-dark-500">In queue</p>
        </div>
      </div>

      {/* Charts Row */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 mb-8">
        {/* Validator Trend Chart */}
        <div className="card lg:col-span-2">
          <h3 className="card-title mb-4">Validator Activity (24h)</h3>
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={trendData}>
                <defs>
                  <linearGradient id="colorValidators" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#8b5cf6" stopOpacity={0.3} />
                    <stop offset="95%" stopColor="#8b5cf6" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
                <XAxis dataKey="hour" stroke="#64748b" fontSize={12} />
                <YAxis stroke="#64748b" fontSize={12} />
                <Tooltip
                  contentStyle={{
                    backgroundColor: '#1e293b',
                    border: '1px solid #334155',
                    borderRadius: '8px',
                  }}
                />
                <Area
                  type="monotone"
                  dataKey="validators"
                  stroke="#8b5cf6"
                  fillOpacity={1}
                  fill="url(#colorValidators)"
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </div>

        {/* Success Rate Pie */}
        <div className="card">
          <h3 className="card-title mb-4">Provisioning Success</h3>
          <div className="h-48">
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={pieData}
                  cx="50%"
                  cy="50%"
                  innerRadius={50}
                  outerRadius={70}
                  paddingAngle={2}
                  dataKey="value"
                >
                  {pieData.map((entry, index) => (
                    <Cell key={`cell-${index}`} fill={entry.color} />
                  ))}
                </Pie>
                <Tooltip
                  contentStyle={{
                    backgroundColor: '#1e293b',
                    border: '1px solid #334155',
                    borderRadius: '8px',
                  }}
                />
              </PieChart>
            </ResponsiveContainer>
          </div>
          <div className="flex justify-center space-x-6 mt-2">
            <div className="flex items-center space-x-2">
              <div className="w-3 h-3 rounded-full bg-green-500" />
              <span className="text-sm text-dark-300">Success</span>
            </div>
            <div className="flex items-center space-x-2">
              <div className="w-3 h-3 rounded-full bg-red-500" />
              <span className="text-sm text-dark-300">Failed</span>
            </div>
          </div>
        </div>
      </div>

      {/* Resource Usage & RPC Health */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
        {/* Resource Usage */}
        <div className="card">
          <h3 className="card-title mb-4">Resource Usage</h3>
          <div className="space-y-4">
            <ResourceBar
              icon={<Cpu className="w-4 h-4" />}
              label="CPU"
              value={health.resource_usage.cpu_percent}
            />
            <ResourceBar
              icon={<Database className="w-4 h-4" />}
              label="Memory"
              value={health.resource_usage.memory_percent}
              detail={health.resource_usage.memory_used}
            />
            <ResourceBar
              icon={<HardDrive className="w-4 h-4" />}
              label="Disk"
              value={health.resource_usage.disk_percent}
              detail={health.resource_usage.disk_used}
            />
          </div>
        </div>

        {/* RPC Health */}
        <div className="card">
          <h3 className="card-title mb-4">Chain RPC Status</h3>
          <div className="space-y-3">
            {health.chain_rpc_status.map((rpc) => (
              <div
                key={rpc.chain_id}
                className="flex items-center justify-between p-3 bg-dark-800 rounded-lg"
              >
                <div className="flex items-center space-x-3">
                  {rpc.status === 'healthy' ? (
                    <CheckCircle className="w-5 h-5 text-green-400" />
                  ) : rpc.status === 'degraded' ? (
                    <AlertTriangle className="w-5 h-5 text-yellow-400" />
                  ) : (
                    <XCircle className="w-5 h-5 text-red-400" />
                  )}
                  <div>
                    <p className="text-sm font-medium text-white">{rpc.chain_id}</p>
                    <p className="text-xs text-dark-400 truncate max-w-[200px]">{rpc.endpoint}</p>
                  </div>
                </div>
                <div className="text-right">
                  <p className="text-sm text-dark-200">{rpc.latency_ms}ms</p>
                  <p className="text-xs text-dark-400">Block #{rpc.block_height.toLocaleString()}</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Recent Errors */}
      <div className="card">
        <div className="card-header">
          <h3 className="card-title">Recent Errors</h3>
          <span className="badge badge-error">{health.recent_errors.length} errors</span>
        </div>
        {health.recent_errors.length === 0 ? (
          <div className="text-center py-8">
            <CheckCircle className="w-12 h-12 text-green-500 mx-auto mb-3" />
            <p className="text-dark-400">No recent errors</p>
          </div>
        ) : (
          <div className="space-y-3">
            {health.recent_errors.map((error) => (
              <ErrorCard key={error.id} error={error} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function ResourceBar({
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
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center space-x-2 text-dark-300">
          {icon}
          <span className="text-sm">{label}</span>
        </div>
        <div className="text-right">
          <span className="text-sm font-medium text-white">{value.toFixed(1)}%</span>
          {detail && <span className="text-xs text-dark-400 ml-2">{detail}</span>}
        </div>
      </div>
      <div className="w-full bg-dark-700 rounded-full h-2">
        <div
          className={`h-2 rounded-full transition-all duration-300 ${getColor(value)}`}
          style={{ width: `${Math.min(value, 100)}%` }}
        />
      </div>
    </div>
  );
}

function ErrorCard({ error }: { error: RecentError }) {
  const typeColors = {
    provisioning: 'bg-purple-900/20 text-purple-400 border-purple-700/50',
    health_check: 'bg-yellow-900/20 text-yellow-400 border-yellow-700/50',
    rpc: 'bg-blue-900/20 text-blue-400 border-blue-700/50',
    system: 'bg-red-900/20 text-red-400 border-red-700/50',
  };

  return (
    <div className="flex items-start space-x-3 p-3 bg-dark-800 rounded-lg">
      <AlertTriangle className="w-5 h-5 text-red-400 flex-shrink-0 mt-0.5" />
      <div className="flex-1 min-w-0">
        <div className="flex items-center space-x-2 mb-1">
          <span className={`badge border text-xs ${typeColors[error.type]}`}>
            {error.type.replace('_', ' ')}
          </span>
          <span className="text-xs text-dark-500">
            {formatDistanceToNow(new Date(error.timestamp), { addSuffix: true })}
          </span>
        </div>
        <p className="text-sm text-dark-200">{error.message}</p>
        {(error.request_id || error.node_id) && (
          <p className="text-xs text-dark-500 mt-1">
            {error.request_id && `Request: ${error.request_id}`}
            {error.request_id && error.node_id && ' • '}
            {error.node_id && `Node: ${error.node_id}`}
          </p>
        )}
      </div>
    </div>
  );
}

export default OverviewPage;
