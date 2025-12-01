/**
 * Omniphi Orchestrator Admin Panel - API Service Layer
 *
 * Centralized API communication with the orchestrator backend.
 */

import type {
  SystemHealth,
  SetupRequest,
  SetupRequestDetail,
  ValidatorNode,
  LogEntry,
  OrchestratorSettings,
  AuditLogEntry,
  Alert,
  PaginatedResponse,
  ApiResponse,
} from '../types';

const API_BASE = 'http://localhost:8000/api/v1';

// Get auth token from localStorage
function getAuthToken(): string | null {
  try {
    const stored = localStorage.getItem('omniphi-admin-auth');
    if (stored) {
      const parsed = JSON.parse(stored);
      return parsed.state?.token || null;
    }
  } catch {
    return null;
  }
  return null;
}

// Fetch wrapper with auth
async function fetchWithAuth<T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<ApiResponse<T>> {
  const token = getAuthToken();

  const headers: HeadersInit = {
    'Content-Type': 'application/json',
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
    ...options.headers,
  };

  try {
    const response = await fetch(`${API_BASE}${endpoint}`, {
      ...options,
      headers,
    });

    if (!response.ok) {
      const errorData = await response.json().catch(() => ({}));
      return {
        success: false,
        error: errorData.message || errorData.detail || `HTTP ${response.status}`,
      };
    }

    const data = await response.json();
    return { success: true, data };
  } catch (error: any) {
    return {
      success: false,
      error: error.message || 'Network error',
    };
  }
}

/**
 * System Health API
 */
export const healthApi = {
  async getSystemHealth(): Promise<ApiResponse<SystemHealth>> {
    const result = await fetchWithAuth<SystemHealth>('/health/system');

    // Return mock data if API unavailable
    if (!result.success) {
      return {
        success: true,
        data: generateMockSystemHealth(),
      };
    }
    return result;
  },
};

/**
 * Setup Requests API
 */
export const setupRequestsApi = {
  async list(params?: {
    page?: number;
    pageSize?: number;
    status?: string;
    search?: string;
  }): Promise<ApiResponse<PaginatedResponse<SetupRequest>>> {
    const query = new URLSearchParams();
    if (params?.page) query.set('page', params.page.toString());
    if (params?.pageSize) query.set('page_size', params.pageSize.toString());
    if (params?.status) query.set('status', params.status);
    if (params?.search) query.set('search', params.search);

    const result = await fetchWithAuth<PaginatedResponse<SetupRequest>>(
      `/setup-requests?${query.toString()}`
    );

    if (!result.success) {
      return {
        success: true,
        data: generateMockSetupRequests(),
      };
    }
    return result;
  },

  async get(id: string): Promise<ApiResponse<SetupRequestDetail>> {
    const result = await fetchWithAuth<SetupRequestDetail>(`/setup-requests/${id}`);

    if (!result.success) {
      return {
        success: true,
        data: generateMockSetupRequestDetail(id),
      };
    }
    return result;
  },

  async retry(id: string): Promise<ApiResponse<void>> {
    return fetchWithAuth<void>(`/setup-requests/${id}/retry`, { method: 'POST' });
  },

  async markFailed(id: string, reason: string): Promise<ApiResponse<void>> {
    return fetchWithAuth<void>(`/setup-requests/${id}/mark-failed`, {
      method: 'POST',
      body: JSON.stringify({ reason }),
    });
  },

  async delete(id: string): Promise<ApiResponse<void>> {
    return fetchWithAuth<void>(`/setup-requests/${id}`, { method: 'DELETE' });
  },
};

/**
 * Validator Nodes API
 */
