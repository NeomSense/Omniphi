/**
 * Omniphi Cloud Internal Dashboard - Type Definitions
 */

// Node Types
export type NodeStatus = 'healthy' | 'warning' | 'error' | 'offline' | 'provisioning';
export type SyncStatus = 'synced' | 'syncing' | 'behind' | 'stalled';

export interface CloudNode {
  id: string;
  node_id: string;
  validator_address: string;
  moniker: string;
  host_machine_id: string;
  vm_id: string;
  region: Region;

  // Status
  status: NodeStatus;
  sync_status: SyncStatus;

  // Metrics
  cpu_percent: number;
  ram_percent: number;
  ram_used_gb: number;
  ram_total_gb: number;
  disk_percent: number;
  disk_used_gb: number;
  disk_total_gb: number;
  network_in_mbps: number;
  network_out_mbps: number;

  // Chain metrics
  block_height: number;
  latest_chain_height: number;
  blocks_behind: number;
  peers: number;
  last_signed_block: number;
  missed_blocks_24h: number;

  // Health
  uptime_percent: number;
  last_heartbeat: string;
  health_score: number;
  restart_count_24h: number;

  // Cost
  monthly_cost: number;

  // Timestamps
  created_at: string;
  last_updated: string;
}

// Region Types
export type Region = 'us-east' | 'us-west' | 'eu-central' | 'asia-pacific';

export interface RegionData {
  id: Region;
  name: string;
  display_name: string;
  total_nodes: number;
  active_nodes: number;
  max_capacity: number;
  available_capacity: number;
  avg_cpu_percent: number;
  avg_ram_percent: number;
  p2p_connectivity_score: number;
  provisioning_queue: number;
  monthly_cost: number;
  cost_per_node: number;
  latency_ms: number;
}

// Fleet Metrics
export interface FleetMetrics {
  total_validators: number;
  active_nodes: number;
  total_cpu_cores: number;
  cpu_usage_percent: number;
  total_ram_gb: number;
  ram_usage_percent: number;
  total_disk_tb: number;
  disk_usage_percent: number;
  avg_uptime_percent: number;
  incidents_24h: number;
  cost_per_validator: number;
  total_monthly_cost: number;
  nodes_by_status: {
    healthy: number;
    warning: number;
    error: number;
    offline: number;
    provisioning: number;
  };
  nodes_by_region: Record<Region, number>;
}

// Incident Types
export type IncidentSeverity = 'critical' | 'high' | 'medium' | 'low';
export type IncidentStatus = 'active' | 'acknowledged' | 'resolved';
export type IncidentType =
  | 'health_failure'
  | 'out_of_sync'
  | 'frequent_restarts'
  | 'network_partition'
  | 'rpc_failure'
  | 'latency_spike'
  | 'disk_full'
  | 'memory_pressure'
  | 'missed_blocks';

export interface Incident {
  id: string;
  type: IncidentType;
  severity: IncidentSeverity;
  status: IncidentStatus;
  title: string;
  description: string;
  node_id: string;
  node_moniker: string;
  region: Region;
  triggered_at: string;
  acknowledged_at?: string;
  acknowledged_by?: string;
  resolved_at?: string;
  resolved_by?: string;
  auto_repair_available: boolean;
  auto_repair_attempted: boolean;
}

// Upgrade Types
export type UpgradeStatus = 'scheduled' | 'in_progress' | 'completed' | 'failed' | 'rolled_back';

export interface ChainUpgrade {
  id: string;
  name: string;
  version: string;
  new_binary_version: string;
  upgrade_height: number;
  current_height: number;
  scheduled_time: string;
  status: UpgradeStatus;

  // Progress
  total_nodes: number;
  updated_nodes: number;
  failed_nodes: number;
  pending_nodes: number;
  completion_percent: number;

  // Canary
  canary_nodes: string[];
  canary_completed: boolean;
  canary_success: boolean;

  // Rollback
  rollback_available: boolean;
  previous_version: string;

  // Logs
  upgrade_logs: UpgradeLog[];
}

export interface UpgradeLog {
  timestamp: string;
  level: 'info' | 'warn' | 'error';
  message: string;
  node_id?: string;
}

export interface NodeUpgradeStatus {
  node_id: string;
  moniker: string;
  region: Region;
  status: 'pending' | 'updating' | 'completed' | 'failed';
  current_version: string;
  target_version: string;
  started_at?: string;
  completed_at?: string;
  error_message?: string;
}

// Cost Types
export interface CostBreakdown {
  total_monthly_cost: number;
  cost_by_region: Record<Region, number>;
  cost_by_machine_type: Record<string, number>;
  cost_per_validator_avg: number;
  compute_cost: number;
  storage_cost: number;
  network_cost: number;

  // Trends
  cost_trend_30d: CostDataPoint[];

  // Savings
  unused_capacity_cost: number;
  potential_savings: number;
  auto_scaling_savings: number;
}

export interface CostDataPoint {
  date: string;
  cost: number;
}

export interface MachineCost {
  machine_id: string;
  machine_type: string;
  region: Region;
  validators_hosted: number;
  monthly_cost: number;
  cost_per_validator: number;
  utilization_percent: number;
}

// Log Types
export type LogLevel = 'info' | 'warn' | 'error' | 'debug';
export type LogSource = 'docker' | 'tendermint' | 'app' | 'system';

export interface LogEntry {
  timestamp: string;
  level: LogLevel;
  source: LogSource;
  message: string;
  node_id?: string;
  metadata?: Record<string, unknown>;
}

// API Response Types
export interface ApiResponse<T> {
  success: boolean;
  data?: T;
  error?: string;
}

export interface PaginatedResponse<T> {
  items: T[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}

// Action Types
export interface NodeAction {
  type: 'restart' | 'terminate' | 'migrate' | 're-provision';
  node_id: string;
  status: 'pending' | 'in_progress' | 'completed' | 'failed';
  started_at: string;
  completed_at?: string;
  error_message?: string;
}
