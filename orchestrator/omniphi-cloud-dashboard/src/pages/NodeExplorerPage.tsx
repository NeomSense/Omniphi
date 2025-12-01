/**
 * Node Explorer Page - List all nodes with filtering
 */

import { useEffect, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import {
  Search,
  RefreshCw,
  ChevronRight,
} from 'lucide-react';
import { api } from '../services/api';
import type { CloudNode, Region } from '../types';

export function NodeExplorerPage() {
  const [searchParams] = useSearchParams();
  const [nodes, setNodes] = useState<CloudNode[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [regionFilter, setRegionFilter] = useState<Region | 'all'>('all');
  const [statusFilter, setStatusFilter] = useState<string>('all');

  useEffect(() => {
    const fetchNodes = async () => {
      setLoading(true);
      const regionParam = searchParams.get('region') as Region | null;
      if (regionParam) {
        setRegionFilter(regionParam);
      }
      const result = await api.fleet.getNodes(regionParam || undefined);
      if (result.success && result.data) {
        setNodes(result.data);
      }
      setLoading(false);
    };
    fetchNodes();
  }, [searchParams]);

  const filteredNodes = nodes.filter((node) => {
    if (search && !node.moniker.toLowerCase().includes(search.toLowerCase()) &&
        !node.node_id.toLowerCase().includes(search.toLowerCase())) {
      return false;
    }
    if (regionFilter !== 'all' && node.region !== regionFilter) {
      return false;
    }
    if (statusFilter !== 'all' && node.status !== statusFilter) {
      return false;
    }
    return true;
  });

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <RefreshCw className="w-8 h-8 text-omniphi-500 animate-spin" />
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Node Explorer</h1>
          <p className="text-dark-400 mt-1">Browse and manage all Omniphi Cloud nodes</p>
        </div>
        <div className="flex items-center space-x-2 text-sm text-dark-400">
          <span>{filteredNodes.length} nodes</span>
        </div>
      </div>

      {/* Filters */}
      <div className="flex flex-wrap gap-4">
        <div className="relative flex-1 min-w-[200px]">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-dark-400" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search nodes..."
            className="input pl-10"
          />
        </div>
        <select
          value={regionFilter}
          onChange={(e) => setRegionFilter(e.target.value as Region | 'all')}
          className="select w-40"
        >
          <option value="all">All Regions</option>
          <option value="us-east">US East</option>
          <option value="us-west">US West</option>
          <option value="eu-central">EU Central</option>
          <option value="asia-pacific">Asia Pacific</option>
        </select>
        <select
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
          className="select w-40"
        >
          <option value="all">All Status</option>
          <option value="healthy">Healthy</option>
          <option value="warning">Warning</option>
          <option value="error">Error</option>
          <option value="offline">Offline</option>
        </select>
      </div>

      {/* Node Table */}
      <div className="card">
        <div className="overflow-x-auto">
          <table className="table">
            <thead>
              <tr>
                <th>Node</th>
                <th>Region</th>
                <th>Status</th>
                <th>CPU</th>
                <th>RAM</th>
                <th>Block Height</th>
                <th>Sync</th>
                <th>Uptime</th>
                <th>Last Heartbeat</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {filteredNodes.map((node) => (
                <tr key={node.id}>
                  <td>
                    <div className="flex items-center space-x-3">
                      <div className={`status-dot ${
                        node.status === 'healthy' ? 'status-dot-healthy' :
                        node.status === 'warning' ? 'status-dot-warning' :
                        node.status === 'error' ? 'status-dot-error' : 'status-dot-offline'
                      }`} />
                      <div>
                        <span className="text-white font-medium">{node.moniker}</span>
                        <div className="text-xs text-dark-500 font-mono">{node.node_id}</div>
                      </div>
                    </div>
                  </td>
                  <td>{node.region.replace('-', ' ').toUpperCase()}</td>
                  <td>
                    <span className={`badge ${
                      node.status === 'healthy' ? 'badge-success' :
                      node.status === 'warning' ? 'badge-warning' :
                      node.status === 'error' ? 'badge-error' : 'badge-neutral'
                    }`}>
                      {node.status}
                    </span>
                  </td>
                  <td>
                    <div className="flex items-center space-x-2">
                      <span className={node.cpu_percent > 80 ? 'text-red-400' : node.cpu_percent > 60 ? 'text-yellow-400' : 'text-green-400'}>
                        {node.cpu_percent.toFixed(0)}%
                      </span>
                    </div>
                  </td>
                  <td>
                    <span className={node.ram_percent > 85 ? 'text-red-400' : node.ram_percent > 70 ? 'text-yellow-400' : 'text-green-400'}>
                      {node.ram_percent.toFixed(0)}%
                    </span>
                  </td>
                  <td className="font-mono">{node.block_height.toLocaleString()}</td>
                  <td>
                    <span className={`badge ${
                      node.sync_status === 'synced' ? 'badge-success' :
                      node.sync_status === 'syncing' ? 'badge-info' : 'badge-warning'
                    }`}>
                      {node.sync_status}
                    </span>
                  </td>
                  <td>{node.uptime_percent.toFixed(2)}%</td>
                  <td className="text-dark-400 text-sm">
                    {new Date(node.last_heartbeat).toLocaleTimeString()}
                  </td>
                  <td>
                    <Link to={`/nodes/${node.id}`} className="btn btn-ghost btn-sm">
                      <ChevronRight className="w-4 h-4" />
                    </Link>
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

export default NodeExplorerPage;