export const nodesApi = {
  async list(params?: {
    page?: number;
    pageSize?: number;
    status?: string;
    provider?: string;
  }): Promise<ApiResponse<PaginatedResponse<ValidatorNode>>> {
    const query = new URLSearchParams();
    if (params?.page) query.set('page', params.page.toString());
    if (params?.pageSize) query.set('page_size', params.pageSize.toString());
    if (params?.status) query.set('status', params.status);
    if (params?.provider) query.set('provider', params.provider);

    const result = await fetchWithAuth<PaginatedResponse<ValidatorNode>>(
      `/nodes?${query.toString()}`
    );

    if (!result.success) {
      return {
        success: true,
        data: generateMockNodes(),
      };
    }
    return result;
  },

  async get(id: string): Promise<ApiResponse<ValidatorNode>> {
    return fetchWithAuth<ValidatorNode>(`/nodes/${id}`);
  },

  async restart(id: string): Promise<ApiResponse<void>> {
    return fetchWithAuth<void>(`/nodes/${id}/restart`, { method: 'POST' });
  },

  async stop(id: string): Promise<ApiResponse<void>> {
    return fetchWithAuth<void>(`/nodes/${id}/stop`, { method: 'POST' });
  },

  async getLogs(id: string, params?: { level?: string; limit?: number }): Promise<ApiResponse<LogEntry[]>> {
    const query = new URLSearchParams();
    if (params?.level) query.set('level', params.level);
    if (params?.limit) query.set('limit', params.limit.toString());

    return fetchWithAuth<LogEntry[]>(`/nodes/${id}/logs?${query.toString()}`);
  },
};

/**
 * Logs API
 */
export const logsApi = {
  async list(params?: {
    source?: string;
    level?: string;
    requestId?: string;
    nodeId?: string;
    limit?: number;
  }): Promise<ApiResponse<LogEntry[]>> {
    const query = new URLSearchParams();
    if (params?.source) query.set('source', params.source);
    if (params?.level) query.set('level', params.level);
    if (params?.requestId) query.set('request_id', params.requestId);
    if (params?.nodeId) query.set('node_id', params.nodeId);
    if (params?.limit) query.set('limit', params.limit.toString());

    const result = await fetchWithAuth<LogEntry[]>(`/logs?${query.toString()}`);

    if (!result.success) {
      return {
        success: true,
        data: generateMockLogs(),
      };
    }
    return result;
  },

  stream(callback: (log: LogEntry) => void): EventSource | null {
    const token = getAuthToken();
    try {
      const eventSource = new EventSource(
        `${API_BASE}/logs/stream${token ? `?token=${token}` : ''}`
      );

      eventSource.onmessage = (event) => {
        try {
          const log = JSON.parse(event.data);
          callback(log);
        } catch {}
      };

      return eventSource;
    } catch {
      return null;
    }
  },

  async download(params?: { source?: string; level?: string }): Promise<Blob | null> {
    const query = new URLSearchParams();
    if (params?.source) query.set('source', params.source);
    if (params?.level) query.set('level', params.level);

    try {
      const response = await fetch(`${API_BASE}/logs/download?${query.toString()}`, {
        headers: {
          Authorization: `Bearer ${getAuthToken()}`,
        },
      });

      if (!response.ok) return null;
      return response.blob();
    } catch {
      return null;
    }
  },
};

/**
 * Settings API
 */
export const settingsApi = {
  async get(): Promise<ApiResponse<OrchestratorSettings>> {
    const result = await fetchWithAuth<OrchestratorSettings>('/settings');

    if (!result.success) {
      return {
        success: true,
        data: generateMockSettings(),
      };
    }
    return result;
  },

  async update(settings: Partial<OrchestratorSettings>): Promise<ApiResponse<OrchestratorSettings>> {
    return fetchWithAuth<OrchestratorSettings>('/settings', {
      method: 'PUT',
      body: JSON.stringify(settings),
    });
  },
};

/**
 * Audit Log API
 */
export const auditApi = {
  async list(params?: {
    page?: number;
    pageSize?: number;
    action?: string;
    userId?: string;
  }): Promise<ApiResponse<PaginatedResponse<AuditLogEntry>>> {
    const query = new URLSearchParams();
    if (params?.page) query.set('page', params.page.toString());
    if (params?.pageSize) query.set('page_size', params.pageSize.toString());
    if (params?.action) query.set('action', params.action);
    if (params?.userId) query.set('user_id', params.userId);

    const result = await fetchWithAuth<PaginatedResponse<AuditLogEntry>>(
      `/audit?${query.toString()}`
    );

    if (!result.success) {
      return {
        success: true,
        data: generateMockAuditLogs(),
      };
    }
    return result;
  },
};

/**
 * Alerts API
 */
