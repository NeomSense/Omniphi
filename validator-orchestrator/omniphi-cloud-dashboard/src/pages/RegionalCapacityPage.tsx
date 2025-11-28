/**
 * Regional Capacity Explorer Page
 */

import { useEffect, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import {
  Globe,
  Server,
  Cpu,
  MemoryStick,
  Network,
  Clock,
  DollarSign,
  Layers,
  RefreshCw,
  ChevronRight,
} from 'lucide-react';
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts';
import { api } from '../services/api';
import type { RegionData, CloudNode, Region } from '../types';

export function RegionalCapacityPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [regions, setRegions] = useState<RegionData[]>([]);
  const [selectedRegion, setSelectedRegion] = useState<Region | null>(null);
  const [regionNodes, setRegionNodes] = useState<CloudNode[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchRegions = async () => {
      setLoading(true);
      const result = await api.regions.getAll();
      if (result.success && result.data) {
        setRegions(result.data);
        const regionParam = searchParams.get('region') as Region;
        if (regionParam && result.data.find((r) => r.id === regionParam)) {
          setSelectedRegion(regionParam);
        }
      }
      setLoading(false);
    };
    fetchRegions();
  }, []);

  useEffect(() => {
    const fetchNodes = async () => {
      if (selectedRegion) {
        const result = await api.fleet.getNodes(selectedRegion);
        if (result.success && result.data) {
          setRegionNodes(result.data);
        }
      } else {
        setRegionNodes([]);
      }
    };
    fetchNodes();
  }, [selectedRegion]);

  const handleRegionSelect = (regionId: Region) => {
    setSelectedRegion(regionId);
    setSearchParams({ region: regionId });
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <RefreshCw className="w-8 h-8 text-omniphi-500 animate-spin" />
      </div>
    );
  }

  const capacityData = regions.map((r) => ({
    name: r.name.replace('-', ' ').toUpperCase(),
    used: r.total_nodes,
    available: r.max_capacity - r.total_nodes,
    max: r.max_capacity,
  }));

  const activeRegion = regions.find((r) => r.id === selectedRegion);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Regional Capacity Explorer</h1>
          <p className="text-dark-400 mt-1">Monitor and manage capacity across all regions</p>
        </div>
      </div>

      {/* Region Cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        {regions.map((region) => (
          <button
            key={region.id}
            onClick={() => handleRegionSelect(region.id)}
            className={`card card-hover text-left ${selectedRegion === region.id ? 'border-omniphi-500' : ''}`}
          >
            <div className="flex items-center justify-between mb-4">
              <div className="flex items-center space-x-2">
                <Globe className="w-5 h-5 text-omniphi-400" />
                <span className="text-white font-semibold">{region.display_name}</span>
              </div>
              <ChevronRight className={`w-4 h-4 text-dark-400 transition-transform ${selectedRegion === region.id ? 'rotate-90' : ''}`} />
            </div>

            <div className="space-y-3">
              <div>
                <div className="flex justify-between text-sm mb-1">
                  <span className="text-dark-400">Capacity</span>
                  <span className="text-white">{region.total_nodes} / {region.max_capacity}</span>
                </div>
                <div className="progress-bar">
                  <div
                    className={`progress-bar-fill ${(region.total_nodes / region.max_capacity) > 0.9 ? 'bg-red-500' : (region.total_nodes / region.max_capacity) > 0.7 ? 'bg-yellow-500' : 'bg-green-500'}`}
                    style={{ width: `${(region.total_nodes / region.max_capacity) * 100}%` }}
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-2 text-sm">
                <div className="flex items-center space-x-2">
                  <Cpu className="w-3 h-3 text-blue-400" />
                  <span className="text-dark-400">CPU:</span>
                  <span className="text-white">{region.avg_cpu_percent}%</span>
                </div>
                <div className="flex items-center space-x-2">
                  <MemoryStick className="w-3 h-3 text-purple-400" />
                  <span className="text-dark-400">RAM:</span>
                  <span className="text-white">{region.avg_ram_percent}%</span>
                </div>
              </div>

              <div className="flex items-center justify-between text-sm pt-2 border-t border-dark-800">
                <span className="text-dark-400">Monthly Cost</span>
                <span className="text-white font-semibold">${region.monthly_cost.toLocaleString()}</span>
              </div>
            </div>
          </button>
        ))}
      </div>

      {/* Capacity Chart */}
      <div className="card">
        <h3 className="text-lg font-semibold text-white mb-4">Capacity Distribution</h3>
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={capacityData} layout="vertical">
              <XAxis type="number" tick={{ fill: '#9ca3af', fontSize: 12 }} />
              <YAxis type="category" dataKey="name" tick={{ fill: '#9ca3af', fontSize: 12 }} width={80} />
              <Tooltip
                contentStyle={{
                  backgroundColor: '#1f2937',
                  border: '1px solid #374151',
                  borderRadius: '0.5rem',
                }}
              />
              <Bar dataKey="used" stackId="a" fill="#7c3aed" name="Used" />
              <Bar dataKey="available" stackId="a" fill="#374151" name="Available" />
            </BarChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/* Selected Region Details */}
      {activeRegion && (
        <div className="card">
          <div className="flex items-center justify-between mb-6">
            <h3 className="text-lg font-semibold text-white">{activeRegion.display_name} Details</h3>
            <Link to={`/nodes?region=${activeRegion.id}`} className="btn btn-secondary btn-sm">
              View All Nodes
            </Link>
          </div>

          {/* Region Stats */}
          <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-4 mb-6">
            <div className="bg-dark-800 rounded-lg p-4">
              <div className="flex items-center space-x-2 mb-1">
                <Server className="w-4 h-4 text-omniphi-400" />
                <span className="text-xs text-dark-400">Total Nodes</span>
              </div>
              <span className="text-xl font-bold text-white">{activeRegion.total_nodes}</span>
            </div>
            <div className="bg-dark-800 rounded-lg p-4">
              <div className="flex items-center space-x-2 mb-1">
                <Layers className="w-4 h-4 text-green-400" />
                <span className="text-xs text-dark-400">Available</span>
              </div>
              <span className="text-xl font-bold text-white">{activeRegion.available_capacity}</span>
            </div>
            <div className="bg-dark-800 rounded-lg p-4">
              <div className="flex items-center space-x-2 mb-1">
                <Network className="w-4 h-4 text-blue-400" />
                <span className="text-xs text-dark-400">P2P Score</span>
              </div>
              <span className="text-xl font-bold text-white">{activeRegion.p2p_connectivity_score}%</span>
            </div>
            <div className="bg-dark-800 rounded-lg p-4">
              <div className="flex items-center space-x-2 mb-1">
                <Clock className="w-4 h-4 text-yellow-400" />
                <span className="text-xs text-dark-400">Queue</span>
              </div>
              <span className="text-xl font-bold text-white">{activeRegion.provisioning_queue}</span>
            </div>
            <div className="bg-dark-800 rounded-lg p-4">
              <div className="flex items-center space-x-2 mb-1">
                <DollarSign className="w-4 h-4 text-green-400" />
                <span className="text-xs text-dark-400">Cost/Node</span>
              </div>
              <span className="text-xl font-bold text-white">${activeRegion.cost_per_node}</span>
            </div>
            <div className="bg-dark-800 rounded-lg p-4">
              <div className="flex items-center space-x-2 mb-1">
                <Clock className="w-4 h-4 text-purple-400" />
                <span className="text-xs text-dark-400">Latency</span>
              </div>
              <span className="text-xl font-bold text-white">{activeRegion.latency_ms}ms</span>
            </div>
          </div>

          {/* Node List Preview */}
          <h4 className="text-md font-medium text-white mb-3">Recent Nodes</h4>
          <div className="overflow-x-auto">
            <table className="table">
              <thead>
                <tr>
                  <th>Node</th>
                  <th>Status</th>
                  <th>CPU</th>
                  <th>RAM</th>
                  <th>Block Height</th>
                  <th>Uptime</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {regionNodes.slice(0, 5).map((node) => (
                  <tr key={node.id}>
                    <td>
                      <div>
                        <span className="text-white font-medium">{node.moniker}</span>
                        <div className="text-xs text-dark-500 font-mono">{node.node_id}</div>
                      </div>
                    </td>
                    <td>
                      <span className={`badge ${node.status === 'healthy' ? 'badge-success' : node.status === 'warning' ? 'badge-warning' : 'badge-error'}`}>
                        {node.status}
                      </span>
                    </td>
                    <td>{node.cpu_percent.toFixed(1)}%</td>
                    <td>{node.ram_percent.toFixed(1)}%</td>
                    <td className="font-mono">{node.block_height.toLocaleString()}</td>
                    <td>{node.uptime_percent.toFixed(2)}%</td>
                    <td>
                      <Link to={`/nodes/${node.id}`} className="btn btn-ghost btn-sm">
                        Details
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}

export default RegionalCapacityPage;
