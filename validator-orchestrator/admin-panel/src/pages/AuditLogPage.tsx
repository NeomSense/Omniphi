/**
 * Audit Log Page
 */

import { useEffect, useState } from 'react';
import { format } from 'date-fns';
import {
  RefreshCw,
  Search,
  User,
  LogIn,
  RotateCw,
  Square,
  Settings,
  AlertTriangle,
  Trash2,
  ChevronLeft,
  ChevronRight,
  Clock,
  Globe,
} from 'lucide-react';
import { api } from '../services/api';
import type { AuditLogEntry, AuditAction } from '../types';

const actionConfig: Record<AuditAction, { icon: typeof LogIn; color: string; label: string }> = {
  login: { icon: LogIn, color: 'text-green-400', label: 'Login' },
  logout: { icon: LogIn, color: 'text-gray-400', label: 'Logout' },
  create_request: { icon: RotateCw, color: 'text-blue-400', label: 'Create Request' },
  retry_provisioning: { icon: RotateCw, color: 'text-yellow-400', label: 'Retry Provisioning' },
  mark_failed: { icon: AlertTriangle, color: 'text-red-400', label: 'Mark Failed' },
  delete_request: { icon: Trash2, color: 'text-red-400', label: 'Delete Request' },
  restart_node: { icon: RotateCw, color: 'text-blue-400', label: 'Restart Node' },
  stop_node: { icon: Square, color: 'text-yellow-400', label: 'Stop Node' },
  update_settings: { icon: Settings, color: 'text-purple-400', label: 'Update Settings' },
  acknowledge_alert: { icon: AlertTriangle, color: 'text-green-400', label: 'Acknowledge Alert' },
};