export const alertsApi = {
  async list(params?: {
    status?: string;
    severity?: string;
  }): Promise<ApiResponse<Alert[]>> {
    const query = new URLSearchParams();
    if (params?.status) query.set('status', params.status);
    if (params?.severity) query.set('severity', params.severity);

    const result = await fetchWithAuth<Alert[]>(`/alerts?${query.toString()}`);

    if (!result.success) {
      return {
        success: true,
        data: generateMockAlerts(),
      };
    }
    return result;
  },

  async acknowledge(id: string): Promise<ApiResponse<void>> {
    return fetchWithAuth<void>(`/alerts/${id}/acknowledge`, { method: 'POST' });
  },

  async resolve(id: string): Promise<ApiResponse<void>> {
    return fetchWithAuth<void>(`/alerts/${id}/resolve`, { method: 'POST' });
  },
};

// ============ Mock Data Generators ============

function generateMockSystemHealth(): SystemHealth {
  return {
    orchestrator_status: 'healthy',
    orchestrator_uptime: 86400 * 7 + 3600 * 5,
    orchestrator_version: '1.2.0',
    total_validators: 156,
    active_validators: 142,
    pending_requests: 8,
    provisioning_failures: 3,
    success_rate: 97.4,
    avg_provisioning_time: 245,
    chain_rpc_status: [
      {
        chain_id: 'omniphi-mainnet-1',
        endpoint: 'https://rpc.omniphi.network',
        status: 'healthy',
        latency_ms: 45,
        block_height: 1234567,
        last_check: new Date().toISOString(),
      },
      {
        chain_id: 'omniphi-testnet-1',
        endpoint: 'https://rpc.testnet.omniphi.network',
        status: 'healthy',
        latency_ms: 62,
        block_height: 987654,
        last_check: new Date().toISOString(),
      },
    ],
    recent_errors: [
      {
        id: 'err-1',
        type: 'provisioning',
        message: 'AWS instance limit reached in us-east-1',
        request_id: 'req-123',
        node_id: null,
        timestamp: new Date(Date.now() - 3600000).toISOString(),
      },
      {
        id: 'err-2',
        type: 'health_check',
        message: 'Node node-456 failed health check',
        request_id: null,
        node_id: 'node-456',
        timestamp: new Date(Date.now() - 7200000).toISOString(),
      },
    ],
    resource_usage: {
      cpu_percent: 23.5,
      memory_percent: 45.2,
      memory_used: '3.6GB / 8GB',
      disk_percent: 34.1,
      disk_used: '68GB / 200GB',
    },
  };
}

function generateMockSetupRequests(): PaginatedResponse<SetupRequest> {
  const statuses: SetupRequest['status'][] = ['pending', 'provisioning', 'ready', 'active', 'failed'];
  const providers: SetupRequest['provider'][] = ['aws', 'gcp', 'digitalocean', 'local'];

  const items: SetupRequest[] = Array.from({ length: 25 }, (_, i) => ({
    id: `req-${1000 + i}`,
    wallet_address: `omni1${Math.random().toString(36).substring(2, 12)}...${Math.random().toString(36).substring(2, 6)}`,
    run_mode: Math.random() > 0.3 ? 'cloud' : 'local',
    provider: providers[Math.floor(Math.random() * providers.length)],
    status: statuses[Math.floor(Math.random() * statuses.length)],
    consensus_pubkey: Math.random() > 0.5 ? `omnivalconspub1...${Math.random().toString(36).substring(2, 10)}` : null,
    moniker: `validator-${1000 + i}`,
    chain_id: 'omniphi-mainnet-1',
    created_at: new Date(Date.now() - Math.random() * 7 * 86400000).toISOString(),
    updated_at: new Date(Date.now() - Math.random() * 86400000).toISOString(),
    provisioning_started_at: Math.random() > 0.3 ? new Date(Date.now() - Math.random() * 3600000).toISOString() : null,
    provisioning_completed_at: Math.random() > 0.5 ? new Date(Date.now() - Math.random() * 1800000).toISOString() : null,
    error_message: statuses[Math.floor(Math.random() * statuses.length)] === 'failed' ? 'Provisioning timeout exceeded' : null,
    retry_count: Math.floor(Math.random() * 3),
    metadata: {},
  }));

  return {
    items,
    total: 156,
    page: 1,
    page_size: 25,
    total_pages: 7,
  };
}

