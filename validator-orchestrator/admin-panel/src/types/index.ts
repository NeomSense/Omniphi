/**
 * Omniphi Orchestrator Admin Panel - Type Definitions
 */

// Authentication
export interface User {
  id: string;
  username: string;
  email: string;
  role: 'admin' | 'operator' | 'viewer';
  createdAt: string;
}

export interface AuthState {
  user: User | null;
  token: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
}

// Validator Setup Request
export type SetupRequestStatus =
  | 'pending'
  | 'provisioning'
  | 'ready'
  | 'active'
  | 'failed'
  | 'stopped';

export type RunMode = 'cloud' | 'local';

export type CloudProvider = 'aws' | 'gcp' | 'azure' | 'digitalocean' | 'hetzner' | 'vultr';

export interface SetupRequest {
  id: string;
  wallet_address: string;
  run_mode: RunMode;
  provider: CloudProvider | 'local';
  status: SetupRequestStatus;
  consensus_pubkey: string | null;
  moniker: string;
  chain_id: string;
  created_at: string;
  updated_at: string;
  provisioning_started_at: string | null;
  provisioning_completed_at: string | null;
  error_message: string | null;
  retry_count: number;
  metadata: Record<string, unknown>;
}

export interface SetupRequestDetail extends SetupRequest {
  provisioning_history: ProvisioningEvent[];
  orchestrator_logs: LogEntry[];
  node: ValidatorNode | null;
}

export interface ProvisioningEvent {
  id: string;
  request_id: string;
  event_type: 'started' | 'progress' | 'completed' | 'failed' | 'retry';
  message: string;
  details: Record<string, unknown>;
  timestamp: string;
}

// Validator Node
export type NodeStatus = 'running' | 'stopped' | 'error' | 'starting' | 'stopping';

export interface ValidatorNode {
  id: string;
  setup_request_id: string;
  provider: CloudProvider | 'local';
  container_id: string | null;
  vm_id: string | null;
  rpc_endpoint: string;
  p2p_endpoint: string;
  grpc_endpoint: string | null;
  metrics_endpoint: string | null;
  status: NodeStatus;
  cpu_percent: number;
  ram_percent: number;
  ram_used: string;
  disk_percent: number;
  disk_used: string;
  block_height: number;
  syncing: boolean;
  peers: number;
  uptime: number;
  last_health_check: string;
  created_at: string;
  updated_at: string;
}

// System Health
export interface SystemHealth {
  orchestrator_status: 'healthy' | 'degraded' | 'unhealthy';
  orchestrator_uptime: number;
  orchestrator_version: string;
  total_validators: number;
  active_validators: number;
  pending_requests: number;
  provisioning_failures: number;
  success_rate: number;
  avg_provisioning_time: number;
  chain_rpc_status: RPCHealth[];
  recent_errors: RecentError[];
  resource_usage: ResourceUsage;
}

export interface RPCHealth {
  chain_id: string;
  endpoint: string;
  status: 'healthy' | 'degraded' | 'unreachable';
  latency_ms: number;
  block_height: number;
  last_check: string;
}

export interface RecentError {
  id: string;
  type: 'provisioning' | 'health_check' | 'rpc' | 'system';
  message: string;
  request_id: string | null;
  node_id: string | null;
  timestamp: string;
}

export interface ResourceUsage {
  cpu_percent: number;
  memory_percent: number;
  memory_used: string;
  disk_percent: number;
  disk_used: string;
}

// Logs
export type LogLevel = 'debug' | 'info' | 'warn' | 'error';
export type LogSource = 'orchestrator' | 'provisioning' | 'health' | 'docker' | 'chain';

export interface LogEntry {
  id: string;
  level: LogLevel;
  source: LogSource;
  message: string;
  request_id: string | null;
  node_id: string | null;
  metadata: Record<string, unknown>;
  timestamp: string;
}

// Orchestrator Settings
export interface OrchestratorSettings {
  default_provider: CloudProvider;
  max_parallel_jobs: number;
  provisioning_retry_limit: number;
  heartbeat_interval_seconds: number;
  log_retention_days: number;
  chain_rpc_endpoints: ChainRPCConfig[];
  snapshot_urls: SnapshotConfig[];
  alert_thresholds: AlertThresholds;
}

export interface ChainRPCConfig {
  chain_id: string;
  endpoints: string[];
  priority: number;
}

export interface SnapshotConfig {
  chain_id: string;
  url: string;
  type: 'pruned' | 'archive';
  provider: string;
}

export interface AlertThresholds {
  max_provisioning_time_minutes: number;
  min_success_rate_percent: number;
  max_consecutive_failures: number;
  health_check_timeout_seconds: number;
}

// Audit Log
export type AuditAction =
  | 'login'
  | 'logout'
  | 'create_request'
  | 'retry_provisioning'
  | 'mark_failed'
  | 'delete_request'
  | 'restart_node'
  | 'stop_node'
  | 'update_settings'
  | 'acknowledge_alert';

export interface AuditLogEntry {
  id: string;
  user_id: string;
  username: string;
  action: AuditAction;
  resource_type: 'request' | 'node' | 'settings' | 'alert';
  resource_id: string | null;
  details: Record<string, unknown>;
  ip_address: string;
  user_agent: string;
  timestamp: string;
}

// Alerts
export type AlertSeverity = 'info' | 'warning' | 'critical';
export type AlertStatus = 'active' | 'acknowledged' | 'resolved';

export interface Alert {
  id: string;
  severity: AlertSeverity;
  status: AlertStatus;
  type: string;
  title: string;
  message: string;
  request_id: string | null;
  node_id: string | null;
  acknowledged_by: string | null;
  acknowledged_at: string | null;
  resolved_at: string | null;
  created_at: string;
}

// API Responses
export interface ApiResponse<T> {
  success: boolean;
  data?: T;
  error?: string;
  message?: string;
}

export interface PaginatedResponse<T> {
  items: T[];
  total: number;
  page: number;
  page_size: number;
  total_pages: number;
}

// Table & Filter
export interface TableColumn<T> {
  key: keyof T | string;
  header: string;
  sortable?: boolean;
  render?: (value: unknown, row: T) => React.ReactNode;
}

export interface FilterState {
  search: string;
  status?: string;
  provider?: string;
  dateFrom?: string;
  dateTo?: string;
}
