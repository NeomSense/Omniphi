/**
 * Alerts / Incidents Page
 */

import { useEffect, useState } from 'react';
import { formatDistanceToNow } from 'date-fns';
import {
  RefreshCw,
  AlertTriangle,
  AlertCircle,
  Info,
  CheckCircle,
  Bell,
  BellOff,
  Server,
  Clock,
  User,
  Filter,
} from 'lucide-react';
import { api } from '../services/api';
import type { Alert, AlertSeverity, AlertStatus } from '../types';

const severityConfig: Record<AlertSeverity, { icon: typeof AlertTriangle; color: string; bg: string }> = {
  info: { icon: Info, color: 'text-blue-400', bg: 'bg-blue-900/30 border-blue-700/50' },
  warning: { icon: AlertTriangle, color: 'text-yellow-400', bg: 'bg-yellow-900/30 border-yellow-700/50' },
  critical: { icon: AlertCircle, color: 'text-red-400', bg: 'bg-red-900/30 border-red-700/50' },
};

const statusConfig: Record<AlertStatus, { color: string; label: string }> = {
  active: { color: 'badge-error', label: 'Active' },
  acknowledged: { color: 'badge-warning', label: 'Acknowledged' },
  resolved: { color: 'badge-success', label: 'Resolved' },
};

