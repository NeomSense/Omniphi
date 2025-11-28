/**
 * Fleet Overview Page - Global metrics dashboard
 */

import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  Server,
  Cpu,
  MemoryStick,
  Activity,
  AlertTriangle,
  DollarSign,
  Globe,
  CheckCircle,
  Clock,
  RefreshCw,
} from 'lucide-react';
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer, PieChart, Pie, Cell } from 'recharts';
import { api } from '../services/api';
import type { FleetMetrics, RegionData, Incident } from '../types';

export function FleetOverviewPage() {
  const [metrics, setMetrics] = useState<FleetMetrics | null>(null);
  const [regions, setRegions] = useState<RegionData[]>([]);
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchData = async () => {
      setLoading(true);
      const [metricsResult, regionsResult, incidentsResult] = await Promise.all([
        api.fleet.getMetrics(),
        api.regions.getAll(),
        api.incidents.getAll('active'),
      ]);

      if (metricsResult.success && metricsResult.data) {
        setMetrics(metricsResult.data);
      }
      if (regionsResult.success && regionsResult.data) {
        setRegions(regionsResult.data);
      }
      if (incidentsResult.success && incidentsResult.data) {
        setIncidents(incidentsResult.data.filter((i) => i.status === 'active'));
      }
      setLoading(false);
    };
    fetchData();
  }, []);

  if (loading || !metrics) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <RefreshCw className="w-8 h-8 text-omniphi-500 animate-spin" />
      </div>
    );
  }

  const statusColors = {
    healthy: '#22c55e',
    warning: '#f59e0b',
    error: '#ef4444',
    offline: '#6b7280',
    provisioning: '#3b82f6',
  };

  const statusData = Object.entries(metrics.nodes_by_status).map(([key, value]) => ({
    name: key,
    value,
    color: statusColors[key as keyof typeof statusColors],
  }));

  const regionData = Object.entries(metrics.nodes_by_region).map(([key, value]) => ({
    name: key.replace('-', ' ').toUpperCase(),
    nodes: value,
  }));

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Fleet Overview</h1>
          <p className="text-dark-400 mt-1">Omniphi Cloud infrastructure status</p>
        </div>
        <div className="flex items-center space-x-2 text-sm">
          <Clock className="w-4 h-4 text-dark-400" />
          <span className="text-dark-400">Last updated: {new Date().toLocaleTimeString()}</span>
          <button className="btn btn-secondary btn-sm ml-4">
            <RefreshCw className="w-4 h-4 mr-2" />
            Refresh
          </button>
        </div>
      </div>

      {/* Active Incidents Banner */}
      {incidents.length > 0 && (
        <div className="bg-red-900/20 border border-red-800/50 rounded-xl p-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-3">
              <AlertTriangle className="w-5 h-5 text-red-400" />
              <span className="text-red-400 font-medium">
                {incidents.length} active incident{incidents.length > 1 ? 's' : ''} requiring attention
              </span>
            </div>
            <Link to="/incidents" className="btn btn-danger btn-sm">
              View Incidents
            </Link>
          </div>
        </div>
      )}

      {/* Top Metrics */}
      <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-4">
        <div className="metric-card">
          <div className="flex items-center space-x-2 mb-2">
            <Server className="w-4 h-4 text-omniphi-400" />
          </div>
          <div className="metric-value">{metrics.total_validators.toLocaleString()}</div>
          <div className="metric-label">Total Validators</div>
        </div>

        <div className="metric-card">
          <div className="flex items-center space-x-2 mb-2">
            <CheckCircle className="w-4 h-4 text-green-400" />
          </div>
          <div className="metric-value">{metrics.active_nodes.toLocaleString()}</div>
          <div className="metric-label">Active Nodes</div>
        </div>

        <div className="metric-card">
          <div className="flex items-center space-x-2 mb-2">
            <Cpu className="w-4 h-4 text-blue-400" />
          </div>
          <div className="metric-value">{metrics.cpu_usage_percent}%</div>
          <div className="metric-label">CPU Usage</div>
          <div className="progress-bar mt-2">
            <div
              className="progress-bar-fill bg-blue-500"
              style={{ width: `${metrics.cpu_usage_percent}%` }}
            />
          </div>
        </div>

        <div className="metric-card">
          <div className="flex items-center space-x-2 mb-2">
            <MemoryStick className="w-4 h-4 text-purple-400" />
          </div>
          <div className="metric-value">{metrics.ram_usage_percent}%</div>
          <div className="metric-label">RAM Usage</div>
          <div className="progress-bar mt-2">
            <div
              className="progress-bar-fill bg-purple-500"
              style={{ width: `${metrics.ram_usage_percent}%` }}
            />
          </div>
        </div>

        <div className="metric-card">
          <div className="flex items-center space-x-2 mb-2">
            <Activity className="w-4 h-4 text-green-400" />
          </div>
          <div className="metric-value">{metrics.avg_uptime_percent}%</div>
          <div className="metric-label">Avg Uptime</div>
        </div>

        <div className="metric-card">
          <div className="flex items-center space-x-2 mb-2">
            <DollarSign className="w-4 h-4 text-yellow-400" />
          </div>
          <div className="metric-value">${metrics.cost_per_validator}</div>
          <div className="metric-label">Cost/Validator</div>
        </div>
      </div>

      {/* Charts Row */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Node Status Distribution */}
        <div className="card">
          <h3 className="text-lg font-semibold text-white mb-4">Node Status</h3>
          <div className="h-48">
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={statusData}
                  cx="50%"
                  cy="50%"
                  innerRadius={40}
                  outerRadius={70}
                  paddingAngle={2}
                  dataKey="value"
                >
                  {statusData.map((entry, index) => (
                    <Cell key={`cell-${index}`} fill={entry.color} />
                  ))}
                </Pie>
                <Tooltip
                  contentStyle={{
                    backgroundColor: '#1f2937',
                    border: '1px solid #374151',
                    borderRadius: '0.5rem',
                  }}
                />
              </PieChart>
            </ResponsiveContainer>
          </div>
          <div className="grid grid-cols-2 gap-2 mt-4">
            {statusData.map((item) => (
              <div key={item.name} className="flex items-center space-x-2">
                <div className="w-3 h-3 rounded-full" style={{ backgroundColor: item.color }} />
                <span className="text-xs text-dark-400 capitalize">{item.name}</span>
                <span className="text-xs text-white ml-auto">{item.value}</span>
              </div>
            ))}
          </div>
        </div>

        {/* Region Distribution */}
        <div className="card">
          <h3 className="text-lg font-semibold text-white mb-4">Nodes by Region</h3>
          <div className="h-48">
            <ResponsiveContainer width="100%" height="100%">
              <AreaChart data={regionData}>
                <defs>
                  <linearGradient id="colorNodes" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="#7c3aed" stopOpacity={0.3} />
                    <stop offset="95%" stopColor="#7c3aed" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <XAxis
                  dataKey="name"
                  tick={{ fill: '#9ca3af', fontSize: 10 }}
                  axisLine={{ stroke: '#374151' }}
                />
                <YAxis hide />
                <Tooltip
                  contentStyle={{
                    backgroundColor: '#1f2937',
                    border: '1px solid #374151',
                    borderRadius: '0.5rem',
                  }}
                />
                <Area
                  type="monotone"
                  dataKey="nodes"
                  stroke="#7c3aed"
                  fill="url(#colorNodes)"
                  strokeWidth={2}
                />
              </AreaChart>
            </ResponsiveContainer>
          </div>
        </div>

        {/* Cost Summary */}
        <div className="card">
          <h3 className="text-lg font-semibold text-white mb-4">Cost Summary</h3>
          <div className="space-y-4">
            <div>
              <div className="flex items-center justify-between mb-1">
                <span className="text-sm text-dark-400">Monthly Cost</span>
                <span className="text-lg font-bold text-white">
                  ${metrics.total_monthly_cost.toLocaleString()}
                </span>
              </div>
              <div className="text-xs text-dark-500">
                ~${(metrics.total_monthly_cost / 30).toFixed(0)}/day
              </div>
            </div>
            <div className="border-t border-dark-800 pt-4">
              <div className="flex items-center justify-between mb-1">
                <span className="text-sm text-dark-400">Incidents (24h)</span>
                <span className={`text-lg font-bold ${metrics.incidents_24h > 0 ? 'text-red-400' : 'text-green-400'}`}>
                  {metrics.incidents_24h}
                </span>
              </div>
            </div>
            <Link to="/costs" className="btn btn-secondary btn-sm w-full mt-4">
              View Cost Details
            </Link>
          </div>
        </div>
      </div>

      {/* Regional Overview */}
      <div className="card">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-lg font-semibold text-white">Regional Capacity</h3>
          <Link to="/regions" className="text-sm text-omniphi-400 hover:text-omniphi-300">
            View All Regions
          </Link>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          {regions.map((region) => (
            <Link
              key={region.id}
              to={`/regions?region=${region.id}`}
              className="bg-dark-800 rounded-lg p-4 hover:bg-dark-700 transition-colors"
            >
              <div className="flex items-center justify-between mb-3">
                <div className="flex items-center space-x-2">
                  <Globe className="w-4 h-4 text-omniphi-400" />
                  <span className="text-white font-medium">{region.display_name}</span>
                </div>
                <span className="badge badge-success">{region.active_nodes} active</span>
              </div>
              <div className="grid grid-cols-2 gap-3 text-sm">
                <div>
                  <span className="text-dark-400">Capacity</span>
                  <div className="text-white">
                    {region.total_nodes}/{region.max_capacity}
                  </div>
                </div>
                <div>
                  <span className="text-dark-400">CPU</span>
                  <div className="text-white">{region.avg_cpu_percent}%</div>
                </div>
                <div>
                  <span className="text-dark-400">RAM</span>
                  <div className="text-white">{region.avg_ram_percent}%</div>
                </div>
                <div>
                  <span className="text-dark-400">Cost</span>
                  <div className="text-white">${region.monthly_cost.toLocaleString()}</div>
                </div>
              </div>
              <div className="mt-3">
                <div className="progress-bar">
                  <div
                    className="progress-bar-fill bg-omniphi-500"
                    style={{ width: `${(region.total_nodes / region.max_capacity) * 100}%` }}
                  />
                </div>
              </div>
            </Link>
          ))}
        </div>
      </div>
    </div>
  );
}

export default FleetOverviewPage;
