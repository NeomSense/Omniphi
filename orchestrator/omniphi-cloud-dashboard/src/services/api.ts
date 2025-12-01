/**
 * Omniphi Cloud Dashboard - API Service Layer
 */

import type {
  CloudNode,
  RegionData,
  FleetMetrics,
  Incident,
  ChainUpgrade,
  CostBreakdown,
  MachineCost,
  LogEntry,
  ApiResponse,
  Region,
  NodeUpgradeStatus,
} from '../types';

const API_BASE = 'http://localhost:8000/api/v1';

async function fetchApi<T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<ApiResponse<T>> {
  try {
    const response = await fetch(`${API_BASE}${endpoint}`, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
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
  } catch {
    return { success: false, error: 'Network error' };
  }
}

// Fleet API
export const fleetApi = {
  async getMetrics(): Promise<ApiResponse<FleetMetrics>> {
    const result = await fetchApi<FleetMetrics>('/cloud/fleet/metrics');
    if (!result.success) {
      return { success: true, data: generateMockFleetMetrics() };
    }
    return result;
  },

  async getNodes(region?: Region): Promise<ApiResponse<CloudNode[]>> {
    const query = region ? `?region=${region}` : '';
    const result = await fetchApi<CloudNode[]>(`/cloud/nodes${query}`);
    if (!result.success) {
      return { success: true, data: generateMockNodes(region) };
    }
    return result;
  },

  async getNode(nodeId: string): Promise<ApiResponse<CloudNode>> {
    const result = await fetchApi<CloudNode>(`/cloud/nodes/${nodeId}`);
    if (!result.success) {
      const nodes = generateMockNodes();
      const node = nodes.find((n) => n.id === nodeId) || nodes[0];
      return { success: true, data: node };
    }
    return result;
  },

  async restartNode(nodeId: string): Promise<ApiResponse<void>> {
    return fetchApi<void>(`/cloud/nodes/${nodeId}/restart`, { method: 'POST' });
  },

  async terminateNode(nodeId: string): Promise<ApiResponse<void>> {
    return fetchApi<void>(`/cloud/nodes/${nodeId}/terminate`, { method: 'POST' });
  },

  async migrateNode(nodeId: string, targetRegion: Region): Promise<ApiResponse<void>> {
    return fetchApi<void>(`/cloud/nodes/${nodeId}/migrate`, {
      method: 'POST',
      body: JSON.stringify({ target_region: targetRegion }),
    });
  },

  async reprovisionNode(nodeId: string): Promise<ApiResponse<void>> {
    return fetchApi<void>(`/cloud/nodes/${nodeId}/reprovision`, { method: 'POST' });
  },
};

// Region API
export const regionApi = {
  async getAll(): Promise<ApiResponse<RegionData[]>> {
    const result = await fetchApi<RegionData[]>('/cloud/regions');
    if (!result.success) {
      return { success: true, data: generateMockRegions() };
    }
    return result;
  },

  async get(regionId: Region): Promise<ApiResponse<RegionData>> {
    const result = await fetchApi<RegionData>(`/cloud/regions/${regionId}`);
    if (!result.success) {
      const regions = generateMockRegions();
      return { success: true, data: regions.find((r) => r.id === regionId) || regions[0] };
    }
    return result;
  },
};

// Incident API
export const incidentApi = {
  async getAll(status?: string): Promise<ApiResponse<Incident[]>> {
    const query = status ? `?status=${status}` : '';
    const result = await fetchApi<Incident[]>(`/cloud/incidents${query}`);
    if (!result.success) {
      return { success: true, data: generateMockIncidents() };
    }
    return result;
  },

  async acknowledge(incidentId: string): Promise<ApiResponse<void>> {
    return fetchApi<void>(`/cloud/incidents/${incidentId}/acknowledge`, { method: 'POST' });
  },

  async triggerAutoRepair(incidentId: string): Promise<ApiResponse<void>> {
    return fetchApi<void>(`/cloud/incidents/${incidentId}/auto-repair`, { method: 'POST' });
  },

  async resolve(incidentId: string): Promise<ApiResponse<void>> {
    return fetchApi<void>(`/cloud/incidents/${incidentId}/resolve`, { method: 'POST' });
  },
};

// Upgrade API
export const upgradeApi = {
  async getAll(): Promise<ApiResponse<ChainUpgrade[]>> {
    const result = await fetchApi<ChainUpgrade[]>('/cloud/upgrades');
    if (!result.success) {
      return { success: true, data: generateMockUpgrades() };
    }
    return result;
  },

  async get(upgradeId: string): Promise<ApiResponse<ChainUpgrade>> {
    const result = await fetchApi<ChainUpgrade>(`/cloud/upgrades/${upgradeId}`);
    if (!result.success) {
      const upgrades = generateMockUpgrades();
      return { success: true, data: upgrades.find((u) => u.id === upgradeId) || upgrades[0] };
    }
    return result;
  },

  async getNodeStatuses(upgradeId: string): Promise<ApiResponse<NodeUpgradeStatus[]>> {
    const result = await fetchApi<NodeUpgradeStatus[]>(`/cloud/upgrades/${upgradeId}/nodes`);
    if (!result.success) {
      return { success: true, data: generateMockNodeUpgradeStatuses() };
    }
    return result;
  },

  async rollback(upgradeId: string): Promise<ApiResponse<void>> {
    return fetchApi<void>(`/cloud/upgrades/${upgradeId}/rollback`, { method: 'POST' });
  },
};

// Cost API
export const costApi = {
  async getBreakdown(): Promise<ApiResponse<CostBreakdown>> {
    const result = await fetchApi<CostBreakdown>('/cloud/costs/breakdown');
    if (!result.success) {
      return { success: true, data: generateMockCostBreakdown() };
    }
    return result;
  },

  async getMachineCosts(): Promise<ApiResponse<MachineCost[]>> {
    const result = await fetchApi<MachineCost[]>('/cloud/costs/machines');
    if (!result.success) {
      return { success: true, data: generateMockMachineCosts() };
    }
    return result;
  },
};

// Logs API
export const logsApi = {
  async getNodeLogs(nodeId: string, source?: string): Promise<ApiResponse<LogEntry[]>> {
    const query = source ? `?source=${source}` : '';
    const result = await fetchApi<LogEntry[]>(`/cloud/nodes/${nodeId}/logs${query}`);
    if (!result.success) {
      return { success: true, data: generateMockLogs(nodeId) };
    }
    return result;
  },
};

// Mock Data Generators
function generateMockFleetMetrics(): FleetMetrics {
  return {
    total_validators: 1250,
    active_nodes: 1235,
    total_cpu_cores: 5000,
    cpu_usage_percent: 62,
    total_ram_gb: 20000,
    ram_usage_percent: 71,
    total_disk_tb: 500,
    disk_usage_percent: 45,
    avg_uptime_percent: 99.94,
    incidents_24h: 3,
    cost_per_validator: 89,
    total_monthly_cost: 111250,
    nodes_by_status: {
      healthy: 1180,
      warning: 42,
      error: 8,
      offline: 5,
      provisioning: 15,
    },
    nodes_by_region: {
      'us-east': 420,
      'us-west': 380,
      'eu-central': 280,
      'asia-pacific': 170,
    },
  };
}

function generateMockNodes(filterRegion?: Region): CloudNode[] {
  const regions: Region[] = ['us-east', 'us-west', 'eu-central', 'asia-pacific'];
  const statuses: CloudNode['status'][] = ['healthy', 'healthy', 'healthy', 'healthy', 'warning', 'error'];
  const syncStatuses: CloudNode['sync_status'][] = ['synced', 'synced', 'synced', 'syncing', 'behind'];

  const nodes: CloudNode[] = [];

  for (let i = 1; i <= 50; i++) {
    const region = regions[i % regions.length];
    if (filterRegion && region !== filterRegion) continue;

    const status = statuses[Math.floor(Math.random() * statuses.length)];
    const syncStatus = syncStatuses[Math.floor(Math.random() * syncStatuses.length)];
    const blockHeight = 1567000 + Math.floor(Math.random() * 1000);
    const chainHeight = blockHeight + (syncStatus === 'synced' ? 0 : Math.floor(Math.random() * 100));

    nodes.push({
      id: `node-${i.toString().padStart(4, '0')}`,
      node_id: `omni-cloud-${region}-${i.toString().padStart(3, '0')}`,
      validator_address: `omnivaloper1${Math.random().toString(36).substring(2, 15)}`,
      moniker: `OmniphiCloud-${region.toUpperCase()}-${i}`,
      host_machine_id: `host-${region}-${Math.floor(i / 10).toString().padStart(2, '0')}`,
      vm_id: `vm-${i.toString().padStart(6, '0')}`,
      region,
      status,
      sync_status: syncStatus,
      cpu_percent: 40 + Math.random() * 40,
      ram_percent: 50 + Math.random() * 35,
      ram_used_gb: 12 + Math.random() * 4,
      ram_total_gb: 16,
      disk_percent: 30 + Math.random() * 30,
      disk_used_gb: 150 + Math.random() * 100,
      disk_total_gb: 500,
      network_in_mbps: 50 + Math.random() * 150,
      network_out_mbps: 30 + Math.random() * 100,
      block_height: blockHeight,
      latest_chain_height: chainHeight,
      blocks_behind: chainHeight - blockHeight,
      peers: 30 + Math.floor(Math.random() * 20),
      last_signed_block: blockHeight - Math.floor(Math.random() * 5),
      missed_blocks_24h: status === 'error' ? Math.floor(Math.random() * 10) : 0,
      uptime_percent: status === 'error' ? 95 + Math.random() * 4 : 99.5 + Math.random() * 0.5,
      last_heartbeat: new Date(Date.now() - Math.random() * 60000).toISOString(),
      health_score: status === 'healthy' ? 95 + Math.random() * 5 : status === 'warning' ? 70 + Math.random() * 20 : 40 + Math.random() * 30,
      restart_count_24h: status === 'error' ? Math.floor(Math.random() * 5) : 0,
      monthly_cost: 75 + Math.random() * 50,
      created_at: new Date(Date.now() - Math.random() * 180 * 86400000).toISOString(),
      last_updated: new Date().toISOString(),
    });
  }

  return nodes;
}

function generateMockRegions(): RegionData[] {
  return [
    {
      id: 'us-east',
      name: 'us-east',
      display_name: 'US East (N. Virginia)',
      total_nodes: 420,
      active_nodes: 415,
      max_capacity: 600,
      available_capacity: 180,
      avg_cpu_percent: 58,
      avg_ram_percent: 68,
      p2p_connectivity_score: 98,
      provisioning_queue: 5,
      monthly_cost: 37380,
      cost_per_node: 89,
      latency_ms: 12,
    },
    {
      id: 'us-west',
      name: 'us-west',
      display_name: 'US West (Oregon)',
      total_nodes: 380,
      active_nodes: 375,
      max_capacity: 500,
      available_capacity: 120,
      avg_cpu_percent: 62,
      avg_ram_percent: 72,
      p2p_connectivity_score: 96,
      provisioning_queue: 3,
      monthly_cost: 33820,
      cost_per_node: 89,
      latency_ms: 18,
    },
    {
      id: 'eu-central',
      name: 'eu-central',
      display_name: 'EU Central (Frankfurt)',
      total_nodes: 280,
      active_nodes: 278,
      max_capacity: 400,
      available_capacity: 120,
      avg_cpu_percent: 55,
      avg_ram_percent: 65,
      p2p_connectivity_score: 97,
      provisioning_queue: 2,
      monthly_cost: 26320,
      cost_per_node: 94,
      latency_ms: 25,
    },
    {
      id: 'asia-pacific',
      name: 'asia-pacific',
      display_name: 'Asia Pacific (Singapore)',
      total_nodes: 170,
      active_nodes: 167,
      max_capacity: 300,
      available_capacity: 130,
      avg_cpu_percent: 52,
      avg_ram_percent: 61,
      p2p_connectivity_score: 94,
      provisioning_queue: 5,
      monthly_cost: 16150,
      cost_per_node: 95,
      latency_ms: 45,
    },
  ];
}

function generateMockIncidents(): Incident[] {
  const types: Incident['type'][] = ['health_failure', 'out_of_sync', 'frequent_restarts', 'rpc_failure', 'latency_spike', 'missed_blocks'];
  const regions: Region[] = ['us-east', 'us-west', 'eu-central', 'asia-pacific'];

  return [
    {
      id: 'inc-001',
      type: 'out_of_sync',
      severity: 'high',
      status: 'active',
      title: 'Node out of sync - 50 blocks behind',
      description: 'Node omni-cloud-us-east-042 has fallen 50 blocks behind the chain tip.',
      node_id: 'node-0042',
      node_moniker: 'OmniphiCloud-US-EAST-42',
      region: 'us-east',
      triggered_at: new Date(Date.now() - 15 * 60000).toISOString(),
      auto_repair_available: true,
      auto_repair_attempted: false,
    },
    {
      id: 'inc-002',
      type: 'frequent_restarts',
      severity: 'medium',
      status: 'acknowledged',
      title: 'Frequent container restarts detected',
      description: 'Node omni-cloud-eu-central-015 has restarted 4 times in the last hour.',
      node_id: 'node-0015',
      node_moniker: 'OmniphiCloud-EU-CENTRAL-15',
      region: 'eu-central',
      triggered_at: new Date(Date.now() - 45 * 60000).toISOString(),
      acknowledged_at: new Date(Date.now() - 30 * 60000).toISOString(),
      acknowledged_by: 'sre-team',
      auto_repair_available: true,
      auto_repair_attempted: true,
    },
    {
      id: 'inc-003',
      type: 'health_failure',
      severity: 'critical',
      status: 'active',
      title: 'Node health check failing',
      description: 'Node omni-cloud-asia-pacific-008 is not responding to health checks.',
      node_id: 'node-0008',
      node_moniker: 'OmniphiCloud-ASIA-PACIFIC-8',
      region: 'asia-pacific',
      triggered_at: new Date(Date.now() - 5 * 60000).toISOString(),
      auto_repair_available: true,
      auto_repair_attempted: false,
    },
    ...Array.from({ length: 5 }, (_, i) => ({
      id: `inc-${(i + 4).toString().padStart(3, '0')}`,
      type: types[i % types.length],
      severity: (['low', 'medium', 'high'] as const)[Math.floor(Math.random() * 3)],
      status: 'resolved' as const,
      title: `Resolved: ${types[i % types.length].replace(/_/g, ' ')}`,
      description: `Issue was automatically resolved.`,
      node_id: `node-${(i + 20).toString().padStart(4, '0')}`,
      node_moniker: `OmniphiCloud-${regions[i % regions.length].toUpperCase()}-${i + 20}`,
      region: regions[i % regions.length],
      triggered_at: new Date(Date.now() - (i + 2) * 3600000).toISOString(),
      resolved_at: new Date(Date.now() - (i + 1) * 3600000).toISOString(),
      resolved_by: 'auto-repair',
      auto_repair_available: true,
      auto_repair_attempted: true,
    })),
  ];
}

function generateMockUpgrades(): ChainUpgrade[] {
  return [
    {
      id: 'upgrade-v2.1.0',
      name: 'Nebula Upgrade',
      version: 'v2.1.0',
      new_binary_version: 'omniphid v2.1.0',
      upgrade_height: 1570000,
      current_height: 1567234,
      scheduled_time: new Date(Date.now() + 2 * 86400000).toISOString(),
      status: 'scheduled',
      total_nodes: 1250,
      updated_nodes: 0,
      failed_nodes: 0,
      pending_nodes: 1250,
      completion_percent: 0,
      canary_nodes: ['node-0001', 'node-0002', 'node-0003'],
      canary_completed: false,
      canary_success: false,
      rollback_available: false,
      previous_version: 'v2.0.3',
      upgrade_logs: [],
    },
    {
      id: 'upgrade-v2.0.3',
      name: 'Hotfix Release',
      version: 'v2.0.3',
      new_binary_version: 'omniphid v2.0.3',
      upgrade_height: 1550000,
      current_height: 1567234,
      scheduled_time: new Date(Date.now() - 5 * 86400000).toISOString(),
      status: 'completed',
      total_nodes: 1250,
      updated_nodes: 1248,
      failed_nodes: 2,
      pending_nodes: 0,
      completion_percent: 99.84,
      canary_nodes: ['node-0001', 'node-0002', 'node-0003'],
      canary_completed: true,
      canary_success: true,
      rollback_available: true,
      previous_version: 'v2.0.2',
      upgrade_logs: [
        { timestamp: new Date(Date.now() - 5 * 86400000).toISOString(), level: 'info', message: 'Upgrade started' },
        { timestamp: new Date(Date.now() - 5 * 86400000 + 300000).toISOString(), level: 'info', message: 'Canary nodes updated successfully' },
        { timestamp: new Date(Date.now() - 5 * 86400000 + 600000).toISOString(), level: 'info', message: 'Rolling out to remaining nodes' },
        { timestamp: new Date(Date.now() - 5 * 86400000 + 3600000).toISOString(), level: 'warn', message: '2 nodes failed to update', node_id: 'node-0042' },
        { timestamp: new Date(Date.now() - 5 * 86400000 + 7200000).toISOString(), level: 'info', message: 'Upgrade completed with 99.84% success rate' },
      ],
    },
  ];
}

function generateMockNodeUpgradeStatuses(): NodeUpgradeStatus[] {
  const regions: Region[] = ['us-east', 'us-west', 'eu-central', 'asia-pacific'];
  const statuses: NodeUpgradeStatus['status'][] = ['completed', 'completed', 'completed', 'pending', 'failed'];

  return Array.from({ length: 20 }, (_, i) => ({
    node_id: `node-${(i + 1).toString().padStart(4, '0')}`,
    moniker: `OmniphiCloud-${regions[i % regions.length].toUpperCase()}-${i + 1}`,
    region: regions[i % regions.length],
    status: statuses[i % statuses.length],
    current_version: statuses[i % statuses.length] === 'completed' ? 'v2.0.3' : 'v2.0.2',
    target_version: 'v2.0.3',
    started_at: statuses[i % statuses.length] !== 'pending' ? new Date(Date.now() - 5 * 86400000).toISOString() : undefined,
    completed_at: statuses[i % statuses.length] === 'completed' ? new Date(Date.now() - 5 * 86400000 + 600000).toISOString() : undefined,
    error_message: statuses[i % statuses.length] === 'failed' ? 'Binary download failed' : undefined,
  }));
}

function generateMockCostBreakdown(): CostBreakdown {
  return {
    total_monthly_cost: 111250,
    cost_by_region: {
      'us-east': 37380,
      'us-west': 33820,
      'eu-central': 26320,
      'asia-pacific': 16150,
    },
    cost_by_machine_type: {
      'c5.2xlarge': 45000,
      'c5.xlarge': 35000,
      'm5.xlarge': 25000,
      'r5.large': 6250,
    },
    cost_per_validator_avg: 89,
    compute_cost: 78375,
    storage_cost: 22250,
    network_cost: 10625,
    cost_trend_30d: Array.from({ length: 30 }, (_, i) => ({
      date: new Date(Date.now() - (29 - i) * 86400000).toISOString().split('T')[0],
      cost: 3500 + Math.random() * 500,
    })),
    unused_capacity_cost: 8500,
    potential_savings: 12000,
    auto_scaling_savings: 5000,
  };
}

function generateMockMachineCosts(): MachineCost[] {
  const regions: Region[] = ['us-east', 'us-west', 'eu-central', 'asia-pacific'];
  const machineTypes = ['c5.2xlarge', 'c5.xlarge', 'm5.xlarge'];

  return Array.from({ length: 15 }, (_, i) => ({
    machine_id: `host-${regions[i % regions.length]}-${i.toString().padStart(2, '0')}`,
    machine_type: machineTypes[i % machineTypes.length],
    region: regions[i % regions.length],
    validators_hosted: 8 + Math.floor(Math.random() * 8),
    monthly_cost: 800 + Math.random() * 400,
    cost_per_validator: 70 + Math.random() * 40,
    utilization_percent: 60 + Math.random() * 35,
  }));
}

function generateMockLogs(nodeId: string): LogEntry[] {
  const levels: LogEntry['level'][] = ['info', 'info', 'info', 'warn', 'error', 'debug'];
  const sources: LogEntry['source'][] = ['tendermint', 'app', 'docker', 'system'];
  const messages = [
    'Block committed',
    'New peer connected',
    'Received vote',
    'Applied state transition',
    'Consensus timeout',
    'RPC request received',
    'Memory usage normal',
    'Disk I/O spike detected',
    'Network latency increased',
    'Health check passed',
  ];

  return Array.from({ length: 100 }, (_, i) => ({
    timestamp: new Date(Date.now() - i * 5000).toISOString(),
    level: levels[Math.floor(Math.random() * levels.length)],
    source: sources[Math.floor(Math.random() * sources.length)],
    message: messages[Math.floor(Math.random() * messages.length)] + ` height=${1567234 - i}`,
    node_id: nodeId,
  }));
}

// Export unified API object
export const api = {
  fleet: fleetApi,
  regions: regionApi,
  incidents: incidentApi,
  upgrades: upgradeApi,
  costs: costApi,
  logs: logsApi,
};

export default api;