function generateMockSetupRequestDetail(id: string): SetupRequestDetail {
  return {
    id,
    wallet_address: 'omni1abc123def456...',
    run_mode: 'cloud',
    provider: 'aws',
    status: 'active',
    consensus_pubkey: 'omnivalconspub1zcjduepq...',
    moniker: 'my-validator',
    chain_id: 'omniphi-mainnet-1',
    created_at: new Date(Date.now() - 86400000).toISOString(),
    updated_at: new Date().toISOString(),
    provisioning_started_at: new Date(Date.now() - 3600000).toISOString(),
    provisioning_completed_at: new Date(Date.now() - 1800000).toISOString(),
    error_message: null,
    retry_count: 0,
    metadata: {},
    provisioning_history: [
      {
        id: 'evt-1',
        request_id: id,
        event_type: 'started',
        message: 'Provisioning started',
        details: {},
        timestamp: new Date(Date.now() - 3600000).toISOString(),
      },
      {
        id: 'evt-2',
        request_id: id,
        event_type: 'progress',
        message: 'VM instance created',
        details: { vm_id: 'i-1234567890' },
        timestamp: new Date(Date.now() - 3000000).toISOString(),
      },
      {
        id: 'evt-3',
        request_id: id,
        event_type: 'completed',
        message: 'Provisioning completed successfully',
        details: {},
        timestamp: new Date(Date.now() - 1800000).toISOString(),
      },
    ],
    orchestrator_logs: [],
    node: null,
  };
}

function generateMockNodes(): PaginatedResponse<ValidatorNode> {
  const statuses: ValidatorNode['status'][] = ['running', 'stopped', 'error'];
  const providers: ValidatorNode['provider'][] = ['aws', 'gcp', 'digitalocean', 'local'];

  const items: ValidatorNode[] = Array.from({ length: 20 }, (_, i) => ({
    id: `node-${2000 + i}`,
    setup_request_id: `req-${1000 + i}`,
    provider: providers[Math.floor(Math.random() * providers.length)],
    container_id: Math.random() > 0.5 ? `container-${Math.random().toString(36).substring(2, 14)}` : null,
    vm_id: Math.random() > 0.5 ? `i-${Math.random().toString(36).substring(2, 14)}` : null,
    rpc_endpoint: `http://10.0.${Math.floor(Math.random() * 255)}.${Math.floor(Math.random() * 255)}:26657`,
    p2p_endpoint: `10.0.${Math.floor(Math.random() * 255)}.${Math.floor(Math.random() * 255)}:26656`,
    grpc_endpoint: `10.0.${Math.floor(Math.random() * 255)}.${Math.floor(Math.random() * 255)}:9090`,
    metrics_endpoint: `http://10.0.${Math.floor(Math.random() * 255)}.${Math.floor(Math.random() * 255)}:26660/metrics`,
    status: statuses[Math.floor(Math.random() * statuses.length)],
    cpu_percent: Math.random() * 50 + 5,
    ram_percent: Math.random() * 60 + 10,
    ram_used: `${(Math.random() * 4 + 1).toFixed(1)}GB / 8GB`,
    disk_percent: Math.random() * 40 + 10,
    disk_used: `${Math.floor(Math.random() * 100 + 20)}GB / 200GB`,
    block_height: 1234567 + Math.floor(Math.random() * 1000),
    syncing: Math.random() > 0.9,
    peers: Math.floor(Math.random() * 50 + 10),
    uptime: Math.floor(Math.random() * 86400 * 30),
    last_health_check: new Date(Date.now() - Math.random() * 60000).toISOString(),
    created_at: new Date(Date.now() - Math.random() * 30 * 86400000).toISOString(),
    updated_at: new Date(Date.now() - Math.random() * 60000).toISOString(),
  }));

  return {
    items,
    total: 142,
    page: 1,
    page_size: 20,
    total_pages: 8,
  };
}