export function AlertsPage() {
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [loading, setLoading] = useState(true);
  const [statusFilter, setStatusFilter] = useState<AlertStatus | ''>('');
  const [severityFilter, setSeverityFilter] = useState<AlertSeverity | ''>('');
  const [actionLoading, setActionLoading] = useState<string | null>(null);

  const fetchAlerts = async () => {
    setLoading(true);
    const result = await api.alerts.list({
      status: statusFilter || undefined,
      severity: severityFilter || undefined,
    });
    if (result.success && result.data) {
      setAlerts(result.data);
    }
    setLoading(false);
  };

  useEffect(() => {
    fetchAlerts();
    const interval = setInterval(fetchAlerts, 30000);
    return () => clearInterval(interval);
  }, [statusFilter, severityFilter]);

  const handleAcknowledge = async (id: string) => {
    setActionLoading(id);
    await api.alerts.acknowledge(id);
    await fetchAlerts();
    setActionLoading(null);
  };

  const handleResolve = async (id: string) => {
    setActionLoading(id);
    await api.alerts.resolve(id);
    await fetchAlerts();
    setActionLoading(null);
  };

  const acknowledgedAlerts = alerts.filter(a => a.status === 'acknowledged');
  const resolvedAlerts = alerts.filter(a => a.status === 'resolved');

  const criticalCount = alerts.filter(a => a.severity === 'critical' && a.status === 'active').length;
  const warningCount = alerts.filter(a => a.severity === 'warning' && a.status === 'active').length;

  return (
    <div className="p-8">
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-white">Alerts & Incidents</h1>
          <p className="text-dark-400 mt-1">Monitor and respond to system alerts</p>
        </div>
        <button onClick={fetchAlerts} className="btn btn-secondary">
          <RefreshCw className={`w-4 h-4 mr-2 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </button>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-6 mb-8">
        <div className={`card border ${criticalCount > 0 ? 'border-red-700/50 bg-red-900/10' : 'border-dark-700'}`}>
          <div className="flex items-center justify-between">
            <div>
              <p className="text-dark-400 text-sm">Critical</p>
              <p className={`text-3xl font-bold ${criticalCount > 0 ? 'text-red-400' : 'text-white'}`}>
                {criticalCount}
              </p>
            </div>
            <AlertCircle className={`w-8 h-8 ${criticalCount > 0 ? 'text-red-400 animate-pulse' : 'text-dark-600'}`} />
          </div>
        </div>

        <div className={`card border ${warningCount > 0 ? 'border-yellow-700/50 bg-yellow-900/10' : 'border-dark-700'}`}>
          <div className="flex items-center justify-between">
            <div>
              <p className="text-dark-400 text-sm">Warning</p>
              <p className={`text-3xl font-bold ${warningCount > 0 ? 'text-yellow-400' : 'text-white'}`}>
                {warningCount}
              </p>
            </div>
            <AlertTriangle className={`w-8 h-8 ${warningCount > 0 ? 'text-yellow-400' : 'text-dark-600'}`} />
          </div>
        </div>

        <div className="card">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-dark-400 text-sm">Acknowledged</p>
              <p className="text-3xl font-bold text-white">{acknowledgedAlerts.length}</p>
            </div>
            <Bell className="w-8 h-8 text-dark-600" />
          </div>
        </div>

        <div className="card">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-dark-400 text-sm">Resolved Today</p>
              <p className="text-3xl font-bold text-white">{resolvedAlerts.length}</p>
            </div>
            <CheckCircle className="w-8 h-8 text-green-600" />
          </div>
        </div>
      </div>

      {/* Filters */}
      <div className="card mb-6">
        <div className="flex flex-wrap items-center gap-4">
          <div className="flex items-center space-x-2 text-dark-400">
            <Filter className="w-4 h-4" />
            <span className="text-sm">Filters:</span>
          </div>

          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value as AlertStatus | '')}
            className="select w-40"
          >
            <option value="">All Status</option>
            <option value="active">Active</option>
            <option value="acknowledged">Acknowledged</option>
            <option value="resolved">Resolved</option>
          </select>

          <select
            value={severityFilter}
            onChange={(e) => setSeverityFilter(e.target.value as AlertSeverity | '')}
            className="select w-40"
          >
            <option value="">All Severity</option>
            <option value="critical">Critical</option>
            <option value="warning">Warning</option>
            <option value="info">Info</option>
          </select>

          {(statusFilter || severityFilter) && (
            <button
              onClick={() => {
                setStatusFilter('');
                setSeverityFilter('');
              }}
              className="btn btn-ghost btn-sm"
            >
              Clear Filters
            </button>
          )}
        </div>
      </div>

      {/* Alerts List */}
      {loading ? (
        <div className="flex items-center justify-center py-12">
          <RefreshCw className="w-8 h-8 text-omniphi-500 animate-spin" />
        </div>
      ) : alerts.length === 0 ? (
        <div className="card text-center py-12">
          <BellOff className="w-12 h-12 text-dark-600 mx-auto mb-4" />
          <h3 className="text-lg font-medium text-white mb-2">No alerts</h3>
          <p className="text-dark-400">All systems are operating normally</p>
        </div>
      ) : (
        <div className="space-y-4">
          {alerts.map((alert) => {
            const severity = severityConfig[alert.severity];
            const status = statusConfig[alert.status];
            const SeverityIcon = severity.icon;
            const isLoading = actionLoading === alert.id;

            return (
              <div
                key={alert.id}
                className={`card border ${severity.bg} transition-all`}
              >
                <div className="flex items-start justify-between">
                  <div className="flex items-start space-x-4">
                    <div className={`p-2 rounded-lg ${alert.severity === 'critical' ? 'bg-red-900/50' : alert.severity === 'warning' ? 'bg-yellow-900/50' : 'bg-blue-900/50'}`}>
                      <SeverityIcon className={`w-5 h-5 ${severity.color}`} />
                    </div>
                    <div className="flex-1">
                      <div className="flex items-center space-x-3 mb-1">
                        <h3 className="font-semibold text-white">{alert.title}</h3>
                        <span className={`badge ${status.color}`}>{status.label}</span>
                        <span className="badge badge-neutral text-xs uppercase">{alert.severity}</span>
                      </div>
                      <p className="text-dark-300 mb-3">{alert.message}</p>

                      <div className="flex flex-wrap items-center gap-4 text-xs text-dark-400">
                        <span className="flex items-center space-x-1">
                          <Clock className="w-3 h-3" />
                          <span>{formatDistanceToNow(new Date(alert.created_at), { addSuffix: true })}</span>
                        </span>

                        {alert.node_id && (
                          <span className="flex items-center space-x-1">
                            <Server className="w-3 h-3" />
                            <span className="font-mono">{alert.node_id}</span>
                          </span>
                        )}

                        {alert.request_id && (
                          <span className="font-mono">Request: {alert.request_id}</span>
                        )}

                        {alert.acknowledged_by && (
                          <span className="flex items-center space-x-1">
                            <User className="w-3 h-3" />
                            <span>
                              Acknowledged by {alert.acknowledged_by}{' '}
                              {alert.acknowledged_at && formatDistanceToNow(new Date(alert.acknowledged_at), { addSuffix: true })}
                            </span>
                          </span>
                        )}
                      </div>
                    </div>
                  </div>

                  <div className="flex items-center space-x-2 ml-4">
                    {alert.status === 'active' && (
                      <>
                        <button
                          onClick={() => handleAcknowledge(alert.id)}
                          disabled={isLoading}
                          className="btn btn-secondary btn-sm"
                        >
                          {isLoading ? (
                            <RefreshCw className="w-4 h-4 animate-spin" />
                          ) : (
                            <>
                              <Bell className="w-4 h-4 mr-1" />
                              Acknowledge
                            </>
                          )}
                        </button>
                        <button
                          onClick={() => handleResolve(alert.id)}
                          disabled={isLoading}
                          className="btn btn-success btn-sm"
                        >
                          <CheckCircle className="w-4 h-4 mr-1" />
                          Resolve
                        </button>
                      </>
                    )}

                    {alert.status === 'acknowledged' && (
                      <button
                        onClick={() => handleResolve(alert.id)}
                        disabled={isLoading}
                        className="btn btn-success btn-sm"
                      >
                        {isLoading ? (
                          <RefreshCw className="w-4 h-4 animate-spin" />
                        ) : (
                          <>
                            <CheckCircle className="w-4 h-4 mr-1" />
                            Resolve
                          </>
                        )}
                      </button>
                    )}

                    {alert.status === 'resolved' && (
                      <span className="text-green-400 text-sm flex items-center">
                        <CheckCircle className="w-4 h-4 mr-1" />
                        Resolved
                      </span>
                    )}
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Alert Types Legend */}
      <div className="mt-8 card">
        <h3 className="card-title mb-4">Alert Types</h3>
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 text-sm">
          <div className="flex items-center space-x-2">
            <div className="w-3 h-3 bg-red-500 rounded-full" />
            <span className="text-dark-300">node_unhealthy - Node failed health checks</span>
          </div>
          <div className="flex items-center space-x-2">
            <div className="w-3 h-3 bg-yellow-500 rounded-full" />
            <span className="text-dark-300">provisioning_stuck - Provisioning timeout</span>
          </div>
          <div className="flex items-center space-x-2">
            <div className="w-3 h-3 bg-orange-500 rounded-full" />
            <span className="text-dark-300">rpc_degraded - RPC latency issues</span>
          </div>
          <div className="flex items-center space-x-2">
            <div className="w-3 h-3 bg-purple-500 rounded-full" />
            <span className="text-dark-300">upgrade_required - Chain upgrade needed</span>
          </div>
        </div>
      </div>
    </div>
  );
}

export default AlertsPage;
