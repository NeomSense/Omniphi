/**
 * Cost & Billing Dashboard Page
 */

import { useEffect, useState } from 'react';
import {
  DollarSign,
  TrendingDown,
  Server,
  Zap,
  RefreshCw,
  ArrowDown,
  ArrowUp,
} from 'lucide-react';
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer, BarChart, Bar, PieChart, Pie, Cell } from 'recharts';
import { api } from '../services/api';
import type { CostBreakdown, MachineCost } from '../types';

export function CostDashboardPage() {
  const [costs, setCosts] = useState<CostBreakdown | null>(null);
  const [machineCosts, setMachineCosts] = useState<MachineCost[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchData = async () => {
      setLoading(true);
      const [costResult, machineResult] = await Promise.all([
        api.costs.getBreakdown(),
        api.costs.getMachineCosts(),
      ]);

      if (costResult.success && costResult.data) {
        setCosts(costResult.data);
      }
      if (machineResult.success && machineResult.data) {
        setMachineCosts(machineResult.data);
      }
      setLoading(false);
    };
    fetchData();
  }, []);

  if (loading || !costs) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <RefreshCw className="w-8 h-8 text-omniphi-500 animate-spin" />
      </div>
    );
  }

  const regionData = Object.entries(costs.cost_by_region).map(([region, cost]) => ({
    name: region.replace('-', ' ').toUpperCase(),
    cost,
  }));

  const costBreakdownData = [
    { name: 'Compute', value: costs.compute_cost, color: '#7c3aed' },
    { name: 'Storage', value: costs.storage_cost, color: '#3b82f6' },
    { name: 'Network', value: costs.network_cost, color: '#22c55e' },
  ];

  const machineTypeData = Object.entries(costs.cost_by_machine_type).map(([type, cost]) => ({
    name: type,
    cost,
  }));

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Cost & Billing Dashboard</h1>
          <p className="text-dark-400 mt-1">Infrastructure cost breakdown and optimization</p>
        </div>
        <div className="flex space-x-2">
          <select className="select w-40">
            <option>Last 30 days</option>
            <option>Last 90 days</option>
            <option>This year</option>
          </select>
        </div>
      </div>

      {/* Top Metrics */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div className="metric-card">
          <div className="flex items-center space-x-2 mb-2">
            <DollarSign className="w-4 h-4 text-green-400" />
          </div>
          <div className="metric-value">${costs.total_monthly_cost.toLocaleString()}</div>
          <div className="metric-label">Monthly Cost</div>
        </div>
        <div className="metric-card">
          <div className="flex items-center space-x-2 mb-2">
            <Server className="w-4 h-4 text-omniphi-400" />
          </div>
          <div className="metric-value">${costs.cost_per_validator_avg}</div>
          <div className="metric-label">Avg Cost/Validator</div>
        </div>
        <div className="metric-card">
          <div className="flex items-center space-x-2 mb-2">
            <TrendingDown className="w-4 h-4 text-yellow-400" />
          </div>
          <div className="metric-value text-yellow-400">${costs.unused_capacity_cost.toLocaleString()}</div>
          <div className="metric-label">Unused Capacity</div>
        </div>
        <div className="metric-card">
          <div className="flex items-center space-x-2 mb-2">
            <Zap className="w-4 h-4 text-blue-400" />
          </div>
          <div className="metric-value text-green-400">${costs.potential_savings.toLocaleString()}</div>
          <div className="metric-label">Potential Savings</div>
        </div>
      </div>

      {/* Cost Trend Chart */}
      <div className="card">
        <h3 className="text-lg font-semibold text-white mb-4">Cost Trend (30 Days)</h3>
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={costs.cost_trend_30d}>
              <defs>
                <linearGradient id="colorCost" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#7c3aed" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#7c3aed" stopOpacity={0} />
                </linearGradient>
              </defs>
              <XAxis
                dataKey="date"
                tick={{ fill: '#9ca3af', fontSize: 10 }}
                tickFormatter={(value) => new Date(value).getDate().toString()}
              />
              <YAxis tick={{ fill: '#9ca3af', fontSize: 12 }} tickFormatter={(value) => `$${value}`} />
              <Tooltip
                contentStyle={{
                  backgroundColor: '#1f2937',
                  border: '1px solid #374151',
                  borderRadius: '0.5rem',
                }}
                formatter={(value: number) => [`$${value.toFixed(2)}`, 'Daily Cost']}
              />
              <Area type="monotone" dataKey="cost" stroke="#7c3aed" fill="url(#colorCost)" strokeWidth={2} />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/* Cost Breakdown Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* By Category */}
        <div className="card">
          <h3 className="text-lg font-semibold text-white mb-4">By Category</h3>
          <div className="h-48">
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={costBreakdownData}
                  cx="50%"
                  cy="50%"
                  innerRadius={40}
                  outerRadius={70}
                  paddingAngle={2}
                  dataKey="value"
                >
                  {costBreakdownData.map((entry, index) => (
                    <Cell key={`cell-${index}`} fill={entry.color} />
                  ))}
                </Pie>
                <Tooltip
                  contentStyle={{
                    backgroundColor: '#1f2937',
                    border: '1px solid #374151',
                    borderRadius: '0.5rem',
                  }}
                  formatter={(value: number) => [`$${value.toLocaleString()}`, '']}
                />
              </PieChart>
            </ResponsiveContainer>
          </div>
          <div className="space-y-2 mt-4">
            {costBreakdownData.map((item) => (
              <div key={item.name} className="flex items-center justify-between">
                <div className="flex items-center space-x-2">
                  <div className="w-3 h-3 rounded-full" style={{ backgroundColor: item.color }} />
                  <span className="text-sm text-dark-400">{item.name}</span>
                </div>
                <span className="text-sm text-white">${item.value.toLocaleString()}</span>
              </div>
            ))}
          </div>
        </div>

        {/* By Region */}
        <div className="card">
          <h3 className="text-lg font-semibold text-white mb-4">By Region</h3>
          <div className="h-48">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={regionData} layout="vertical">
                <XAxis type="number" tick={{ fill: '#9ca3af', fontSize: 12 }} tickFormatter={(v) => `$${v / 1000}k`} />
                <YAxis type="category" dataKey="name" tick={{ fill: '#9ca3af', fontSize: 10 }} width={70} />
                <Tooltip
                  contentStyle={{
                    backgroundColor: '#1f2937',
                    border: '1px solid #374151',
                    borderRadius: '0.5rem',
                  }}
                  formatter={(value: number) => [`$${value.toLocaleString()}`, 'Cost']}
                />
                <Bar dataKey="cost" fill="#7c3aed" radius={[0, 4, 4, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>

        {/* By Machine Type */}
        <div className="card">
          <h3 className="text-lg font-semibold text-white mb-4">By Machine Type</h3>
          <div className="h-48">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={machineTypeData} layout="vertical">
                <XAxis type="number" tick={{ fill: '#9ca3af', fontSize: 12 }} tickFormatter={(v) => `$${v / 1000}k`} />
                <YAxis type="category" dataKey="name" tick={{ fill: '#9ca3af', fontSize: 10 }} width={70} />
                <Tooltip
                  contentStyle={{
                    backgroundColor: '#1f2937',
                    border: '1px solid #374151',
                    borderRadius: '0.5rem',
                  }}
                  formatter={(value: number) => [`$${value.toLocaleString()}`, 'Cost']}
                />
                <Bar dataKey="cost" fill="#3b82f6" radius={[0, 4, 4, 0]} />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </div>
      </div>

      {/* Savings Opportunities */}
      <div className="card">
        <h3 className="text-lg font-semibold text-white mb-4">Savings Opportunities</h3>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          <div className="bg-dark-800 rounded-lg p-4">
            <div className="flex items-center justify-between mb-2">
              <span className="text-dark-400">Unused Capacity</span>
              <ArrowDown className="w-4 h-4 text-yellow-400" />
            </div>
            <div className="text-2xl font-bold text-yellow-400">${costs.unused_capacity_cost.toLocaleString()}</div>
            <p className="text-xs text-dark-500 mt-1">
              Right-size underutilized machines
            </p>
          </div>
          <div className="bg-dark-800 rounded-lg p-4">
            <div className="flex items-center justify-between mb-2">
              <span className="text-dark-400">Auto-Scaling Savings</span>
              <ArrowDown className="w-4 h-4 text-green-400" />
            </div>
            <div className="text-2xl font-bold text-green-400">${costs.auto_scaling_savings.toLocaleString()}</div>
            <p className="text-xs text-dark-500 mt-1">
              Enable auto-scaling for variable workloads
            </p>
          </div>
          <div className="bg-dark-800 rounded-lg p-4">
            <div className="flex items-center justify-between mb-2">
              <span className="text-dark-400">Total Potential</span>
              <ArrowUp className="w-4 h-4 text-omniphi-400" />
            </div>
            <div className="text-2xl font-bold text-omniphi-400">${costs.potential_savings.toLocaleString()}</div>
            <p className="text-xs text-dark-500 mt-1">
              {((costs.potential_savings / costs.total_monthly_cost) * 100).toFixed(1)}% of monthly cost
            </p>
          </div>
        </div>
      </div>

      {/* Machine Cost Table */}
      <div className="card">
        <h3 className="text-lg font-semibold text-white mb-4">Cost per Machine</h3>
        <div className="overflow-x-auto">
          <table className="table">
            <thead>
              <tr>
                <th>Machine ID</th>
                <th>Type</th>
                <th>Region</th>
                <th>Validators</th>
                <th>Utilization</th>
                <th>Monthly Cost</th>
                <th>Cost/Validator</th>
              </tr>
            </thead>
            <tbody>
              {machineCosts.map((machine) => (
                <tr key={machine.machine_id}>
                  <td className="font-mono text-sm">{machine.machine_id}</td>
                  <td>{machine.machine_type}</td>
                  <td>{machine.region.toUpperCase()}</td>
                  <td>{machine.validators_hosted}</td>
                  <td>
                    <div className="flex items-center space-x-2">
                      <div className="w-16 progress-bar">
                        <div
                          className={`progress-bar-fill ${machine.utilization_percent > 80 ? 'bg-green-500' : machine.utilization_percent > 50 ? 'bg-yellow-500' : 'bg-red-500'}`}
                          style={{ width: `${machine.utilization_percent}%` }}
                        />
                      </div>
                      <span className="text-sm">{machine.utilization_percent.toFixed(0)}%</span>
                    </div>
                  </td>
                  <td className="font-semibold">${machine.monthly_cost.toFixed(2)}</td>
                  <td className={machine.cost_per_validator > 100 ? 'text-red-400' : 'text-green-400'}>
                    ${machine.cost_per_validator.toFixed(2)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}

export default CostDashboardPage;