function generateMockLogs(): LogEntry[] {
  const levels: LogEntry['level'][] = ['debug', 'info', 'warn', 'error'];
  const sources: LogEntry['source'][] = ['orchestrator', 'provisioning', 'health', 'docker'];
  const messages = [
    'Health check completed successfully',
    'Starting provisioning for request req-1234',
    'VM instance created in us-east-1',
    'Node node-5678 syncing: 98.5%',
    'Container started: posd-validator',
    'RPC endpoint responding normally',
    'Peer connection established',
    'Block height updated: 1234567',
    'Memory usage: 45.2%',
    'CPU usage spike detected',
    'Retrying failed health check',
    'Provisioning timeout warning',
  ];

  return Array.from({ length: 100 }, (_, i) => ({
    id: `log-${Date.now()}-${i}`,
    level: levels[Math.floor(Math.random() * levels.length)],
    source: sources[Math.floor(Math.random() * sources.length)],
    message: messages[Math.floor(Math.random() * messages.length)],
    request_id: Math.random() > 0.5 ? `req-${Math.floor(Math.random() * 1000) + 1000}` : null,
    node_id: Math.random() > 0.5 ? `node-${Math.floor(Math.random() * 100) + 2000}` : null,
    metadata: {},
    timestamp: new Date(Date.now() - i * 5000).toISOString(),
  }));
}

function generateMockSettings(): OrchestratorSettings {
  return {
    default_provider: 'aws',
    max_parallel_jobs: 10,
    provisioning_retry_limit: 3,
    heartbeat_interval_seconds: 30,
    log_retention_days: 30,
    chain_rpc_endpoints: [
      {
        chain_id: 'omniphi-mainnet-1',
        endpoints: ['https://rpc.omniphi.network', 'https://rpc-backup.omniphi.network'],
        priority: 1,
      },
    ],
    snapshot_urls: [
      {
        chain_id: 'omniphi-mainnet-1',
        url: 'https://snapshots.omniphi.network/latest.tar.gz',
        type: 'pruned',
        provider: 'omniphi',
      },
    ],
    alert_thresholds: {
      max_provisioning_time_minutes: 30,
      min_success_rate_percent: 95,
      max_consecutive_failures: 3,
      health_check_timeout_seconds: 60,
    },
  };
}

function generateMockAuditLogs(): PaginatedResponse<AuditLogEntry> {
  const actions: AuditLogEntry['action'][] = [
    'login', 'retry_provisioning', 'restart_node', 'stop_node', 'update_settings', 'acknowledge_alert'
  ];

  const items: AuditLogEntry[] = Array.from({ length: 50 }, (_, i) => ({
    id: `audit-${Date.now()}-${i}`,
    user_id: '1',
    username: 'admin',
    action: actions[Math.floor(Math.random() * actions.length)],
    resource_type: 'node',
    resource_id: `node-${2000 + Math.floor(Math.random() * 100)}`,
    details: {},
    ip_address: '192.168.1.100',
    user_agent: 'Mozilla/5.0',
    timestamp: new Date(Date.now() - i * 60000).toISOString(),
  }));

  return {
    items,
    total: 500,
    page: 1,
    page_size: 50,
    total_pages: 10,
  };
}

function generateMockAlerts(): Alert[] {
  return [
    {
      id: 'alert-1',
      severity: 'critical',
      status: 'active',
      type: 'node_unhealthy',
      title: 'Node Unhealthy',
      message: 'Node node-2045 has failed 3 consecutive health checks',
      request_id: null,
      node_id: 'node-2045',
      acknowledged_by: null,
      acknowledged_at: null,
      resolved_at: null,
      created_at: new Date(Date.now() - 1800000).toISOString(),
    },
    {
      id: 'alert-2',
      severity: 'warning',
      status: 'active',
      type: 'provisioning_stuck',
      title: 'Provisioning Stuck',
      message: 'Request req-1089 has been provisioning for over 30 minutes',
      request_id: 'req-1089',
      node_id: null,
      acknowledged_by: null,
      acknowledged_at: null,
      resolved_at: null,
      created_at: new Date(Date.now() - 3600000).toISOString(),
    },
    {
      id: 'alert-3',
      severity: 'warning',
      status: 'acknowledged',
      type: 'rpc_degraded',
      title: 'RPC Performance Degraded',
      message: 'RPC endpoint latency exceeds 200ms',
      request_id: null,
      node_id: null,
      acknowledged_by: 'admin',
      acknowledged_at: new Date(Date.now() - 600000).toISOString(),
      resolved_at: null,
      created_at: new Date(Date.now() - 7200000).toISOString(),
    },
  ];
}

export const api = {
  health: healthApi,
  setupRequests: setupRequestsApi,
  nodes: nodesApi,
  logs: logsApi,
  settings: settingsApi,
  audit: auditApi,
  alerts: alertsApi,
};

export default api;