export function AuditLogPage() {
  const [logs, setLogs] = useState<AuditLogEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [actionFilter, setActionFilter] = useState('');
  const [search, setSearch] = useState('');

  const pageSize = 25;

  const fetchLogs = async () => {
    setLoading(true);
    const result = await api.audit.list({
      page,
      pageSize,
      action: actionFilter || undefined,
    });
    if (result.success && result.data) {
      setLogs(result.data.items);
      setTotal(result.data.total);
    }
    setLoading(false);
  };

  useEffect(() => {
    fetchLogs();
  }, [page, actionFilter]);

  const filteredLogs = logs.filter((log) => {
    if (search) {
      const query = search.toLowerCase();
      return (
        log.username.toLowerCase().includes(query) ||
        log.action.toLowerCase().includes(query) ||
        log.resource_id?.toLowerCase().includes(query) ||
        log.ip_address.includes(query)
      );
    }
    return true;
  });

  const totalPages = Math.ceil(total / pageSize);

  return (
    <div className="p-8">
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-white">Audit Log</h1>
          <p className="text-dark-400 mt-1">Track all admin actions and system events</p>
        </div>
        <button onClick={fetchLogs} className="btn btn-secondary">
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
                placeholder="Search by user, action, or resource..."
                className="input pl-10"
              />
            </div>
          </div>
          <select
            value={actionFilter}
            onChange={(e) => {
              setActionFilter(e.target.value);
              setPage(1);
            }}
            className="select w-48"
          >
            <option value="">All Actions</option>
            <option value="login">Login</option>
            <option value="logout">Logout</option>
            <option value="retry_provisioning">Retry Provisioning</option>
            <option value="restart_node">Restart Node</option>
            <option value="stop_node">Stop Node</option>
            <option value="update_settings">Update Settings</option>
            <option value="acknowledge_alert">Acknowledge Alert</option>
            <option value="mark_failed">Mark Failed</option>
            <option value="delete_request">Delete Request</option>
          </select>
        </div>
      </div>

      {/* Audit Log Table */}
      <div className="card p-0">
        <div className="table-container">
          <table className="table">
            <thead className="table-header">
              <tr>
                <th>Timestamp</th>
                <th>User</th>
                <th>Action</th>
                <th>Resource</th>
                <th>IP Address</th>
                <th>Details</th>
              </tr>
            </thead>
            <tbody className="table-body">
              {loading ? (
                <tr>
                  <td colSpan={6} className="text-center py-8">
                    <RefreshCw className="w-6 h-6 text-omniphi-500 animate-spin mx-auto" />
                  </td>
                </tr>
              ) : filteredLogs.length === 0 ? (
                <tr>
                  <td colSpan={6} className="text-center py-8 text-dark-400">
                    No audit logs found
                  </td>
                </tr>
              ) : (
                filteredLogs.map((log) => {
                  const config = actionConfig[log.action];
                  const ActionIcon = config?.icon || Clock;

                  return (
                    <tr key={log.id}>
                      <td>
                        <div className="flex flex-col">
                          <span className="text-dark-200">
                            {format(new Date(log.timestamp), 'MMM d, yyyy')}
                          </span>
                          <span className="text-xs text-dark-500">
                            {format(new Date(log.timestamp), 'HH:mm:ss')}
                          </span>
                        </div>
                      </td>
                      <td>
                        <div className="flex items-center space-x-2">
                          <div className="w-8 h-8 bg-dark-700 rounded-full flex items-center justify-center">
                            <User className="w-4 h-4 text-dark-400" />
                          </div>
                          <span className="font-medium text-white">{log.username}</span>
                        </div>
                      </td>
                      <td>
                        <div className="flex items-center space-x-2">
                          <ActionIcon className={`w-4 h-4 ${config?.color || 'text-dark-400'}`} />
                          <span className={config?.color || 'text-dark-200'}>
                            {config?.label || log.action}
                          </span>
                        </div>
                      </td>
                      <td>
                        {log.resource_id ? (
                          <div className="flex flex-col">
                            <span className="text-dark-200 capitalize">{log.resource_type}</span>
                            <span className="text-xs text-dark-500 font-mono">{log.resource_id}</span>
                          </div>
                        ) : (
                          <span className="text-dark-500">-</span>
                        )}
                      </td>
                      <td>
                        <div className="flex items-center space-x-2">
                          <Globe className="w-4 h-4 text-dark-500" />
                          <span className="font-mono text-xs text-dark-300">{log.ip_address}</span>
                        </div>
                      </td>
                      <td>
                        {Object.keys(log.details).length > 0 ? (
                          <button
                            onClick={() => alert(JSON.stringify(log.details, null, 2))}
                            className="btn btn-ghost btn-sm text-xs"
                          >
                            View
                          </button>
                        ) : (
                          <span className="text-dark-500">-</span>
                        )}
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="flex items-center justify-between px-6 py-4 border-t border-dark-700">
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

      {/* Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-4 gap-6 mt-8">
        <div className="card">
          <div className="flex items-center space-x-3">
            <div className="p-2 bg-green-900/30 rounded-lg">
              <LogIn className="w-5 h-5 text-green-400" />
            </div>
            <div>
              <p className="text-2xl font-bold text-white">
                {logs.filter(l => l.action === 'login').length}
              </p>
              <p className="text-sm text-dark-400">Logins Today</p>
            </div>
          </div>
        </div>

        <div className="card">
          <div className="flex items-center space-x-3">
            <div className="p-2 bg-blue-900/30 rounded-lg">
              <RotateCw className="w-5 h-5 text-blue-400" />
            </div>
            <div>
              <p className="text-2xl font-bold text-white">
                {logs.filter(l => l.action === 'restart_node' || l.action === 'retry_provisioning').length}
              </p>
              <p className="text-sm text-dark-400">Restarts/Retries</p>
            </div>
          </div>
        </div>

        <div className="card">
          <div className="flex items-center space-x-3">
            <div className="p-2 bg-purple-900/30 rounded-lg">
              <Settings className="w-5 h-5 text-purple-400" />
            </div>
            <div>
              <p className="text-2xl font-bold text-white">
                {logs.filter(l => l.action === 'update_settings').length}
              </p>
              <p className="text-sm text-dark-400">Config Changes</p>
            </div>
          </div>
        </div>

        <div className="card">
          <div className="flex items-center space-x-3">
            <div className="p-2 bg-red-900/30 rounded-lg">
              <AlertTriangle className="w-5 h-5 text-red-400" />
            </div>
            <div>
              <p className="text-2xl font-bold text-white">
                {logs.filter(l => l.action === 'mark_failed' || l.action === 'delete_request').length}
              </p>
              <p className="text-sm text-dark-400">Failures/Deletes</p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

export default AuditLogPage;
