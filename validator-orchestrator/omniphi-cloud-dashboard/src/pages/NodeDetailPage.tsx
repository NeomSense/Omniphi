/**
 * Node Detail Page - Detailed node view with logs and actions
 */

import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import {
  ArrowLeft,
  Cpu,
  MemoryStick,
  HardDrive,
  Network,
  RefreshCw,
  RotateCcw,
  Power,
  ArrowRightLeft,
  Wrench,
} from 'lucide-react';
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts';
import { api } from '../services/api';
import type { CloudNode, LogEntry, Region } from '../types';

export function NodeDetailPage() {
  const { nodeId } = useParams<{ nodeId: string }>();
  const [node, setNode] = useState<CloudNode | null>(null);
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [activeLogSource, setActiveLogSource] = useState<string>('all');
  const [loading, setLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [showMigrateModal, setShowMigrateModal] = useState(false);

  useEffect(() => {
    const fetchData = async () => {
      if (!nodeId) return;
      setLoading(true);
      const [nodeResult, logsResult] = await Promise.all([
        api.fleet.getNode(nodeId),
        api.logs.getNodeLogs(nodeId),
      ]);

      if (nodeResult.success && nodeResult.data) {
        setNode(nodeResult.data);
      }
      if (logsResult.success && logsResult.data) {
        setLogs(logsResult.data);
      }
      setLoading(false);
    };
    fetchData();
  }, [nodeId]);

  const handleAction = async (action: 'restart' | 'terminate' | 'reprovision') => {
    if (!nodeId) return;
    setActionLoading(action);

    switch (action) {
      case 'restart':
        await api.fleet.restartNode(nodeId);
        break;
      case 'terminate':
        await api.fleet.terminateNode(nodeId);
        break;
      case 'reprovision':
        await api.fleet.reprovisionNode(nodeId);
        break;
    }

    setActionLoading(null);
    // Refresh node data
    const result = await api.fleet.getNode(nodeId);
    if (result.success && result.data) {
      setNode(result.data);
    }
  };

  const handleMigrate = async (targetRegion: Region) => {
    if (!nodeId) return;
    setActionLoading('migrate');
    await api.fleet.migrateNode(nodeId, targetRegion);
    setActionLoading(null);
    setShowMigrateModal(false);
  };

  const filteredLogs = activeLogSource === 'all'
    ? logs
    : logs.filter((log) => log.source === activeLogSource);

  if (loading || !node) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <RefreshCw className="w-8 h-8 text-omniphi-500 animate-spin" />
      </div>
    );
  }

  // Mock metrics history
  const metricsHistory = Array.from({ length: 24 }, (_, i) => ({
    time: `${23 - i}h`,
    cpu: 50 + Math.random() * 30,
    ram: 60 + Math.random() * 25,
  })).reverse();

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <Link to="/nodes" className="inline-flex items-center text-dark-400 hover:text-white mb-4">
          <ArrowLeft className="w-4 h-4 mr-2" />
          Back to Nodes
        </Link>

        <div className="flex items-start justify-between">
          <div>
            <div className="flex items-center space-x-3">
              <div className={`status-dot ${node.status === 'healthy' ? 'status-dot-healthy' : node.status === 'warning' ? 'status-dot-warning' : 'status-dot-error'}`} />
              <h1 className="text-2xl font-bold text-white">{node.moniker}</h1>
              <span className={`badge ${node.status === 'healthy' ? 'badge-success' : node.status === 'warning' ? 'badge-warning' : 'badge-error'}`}>
                {node.status}
              </span>
            </div>
            <p className="text-dark-400 mt-1 font-mono text-sm">{node.node_id}</p>
          </div>

          <div className="flex space-x-2">
            <button
              onClick={() => handleAction('restart')}
              disabled={actionLoading !== null}
              className="btn btn-secondary btn-sm"
            >
              {actionLoading === 'restart' ? <RefreshCw className="w-4 h-4 mr-2 animate-spin" /> : <RotateCcw className="w-4 h-4 mr-2" />}
              Restart
            </button>
            <button
              onClick={() => setShowMigrateModal(true)}
              disabled={actionLoading !== null}
              className="btn btn-secondary btn-sm"
            >
              <ArrowRightLeft className="w-4 h-4 mr-2" />
              Migrate
            </button>
            <button
              onClick={() => handleAction('reprovision')}
              disabled={actionLoading !== null}
              className="btn btn-warning btn-sm"
            >
              {actionLoading === 'reprovision' ? <RefreshCw className="w-4 h-4 mr-2 animate-spin" /> : <Wrench className="w-4 h-4 mr-2" />}
              Re-provision
            </button>
            <button
              onClick={() => handleAction('terminate')}
              disabled={actionLoading !== null}
              className="btn btn-danger btn-sm"
            >
              <Power className="w-4 h-4 mr-2" />
              Terminate
            </button>
          </div>
        </div>
      </div>

      {/* Node Info */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Status Card */}
        <div className="card">
          <h3 className="text-lg font-semibold text-white mb-4">Node Info</h3>
          <div className="space-y-3">
            <div className="flex justify-between">
              <span className="text-dark-400">Host Machine</span>
              <span className="text-white font-mono text-sm">{node.host_machine_id}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-dark-400">VM ID</span>
              <span className="text-white font-mono text-sm">{node.vm_id}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-dark-400">Region</span>
              <span className="text-white">{node.region.replace('-', ' ').toUpperCase()}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-dark-400">Sync Status</span>
              <span className={`badge ${node.sync_status === 'synced' ? 'badge-success' : node.sync_status === 'syncing' ? 'badge-info' : 'badge-warning'}`}>
                {node.sync_status}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-dark-400">Last Heartbeat</span>
              <span className="text-white text-sm">
                {new Date(node.last_heartbeat).toLocaleString()}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-dark-400">Monthly Cost</span>
              <span className="text-white font-semibold">${node.monthly_cost.toFixed(2)}</span>
            </div>
          </div>
        </div>

        {/* Resource Usage */}
        <div className="card">
          <h3 className="text-lg font-semibold text-white mb-4">Resource Usage</h3>
          <div className="space-y-4">
            <div>
              <div className="flex items-center justify-between mb-1">
                <div className="flex items-center space-x-2">
                  <Cpu className="w-4 h-4 text-blue-400" />
                  <span className="text-sm text-dark-400">CPU</span>
                </div>
                <span className="text-white font-semibold">{node.cpu_percent.toFixed(1)}%</span>
              </div>
              <div className="progress-bar">
                <div className="progress-bar-fill bg-blue-500" style={{ width: `${node.cpu_percent}%` }} />
              </div>
            </div>
            <div>
              <div className="flex items-center justify-between mb-1">
                <div className="flex items-center space-x-2">
                  <MemoryStick className="w-4 h-4 text-purple-400" />
                  <span className="text-sm text-dark-400">RAM ({node.ram_used_gb.toFixed(1)}/{node.ram_total_gb}GB)</span>
                </div>
                <span className="text-white font-semibold">{node.ram_percent.toFixed(1)}%</span>
              </div>
              <div className="progress-bar">
                <div className="progress-bar-fill bg-purple-500" style={{ width: `${node.ram_percent}%` }} />
              </div>
            </div>
            <div>
              <div className="flex items-center justify-between mb-1">
                <div className="flex items-center space-x-2">
                  <HardDrive className="w-4 h-4 text-green-400" />
                  <span className="text-sm text-dark-400">Disk ({node.disk_used_gb.toFixed(0)}/{node.disk_total_gb}GB)</span>
                </div>
                <span className="text-white font-semibold">{node.disk_percent.toFixed(1)}%</span>
              </div>
              <div className="progress-bar">
                <div className="progress-bar-fill bg-green-500" style={{ width: `${node.disk_percent}%` }} />
              </div>
            </div>
            <div className="pt-2 border-t border-dark-800">
              <div className="flex items-center justify-between">
                <div className="flex items-center space-x-2">
                  <Network className="w-4 h-4 text-yellow-400" />
                  <span className="text-sm text-dark-400">Network</span>
                </div>
                <div className="text-sm">
                  <span className="text-green-400">↓ {node.network_in_mbps.toFixed(0)}Mbps</span>
                  <span className="text-dark-500 mx-2">/</span>
                  <span className="text-blue-400">↑ {node.network_out_mbps.toFixed(0)}Mbps</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        {/* Chain Metrics */}
        <div className="card">
          <h3 className="text-lg font-semibold text-white mb-4">Chain Metrics</h3>
          <div className="space-y-3">
            <div className="flex justify-between">
              <span className="text-dark-400">Block Height</span>
              <span className="text-white font-mono">{node.block_height.toLocaleString()}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-dark-400">Chain Height</span>
              <span className="text-white font-mono">{node.latest_chain_height.toLocaleString()}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-dark-400">Blocks Behind</span>
              <span className={`font-mono ${node.blocks_behind > 10 ? 'text-red-400' : node.blocks_behind > 0 ? 'text-yellow-400' : 'text-green-400'}`}>
                {node.blocks_behind}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-dark-400">Peers</span>
              <span className="text-white">{node.peers}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-dark-400">Missed Blocks (24h)</span>
              <span className={`${node.missed_blocks_24h > 0 ? 'text-red-400' : 'text-green-400'}`}>
                {node.missed_blocks_24h}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-dark-400">Uptime</span>
              <span className="text-white font-semibold">{node.uptime_percent.toFixed(2)}%</span>
            </div>
            <div className="flex justify-between">
              <span className="text-dark-400">Restarts (24h)</span>
              <span className={`${node.restart_count_24h > 0 ? 'text-yellow-400' : 'text-white'}`}>
                {node.restart_count_24h}
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Metrics Chart */}
      <div className="card">
        <h3 className="text-lg font-semibold text-white mb-4">Resource Usage (24h)</h3>
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={metricsHistory}>
              <defs>
                <linearGradient id="colorCpu" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#3b82f6" stopOpacity={0} />
                </linearGradient>
                <linearGradient id="colorRam" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#a855f7" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#a855f7" stopOpacity={0} />
                </linearGradient>
              </defs>
              <XAxis dataKey="time" tick={{ fill: '#9ca3af', fontSize: 12 }} />
              <YAxis domain={[0, 100]} tick={{ fill: '#9ca3af', fontSize: 12 }} />
              <Tooltip
                contentStyle={{
                  backgroundColor: '#1f2937',
                  border: '1px solid #374151',
                  borderRadius: '0.5rem',
                }}
              />
              <Area type="monotone" dataKey="cpu" stroke="#3b82f6" fill="url(#colorCpu)" name="CPU %" />
              <Area type="monotone" dataKey="ram" stroke="#a855f7" fill="url(#colorRam)" name="RAM %" />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/* Logs */}
      <div className="card">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-lg font-semibold text-white">Logs</h3>
          <div className="tabs" style={{ marginBottom: 0, borderBottom: 'none' }}>
            {['all', 'docker', 'tendermint', 'app', 'system'].map((source) => (
              <button
                key={source}
                onClick={() => setActiveLogSource(source)}
                className={`tab ${activeLogSource === source ? 'active' : ''}`}
              >
                {source.charAt(0).toUpperCase() + source.slice(1)}
              </button>
            ))}
          </div>
        </div>
        <div className="log-viewer">
          {filteredLogs.slice(0, 50).map((log, i) => (
            <div key={i} className="log-line">
              <span className="log-timestamp">
                {new Date(log.timestamp).toLocaleTimeString()}
              </span>
              <span className={`log-level-${log.level}`}>[{log.level.toUpperCase()}]</span>
              <span className="text-dark-500">[{log.source}]</span>
              <span className="log-message">{log.message}</span>
            </div>
          ))}
        </div>
      </div>

      {/* Migrate Modal */}
      {showMigrateModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="card max-w-md w-full mx-4">
            <h3 className="text-xl font-bold text-white mb-4">Migrate Node</h3>
            <p className="text-dark-400 mb-6">Select target region for migration:</p>
            <div className="space-y-2">
              {(['us-east', 'us-west', 'eu-central', 'asia-pacific'] as Region[])
                .filter((r) => r !== node.region)
                .map((region) => (
                  <button
                    key={region}
                    onClick={() => handleMigrate(region)}
                    disabled={actionLoading === 'migrate'}
                    className="w-full p-3 bg-dark-800 hover:bg-dark-700 rounded-lg text-left flex items-center justify-between"
                  >
                    <span className="text-white">{region.replace('-', ' ').toUpperCase()}</span>
                    <ArrowRightLeft className="w-4 h-4 text-dark-400" />
                  </button>
                ))}
            </div>
            <div className="flex justify-end mt-6">
              <button onClick={() => setShowMigrateModal(false)} className="btn btn-secondary">
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export default NodeDetailPage;
