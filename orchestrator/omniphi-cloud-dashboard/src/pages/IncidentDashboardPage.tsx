/**
 * Incident & Alert Dashboard Page
 */

import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import {
  AlertTriangle,
  AlertCircle,
  CheckCircle,
  Clock,
  RefreshCw,
  Eye,
  Wrench,
  ArrowRightLeft,
  FileText,
} from 'lucide-react';
import { api } from '../services/api';
import type { Incident } from '../types';

export function IncidentDashboardPage() {
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [filter, setFilter] = useState<'all' | 'active' | 'acknowledged' | 'resolved'>('all');
  const [loading, setLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState<string | null>(null);

  useEffect(() => {
    const fetchIncidents = async () => {
      setLoading(true);
      const result = await api.incidents.getAll();
      if (result.success && result.data) {
        setIncidents(result.data);
      }
      setLoading(false);
    };
    fetchIncidents();
  }, []);

  const handleAcknowledge = async (incidentId: string) => {
    setActionLoading(incidentId);
    await api.incidents.acknowledge(incidentId);
    setIncidents((prev) =>
      prev.map((inc) =>
        inc.id === incidentId
          ? { ...inc, status: 'acknowledged', acknowledged_at: new Date().toISOString(), acknowledged_by: 'current-user' }
          : inc
      )
    );
    setActionLoading(null);
  };

  const handleAutoRepair = async (incidentId: string) => {
    setActionLoading(incidentId);
    await api.incidents.triggerAutoRepair(incidentId);
    setActionLoading(null);
  };

  const handleResolve = async (incidentId: string) => {
    setActionLoading(incidentId);
    await api.incidents.resolve(incidentId);
    setIncidents((prev) =>
      prev.map((inc) =>
        inc.id === incidentId
          ? { ...inc, status: 'resolved', resolved_at: new Date().toISOString(), resolved_by: 'current-user' }
          : inc
      )
    );
    setActionLoading(null);
  };

  const filteredIncidents = incidents.filter((inc) => {
    if (filter === 'all') return true;
    return inc.status === filter;
  });

  const activeCount = incidents.filter((i) => i.status === 'active').length;
  const acknowledgedCount = incidents.filter((i) => i.status === 'acknowledged').length;
  const criticalCount = incidents.filter((i) => i.severity === 'critical' && i.status !== 'resolved').length;

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <RefreshCw className="w-8 h-8 text-omniphi-500 animate-spin" />
      </div>
    );
  }

  const getSeverityColor = (severity: Incident['severity']) => {
    switch (severity) {
      case 'critical':
        return 'border-l-red-500 bg-red-900/10';
      case 'high':
        return 'border-l-orange-500 bg-orange-900/10';
      case 'medium':
        return 'border-l-yellow-500 bg-yellow-900/10';
      case 'low':
        return 'border-l-blue-500 bg-blue-900/10';
    }
  };

  const getSeverityBadge = (severity: Incident['severity']) => {
    switch (severity) {
      case 'critical':
        return <span className="badge badge-error">Critical</span>;
      case 'high':
        return <span className="badge badge-warning">High</span>;
      case 'medium':
        return <span className="badge badge-info">Medium</span>;
      case 'low':
        return <span className="badge badge-neutral">Low</span>;
    }
  };

  const getTypeIcon = (type: Incident['type']) => {
    switch (type) {
      case 'health_failure':
        return <AlertCircle className="w-5 h-5 text-red-400" />;
      case 'out_of_sync':
        return <RefreshCw className="w-5 h-5 text-yellow-400" />;
      case 'frequent_restarts':
        return <RefreshCw className="w-5 h-5 text-orange-400" />;
      case 'network_partition':
        return <AlertTriangle className="w-5 h-5 text-red-400" />;
      case 'rpc_failure':
        return <AlertTriangle className="w-5 h-5 text-orange-400" />;
      case 'latency_spike':
        return <Clock className="w-5 h-5 text-yellow-400" />;
      default:
        return <AlertTriangle className="w-5 h-5 text-dark-400" />;
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Incident & Alert Dashboard</h1>
          <p className="text-dark-400 mt-1">Monitor and respond to infrastructure incidents</p>
        </div>
        <button className="btn btn-secondary">
          <RefreshCw className="w-4 h-4 mr-2" />
          Refresh
        </button>
      </div>

      {/* Summary Cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
        <div className="metric-card">
          <div className="flex items-center space-x-2 mb-2">
            <AlertTriangle className="w-4 h-4 text-red-400" />
          </div>
          <div className="metric-value text-red-400">{criticalCount}</div>
          <div className="metric-label">Critical</div>
        </div>
        <div className="metric-card">
          <div className="flex items-center space-x-2 mb-2">
            <AlertCircle className="w-4 h-4 text-yellow-400" />
          </div>
          <div className="metric-value text-yellow-400">{activeCount}</div>
          <div className="metric-label">Active</div>
        </div>
        <div className="metric-card">
          <div className="flex items-center space-x-2 mb-2">
            <Eye className="w-4 h-4 text-blue-400" />
          </div>
          <div className="metric-value text-blue-400">{acknowledgedCount}</div>
          <div className="metric-label">Acknowledged</div>
        </div>
        <div className="metric-card">
          <div className="flex items-center space-x-2 mb-2">
            <CheckCircle className="w-4 h-4 text-green-400" />
          </div>
          <div className="metric-value text-green-400">
            {incidents.filter((i) => i.status === 'resolved').length}
          </div>
          <div className="metric-label">Resolved (24h)</div>
        </div>
      </div>

      {/* Filter Tabs */}
      <div className="tabs">
        {(['all', 'active', 'acknowledged', 'resolved'] as const).map((f) => (
          <button
            key={f}
            onClick={() => setFilter(f)}
            className={`tab ${filter === f ? 'active' : ''}`}
          >
            {f.charAt(0).toUpperCase() + f.slice(1)}
            {f !== 'all' && (
              <span className="ml-2 text-xs bg-dark-700 px-1.5 py-0.5 rounded">
                {incidents.filter((i) => i.status === f).length}
              </span>
            )}
          </button>
        ))}
      </div>

      {/* Incident List */}
      <div className="space-y-3">
        {filteredIncidents.length === 0 ? (
          <div className="card text-center py-12">
            <CheckCircle className="w-12 h-12 text-green-400 mx-auto mb-4" />
            <h3 className="text-lg font-semibold text-white mb-2">No incidents</h3>
            <p className="text-dark-400">All systems are operating normally</p>
          </div>
        ) : (
          filteredIncidents.map((incident) => (
            <div
              key={incident.id}
              className={`card border-l-4 ${getSeverityColor(incident.severity)}`}
            >
              <div className="flex items-start justify-between">
                <div className="flex items-start space-x-4">
                  {getTypeIcon(incident.type)}
                  <div>
                    <div className="flex items-center space-x-2 mb-1">
                      <h3 className="text-white font-semibold">{incident.title}</h3>
                      {getSeverityBadge(incident.severity)}
                      <span className={`badge ${
                        incident.status === 'active' ? 'badge-error' :
                        incident.status === 'acknowledged' ? 'badge-info' : 'badge-success'
                      }`}>
                        {incident.status}
                      </span>
                    </div>
                    <p className="text-dark-400 text-sm mb-2">{incident.description}</p>
                    <div className="flex items-center space-x-4 text-xs text-dark-500">
                      <Link to={`/nodes/${incident.node_id}`} className="hover:text-white">
                        Node: <span className="text-dark-300">{incident.node_moniker}</span>
                      </Link>
                      <span>Region: <span className="text-dark-300">{incident.region.toUpperCase()}</span></span>
                      <span>Triggered: <span className="text-dark-300">{new Date(incident.triggered_at).toLocaleString()}</span></span>
                      {incident.acknowledged_at && (
                        <span>Ack'd by: <span className="text-dark-300">{incident.acknowledged_by}</span></span>
                      )}
                    </div>
                  </div>
                </div>

                {/* Actions */}
                <div className="flex space-x-2">
                  {incident.status === 'active' && (
                    <>
                      <button
                        onClick={() => handleAcknowledge(incident.id)}
                        disabled={actionLoading === incident.id}
                        className="btn btn-secondary btn-sm"
                      >
                        <Eye className="w-4 h-4 mr-1" />
                        Acknowledge
                      </button>
                      {incident.auto_repair_available && !incident.auto_repair_attempted && (
                        <button
                          onClick={() => handleAutoRepair(incident.id)}
                          disabled={actionLoading === incident.id}
                          className="btn btn-warning btn-sm"
                        >
                          <Wrench className="w-4 h-4 mr-1" />
                          Auto-Repair
                        </button>
                      )}
                    </>
                  )}
                  {incident.status === 'acknowledged' && (
                    <button
                      onClick={() => handleResolve(incident.id)}
                      disabled={actionLoading === incident.id}
                      className="btn btn-success btn-sm"
                    >
                      <CheckCircle className="w-4 h-4 mr-1" />
                      Resolve
                    </button>
                  )}
                  <Link to={`/nodes/${incident.node_id}`} className="btn btn-ghost btn-sm">
                    <FileText className="w-4 h-4 mr-1" />
                    Logs
                  </Link>
                  <Link to={`/nodes/${incident.node_id}`} className="btn btn-ghost btn-sm">
                    <ArrowRightLeft className="w-4 h-4 mr-1" />
                    Migrate
                  </Link>
                </div>
              </div>
            </div>
          ))
        )}
      </div>
    </div>
  );
}

export default IncidentDashboardPage;
