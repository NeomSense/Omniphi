/**
 * Validator Setup Requests Page
 */

import { useEffect, useState } from 'react';
import { format, formatDistanceToNow } from 'date-fns';
import {
  Search,
  RefreshCw,
  Eye,
  RotateCw,
  Trash2,
  XCircle,
  CheckCircle,
  Clock,
  AlertTriangle,
  Cloud,
  Monitor,
  X,
  ChevronLeft,
  ChevronRight,
} from 'lucide-react';
import { api } from '../services/api';
import type { SetupRequest, SetupRequestDetail, SetupRequestStatus } from '../types';

const statusConfig: Record<SetupRequestStatus, { color: string; icon: typeof Clock }> = {
  pending: { color: 'badge-neutral', icon: Clock },
  provisioning: { color: 'badge-info', icon: RefreshCw },
  ready: { color: 'badge-success', icon: CheckCircle },
  active: { color: 'badge-success', icon: CheckCircle },
  failed: { color: 'badge-error', icon: XCircle },
  stopped: { color: 'badge-warning', icon: AlertTriangle },
};

export function SetupRequestsPage() {
  const [requests, setRequests] = useState<SetupRequest[]>([]);
  const [loading, setLoading] = useState(true);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [selectedRequest, setSelectedRequest] = useState<SetupRequestDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);

  const pageSize = 15;

  const fetchRequests = async () => {
    setLoading(true);
    const result = await api.setupRequests.list({
      page,
      pageSize,
      status: statusFilter || undefined,
      search: search || undefined,
    });
    if (result.success && result.data) {
      setRequests(result.data.items);
      setTotal(result.data.total);
    }
    setLoading(false);
  };

  useEffect(() => {
    fetchRequests();
  }, [page, statusFilter]);

  const handleSearch = () => {
    setPage(1);
    fetchRequests();
  };

  const handleViewDetails = async (id: string) => {
    setDetailLoading(true);
    const result = await api.setupRequests.get(id);
    if (result.success && result.data) {
      setSelectedRequest(result.data);
    }
    setDetailLoading(false);
  };

  const handleRetry = async (id: string) => {
    if (confirm('Are you sure you want to retry provisioning for this request?')) {
      await api.setupRequests.retry(id);
      fetchRequests();
    }
  };

  const handleMarkFailed = async (id: string) => {
    const reason = prompt('Enter failure reason:');
    if (reason) {
      await api.setupRequests.markFailed(id, reason);
      fetchRequests();
    }
  };

  const handleDelete = async (id: string) => {
    if (confirm('Are you sure you want to delete this request? This action cannot be undone.')) {
      await api.setupRequests.delete(id);
      fetchRequests();
    }
  };

  const totalPages = Math.ceil(total / pageSize);

  return (
    <div className="p-8">
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-white">Setup Requests</h1>
          <p className="text-dark-400 mt-1">Manage validator provisioning requests</p>
        </div>
        <button onClick={fetchRequests} className="btn btn-secondary">
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
                onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
                placeholder="Search by wallet, moniker, or ID..."
                className="input pl-10"
              />
            </div>
          </div>
          <select
            value={statusFilter}
            onChange={(e) => {
              setStatusFilter(e.target.value);
              setPage(1);
            }}
            className="select w-40"
          >
            <option value="">All Status</option>
            <option value="pending">Pending</option>
            <option value="provisioning">Provisioning</option>
            <option value="ready">Ready</option>
            <option value="active">Active</option>
            <option value="failed">Failed</option>
            <option value="stopped">Stopped</option>
          </select>
          <button onClick={handleSearch} className="btn btn-primary">
            Search
          </button>
        </div>
      </div>

      {/* Table */}
      <div className="card p-0">
        <div className="table-container">
          <table className="table">
            <thead className="table-header">
              <tr>
                <th>Request ID</th>
                <th>Wallet</th>
                <th>Moniker</th>
                <th>Mode</th>
                <th>Provider</th>
                <th>Status</th>
                <th>Created</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody className="table-body">
              {loading ? (
                <tr>
                  <td colSpan={8} className="text-center py-8">
                    <RefreshCw className="w-6 h-6 text-omniphi-500 animate-spin mx-auto" />
                  </td>
                </tr>
              ) : requests.length === 0 ? (
                <tr>
                  <td colSpan={8} className="text-center py-8 text-dark-400">
                    No requests found
                  </td>
                </tr>
              ) : (
                requests.map((req) => {
                  const StatusIcon = statusConfig[req.status].icon;
                  return (
                    <tr key={req.id}>
                      <td className="font-mono text-xs">{req.id}</td>
                      <td className="font-mono text-xs max-w-[150px] truncate">{req.wallet_address}</td>
                      <td>{req.moniker}</td>
                      <td>
                        <span className="flex items-center space-x-1">
                          {req.run_mode === 'cloud' ? (
                            <Cloud className="w-4 h-4 text-blue-400" />
                          ) : (
                            <Monitor className="w-4 h-4 text-green-400" />
                          )}
                          <span className="capitalize">{req.run_mode}</span>
                        </span>
                      </td>
                      <td className="capitalize">{req.provider}</td>
                      <td>
                        <span className={`badge ${statusConfig[req.status].color}`}>
                          <StatusIcon className="w-3 h-3 mr-1" />
                          {req.status}
                        </span>
                      </td>
                      <td className="text-dark-400 text-xs">
                        {formatDistanceToNow(new Date(req.created_at), { addSuffix: true })}
                      </td>
                      <td>
                        <div className="flex items-center space-x-2">
                          <button
                            onClick={() => handleViewDetails(req.id)}
                            className="btn btn-ghost btn-icon btn-sm"
                            title="View Details"
                          >
                            <Eye className="w-4 h-4" />
                          </button>
                          {(req.status === 'failed' || req.status === 'pending') && (
                            <button
                              onClick={() => handleRetry(req.id)}
                              className="btn btn-ghost btn-icon btn-sm"
                              title="Retry Provisioning"
                            >
                              <RotateCw className="w-4 h-4" />
                            </button>
                          )}
                          {req.status !== 'failed' && req.status !== 'stopped' && (
                            <button
                              onClick={() => handleMarkFailed(req.id)}
                              className="btn btn-ghost btn-icon btn-sm text-yellow-400"
                              title="Mark as Failed"
                            >
                              <AlertTriangle className="w-4 h-4" />
                            </button>
                          )}
                          <button
                            onClick={() => handleDelete(req.id)}
                            className="btn btn-ghost btn-icon btn-sm text-red-400"
                            title="Delete"
                          >
                            <Trash2 className="w-4 h-4" />
                          </button>
                        </div>
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

      {/* Detail Modal */}
      {selectedRequest && (
        <RequestDetailModal
          request={selectedRequest}
          loading={detailLoading}
          onClose={() => setSelectedRequest(null)}
        />
      )}
    </div>
  );
}

function RequestDetailModal({
  request,
  loading,
  onClose,
}: {
  request: SetupRequestDetail;
  loading: boolean;
  onClose: () => void;
}) {
  if (loading) {
    return (
      <div className="modal-overlay" onClick={onClose}>
        <div className="modal max-w-3xl" onClick={(e) => e.stopPropagation()}>
          <div className="p-8 text-center">
            <RefreshCw className="w-8 h-8 text-omniphi-500 animate-spin mx-auto" />
          </div>
        </div>
      </div>
    );
  }

  const StatusIcon = statusConfig[request.status].icon;

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal max-w-3xl" onClick={(e) => e.stopPropagation()}>
        <div className="modal-header">
          <div>
            <h2 className="text-lg font-semibold text-white">Request Details</h2>
            <p className="text-sm text-dark-400 font-mono">{request.id}</p>
          </div>
          <button onClick={onClose} className="btn btn-ghost btn-icon">
            <X className="w-5 h-5" />
          </button>
        </div>

        <div className="modal-body max-h-[70vh] overflow-y-auto">
          {/* Status Banner */}
          <div className={`p-4 rounded-lg mb-6 ${request.status === 'active' ? 'bg-green-900/20' : request.status === 'failed' ? 'bg-red-900/20' : 'bg-dark-800'}`}>
            <div className="flex items-center space-x-3">
              <StatusIcon className={`w-6 h-6 ${request.status === 'active' ? 'text-green-400' : request.status === 'failed' ? 'text-red-400' : 'text-dark-400'}`} />
              <div>
                <p className="font-medium text-white capitalize">{request.status}</p>
                {request.error_message && (
                  <p className="text-sm text-red-400 mt-1">{request.error_message}</p>
                )}
              </div>
            </div>
          </div>

          {/* Details Grid */}
          <div className="grid grid-cols-2 gap-4 mb-6">
            <div className="bg-dark-800 p-3 rounded-lg">
              <p className="text-xs text-dark-400 mb-1">Wallet Address</p>
              <p className="text-sm font-mono text-dark-200 break-all">{request.wallet_address}</p>
            </div>
            <div className="bg-dark-800 p-3 rounded-lg">
              <p className="text-xs text-dark-400 mb-1">Moniker</p>
              <p className="text-sm text-dark-200">{request.moniker}</p>
            </div>
            <div className="bg-dark-800 p-3 rounded-lg">
              <p className="text-xs text-dark-400 mb-1">Run Mode</p>
              <p className="text-sm text-dark-200 capitalize flex items-center">
                {request.run_mode === 'cloud' ? (
                  <Cloud className="w-4 h-4 mr-2 text-blue-400" />
                ) : (
                  <Monitor className="w-4 h-4 mr-2 text-green-400" />
                )}
                {request.run_mode}
              </p>
            </div>
            <div className="bg-dark-800 p-3 rounded-lg">
              <p className="text-xs text-dark-400 mb-1">Provider</p>
              <p className="text-sm text-dark-200 capitalize">{request.provider}</p>
            </div>
            <div className="bg-dark-800 p-3 rounded-lg">
              <p className="text-xs text-dark-400 mb-1">Chain ID</p>
              <p className="text-sm text-dark-200">{request.chain_id}</p>
            </div>
            <div className="bg-dark-800 p-3 rounded-lg">
              <p className="text-xs text-dark-400 mb-1">Retry Count</p>
              <p className="text-sm text-dark-200">{request.retry_count}</p>
            </div>
          </div>

          {/* Consensus Pubkey */}
          {request.consensus_pubkey && (
            <div className="bg-dark-800 p-3 rounded-lg mb-6">
              <p className="text-xs text-dark-400 mb-1">Consensus Public Key</p>
              <p className="text-sm font-mono text-dark-200 break-all">{request.consensus_pubkey}</p>
            </div>
          )}

          {/* Provisioning History */}
          <div className="mb-6">
            <h3 className="text-sm font-medium text-white mb-3">Provisioning History</h3>
            <div className="space-y-2">
              {request.provisioning_history.length === 0 ? (
                <p className="text-sm text-dark-400">No events recorded</p>
              ) : (
                request.provisioning_history.map((event) => (
                  <div key={event.id} className="flex items-start space-x-3 p-3 bg-dark-800 rounded-lg">
                    <div className={`w-2 h-2 rounded-full mt-1.5 ${
                      event.event_type === 'completed' ? 'bg-green-500' :
                      event.event_type === 'failed' ? 'bg-red-500' :
                      event.event_type === 'retry' ? 'bg-yellow-500' :
                      'bg-blue-500'
                    }`} />
                    <div className="flex-1">
                      <div className="flex items-center justify-between">
                        <p className="text-sm text-dark-200">{event.message}</p>
                        <p className="text-xs text-dark-500">
                          {format(new Date(event.timestamp), 'MMM d, HH:mm:ss')}
                        </p>
                      </div>
                    </div>
                  </div>
                ))
              )}
            </div>
          </div>

          {/* Timestamps */}
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <p className="text-dark-400">Created</p>
              <p className="text-dark-200">{format(new Date(request.created_at), 'PPpp')}</p>
            </div>
            <div>
              <p className="text-dark-400">Last Updated</p>
              <p className="text-dark-200">{format(new Date(request.updated_at), 'PPpp')}</p>
            </div>
          </div>
        </div>

        <div className="modal-footer">
          <button onClick={onClose} className="btn btn-secondary">
            Close
          </button>
        </div>
      </div>
    </div>
  );
}

export default SetupRequestsPage;
