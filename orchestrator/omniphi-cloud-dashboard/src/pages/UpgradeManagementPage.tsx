/**
 * Upgrade Management Page - Chain upgrade tracking and rollout
 */

import { useEffect, useState } from 'react';
import {
  RotateCcw,
  RefreshCw,
  ChevronDown,
  ChevronUp,
} from 'lucide-react';
import { api } from '../services/api';
import type { ChainUpgrade, NodeUpgradeStatus } from '../types';

export function UpgradeManagementPage() {
  const [upgrades, setUpgrades] = useState<ChainUpgrade[]>([]);
  const [selectedUpgrade, setSelectedUpgrade] = useState<ChainUpgrade | null>(null);
  const [nodeStatuses, setNodeStatuses] = useState<NodeUpgradeStatus[]>([]);
  const [showLogs, setShowLogs] = useState(false);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchData = async () => {
      setLoading(true);
      const result = await api.upgrades.getAll();
      if (result.success && result.data) {
        setUpgrades(result.data);
        if (result.data.length > 0) {
          setSelectedUpgrade(result.data[0]);
        }
      }
      setLoading(false);
    };
    fetchData();
  }, []);

  useEffect(() => {
    const fetchNodeStatuses = async () => {
      if (selectedUpgrade) {
        const result = await api.upgrades.getNodeStatuses(selectedUpgrade.id);
        if (result.success && result.data) {
          setNodeStatuses(result.data);
        }
      }
    };
    fetchNodeStatuses();
  }, [selectedUpgrade]);

  const handleRollback = async () => {
    if (selectedUpgrade) {
      await api.upgrades.rollback(selectedUpgrade.id);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <RefreshCw className="w-8 h-8 text-omniphi-500 animate-spin" />
      </div>
    );
  }

  const getStatusBadge = (status: ChainUpgrade['status']) => {
    switch (status) {
      case 'scheduled':
        return <span className="badge badge-info">Scheduled</span>;
      case 'in_progress':
        return <span className="badge badge-warning">In Progress</span>;
      case 'completed':
        return <span className="badge badge-success">Completed</span>;
      case 'failed':
        return <span className="badge badge-error">Failed</span>;
      case 'rolled_back':
        return <span className="badge badge-neutral">Rolled Back</span>;
      default:
        return null;
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-white">Upgrade Management</h1>
        <p className="text-dark-400 mt-1">Track and manage chain upgrades across the fleet</p>
      </div>

      {/* Upgrade List */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Upgrades Sidebar */}
        <div className="space-y-3">
          {upgrades.map((upgrade) => (
            <button
              key={upgrade.id}
              onClick={() => setSelectedUpgrade(upgrade)}
              className={`card card-hover w-full text-left ${selectedUpgrade?.id === upgrade.id ? 'border-omniphi-500' : ''}`}
            >
              <div className="flex items-center justify-between mb-2">
                <span className="text-white font-semibold">{upgrade.name}</span>
                {getStatusBadge(upgrade.status)}
              </div>
              <div className="text-sm text-dark-400 space-y-1">
                <div>Version: <span className="text-white">{upgrade.version}</span></div>
                <div>Height: <span className="text-white">{upgrade.upgrade_height.toLocaleString()}</span></div>
                {upgrade.status === 'scheduled' && (
                  <div>Scheduled: <span className="text-white">{new Date(upgrade.scheduled_time).toLocaleDateString()}</span></div>
                )}
              </div>
              {upgrade.status !== 'scheduled' && (
                <div className="mt-3">
                  <div className="flex justify-between text-xs mb-1">
                    <span className="text-dark-400">Progress</span>
                    <span className="text-white">{upgrade.completion_percent.toFixed(1)}%</span>
                  </div>
                  <div className="progress-bar">
                    <div
                      className={`progress-bar-fill ${upgrade.failed_nodes > 0 ? 'bg-yellow-500' : 'bg-green-500'}`}
                      style={{ width: `${upgrade.completion_percent}%` }}
                    />
                  </div>
                </div>
              )}
            </button>
          ))}
        </div>

        {/* Selected Upgrade Details */}
        {selectedUpgrade && (
          <div className="lg:col-span-2 space-y-6">
            {/* Upgrade Info Card */}
            <div className="card">
              <div className="flex items-center justify-between mb-6">
                <div>
                  <h2 className="text-xl font-bold text-white">{selectedUpgrade.name}</h2>
                  <p className="text-dark-400">{selectedUpgrade.new_binary_version}</p>
                </div>
                <div className="flex space-x-2">
                  {selectedUpgrade.rollback_available && (
                    <button onClick={handleRollback} className="btn btn-warning btn-sm">
                      <RotateCcw className="w-4 h-4 mr-2" />
                      Rollback
                    </button>
                  )}
                </div>
              </div>

              {/* Progress Stats */}
              <div className="grid grid-cols-2 md:grid-cols-5 gap-4 mb-6">
                <div className="bg-dark-800 rounded-lg p-3 text-center">
                  <div className="text-2xl font-bold text-white">{selectedUpgrade.total_nodes}</div>
                  <div className="text-xs text-dark-400">Total Nodes</div>
                </div>
                <div className="bg-dark-800 rounded-lg p-3 text-center">
                  <div className="text-2xl font-bold text-green-400">{selectedUpgrade.updated_nodes}</div>
                  <div className="text-xs text-dark-400">Updated</div>
                </div>
                <div className="bg-dark-800 rounded-lg p-3 text-center">
                  <div className="text-2xl font-bold text-red-400">{selectedUpgrade.failed_nodes}</div>
                  <div className="text-xs text-dark-400">Failed</div>
                </div>
                <div className="bg-dark-800 rounded-lg p-3 text-center">
                  <div className="text-2xl font-bold text-yellow-400">{selectedUpgrade.pending_nodes}</div>
                  <div className="text-xs text-dark-400">Pending</div>
                </div>
                <div className="bg-dark-800 rounded-lg p-3 text-center">
                  <div className="text-2xl font-bold text-omniphi-400">{selectedUpgrade.completion_percent.toFixed(1)}%</div>
                  <div className="text-xs text-dark-400">Complete</div>
                </div>
              </div>

              {/* Canary Status */}
              <div className="bg-dark-800 rounded-lg p-4 mb-6">
                <div className="flex items-center justify-between mb-2">
                  <span className="text-white font-medium">Canary Nodes</span>
                  <span className={`badge ${selectedUpgrade.canary_success ? 'badge-success' : selectedUpgrade.canary_completed ? 'badge-error' : 'badge-info'}`}>
                    {selectedUpgrade.canary_success ? 'Success' : selectedUpgrade.canary_completed ? 'Failed' : 'Pending'}
                  </span>
                </div>
                <div className="flex space-x-2">
                  {selectedUpgrade.canary_nodes.map((nodeId) => (
                    <span key={nodeId} className="text-xs bg-dark-700 px-2 py-1 rounded text-dark-300 font-mono">
                      {nodeId}
                    </span>
                  ))}
                </div>
              </div>

              {/* Upgrade Height Info */}
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <span className="text-sm text-dark-400">Upgrade Height</span>
                  <div className="text-white font-mono text-lg">{selectedUpgrade.upgrade_height.toLocaleString()}</div>
                </div>
                <div>
                  <span className="text-sm text-dark-400">Current Height</span>
                  <div className="text-white font-mono text-lg">{selectedUpgrade.current_height.toLocaleString()}</div>
                </div>
              </div>
            </div>

            {/* Node Statuses */}
            <div className="card">
              <h3 className="text-lg font-semibold text-white mb-4">Node Update Status</h3>
              <div className="overflow-x-auto">
                <table className="table">
                  <thead>
                    <tr>
                      <th>Node</th>
                      <th>Region</th>
                      <th>Status</th>
                      <th>Current</th>
                      <th>Target</th>
                      <th>Started</th>
                    </tr>
                  </thead>
                  <tbody>
                    {nodeStatuses.map((node) => (
                      <tr key={node.node_id}>
                        <td className="font-mono text-sm">{node.moniker}</td>
                        <td>{node.region.toUpperCase()}</td>
                        <td>
                          <span className={`badge ${
                            node.status === 'completed' ? 'badge-success' :
                            node.status === 'failed' ? 'badge-error' :
                            node.status === 'updating' ? 'badge-warning' : 'badge-neutral'
                          }`}>
                            {node.status}
                          </span>
                        </td>
                        <td className="font-mono text-sm">{node.current_version}</td>
                        <td className="font-mono text-sm">{node.target_version}</td>
                        <td className="text-sm text-dark-400">
                          {node.started_at ? new Date(node.started_at).toLocaleTimeString() : '-'}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>

            {/* Upgrade Logs */}
            <div className="card">
              <button
                onClick={() => setShowLogs(!showLogs)}
                className="flex items-center justify-between w-full text-left"
              >
                <h3 className="text-lg font-semibold text-white">Upgrade Logs</h3>
                {showLogs ? <ChevronUp className="w-5 h-5 text-dark-400" /> : <ChevronDown className="w-5 h-5 text-dark-400" />}
              </button>
              {showLogs && (
                <div className="log-viewer mt-4">
                  {selectedUpgrade.upgrade_logs.map((log, i) => (
                    <div key={i} className="log-line">
                      <span className="log-timestamp">{new Date(log.timestamp).toLocaleTimeString()}</span>
                      <span className={`log-level-${log.level}`}>[{log.level.toUpperCase()}]</span>
                      <span className="log-message">{log.message}</span>
                    </div>
                  ))}
                  {selectedUpgrade.upgrade_logs.length === 0 && (
                    <div className="text-dark-400 text-center py-4">No logs available</div>
                  )}
                </div>
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

export default UpgradeManagementPage;
