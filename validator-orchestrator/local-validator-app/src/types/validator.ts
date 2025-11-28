// Enhanced Type Definitions for Omniphi Local Validator App

export interface ValidatorStatus {
  moniker: string;
  chain_id: string;
  network_id: string;
  block_height: number;
  syncing: boolean;
  peers: number;
  is_active: boolean;
  jailed: boolean;
  missed_blocks: number;
  last_signature: string | null;
  uptime: number;
  running: boolean;
  error: string | null;
}

export interface ValidatorMetadata {
  operator_address: string;
  validator_address: string;
  consensus_pubkey: string;
  commission_rate: string;
  commission_max_rate: string;
  commission_max_change_rate: string;
  min_self_delegation: string;
  self_delegation: string;
  delegator_shares: string;
  tokens: string;
  voting_power: string;
  keybase_identity?: string;
  website?: string;
  details?: string;
  security_contact?: string;
}

export interface NodeHealth {
  cpu: number; // percentage
  ram: string; // e.g., "423MB / 16GB"
  ram_percent: number;
  disk: string; // e.g., "3.4GB / 120GB"
  disk_percent: number;
  db_size: string;
  net_in: string; // e.g., "1.2MB/s"
  net_out: string;
  node_id: string;
  rpc_port: number;
  p2p_port: number;
  grpc_port?: number;
}

export interface RewardsInfo {
  daily: string;
  weekly: string;
  monthly: string;
  apr: string;
  apy?: string;
  next_reward: string;
  total_rewards?: string;
  unclaimed_rewards?: string;
}

export interface PoCScore {
  total_score: number;
  reliability: number;
  contributions: number;
  governance: number;
  tier: 'Bronze' | 'Silver' | 'Gold' | 'Platinum';
  tier_color: string;
  rank?: number;
  history?: PoCHistoryEntry[];
}

export interface PoCHistoryEntry {
  timestamp: string;
  score: number;
  change: number;
}

export interface UpgradeInfo {
  current_version: string;
  latest_version: string;
  upgrade_height?: number;
  remaining_blocks?: number;
  upgrade_name?: string;
  upgrade_info?: string;
  should_upgrade: boolean;
  is_urgent: boolean;
}

export interface SlashingInfo {
  missed_blocks_window: number;
  missed_blocks_count: number;
  missed_blocks_percent: number;
  double_sign_protection: boolean;
  last_double_sign_check: string | null;
  slashing_risk: 'low' | 'medium' | 'high';
  tombstoned: boolean;
}

export interface HeartbeatInfo {
  orchestrator_url: string | null;
  wallet_address: string | null;
  last_heartbeat: string | null;
  heartbeat_interval: number; // seconds
  heartbeat_status: 'success' | 'failed' | 'pending' | 'never';
  last_response_code: number | null;
  auto_heartbeat: boolean;
}

export interface ValidatorConfig {
  rpc_port: number;
  p2p_port: number;
  grpc_port?: number;
  auto_update: boolean;
  pruning_mode: 'default' | 'nothing' | 'everything' | 'custom';
  snapshot_interval: number;
  heartbeat_interval: number;
  data_folder: string;
  log_level: 'info' | 'debug' | 'warn' | 'error';
}

export interface LogEntry {
  timestamp: string;
  level: 'info' | 'debug' | 'warn' | 'error';
  message: string;
  source?: 'tendermint' | 'abci' | 'app' | 'upgrade';
}

export interface ConsensusKey {
  type: string;
  value: string; // base64 public key
  address: string;
  fingerprint?: string;
}

// API Response Types
export interface ApiResponse<T> {
  success: boolean;
  data?: T;
  error?: string;
}

// Chart Data Types
export interface ChartDataPoint {
  timestamp: number;
  value: number;
  label?: string;
}

export interface BlockHeightHistory {
  timestamps: number[];
  heights: number[];
}

export interface RewardsHistory {
  daily: ChartDataPoint[];
  weekly: ChartDataPoint[];
  monthly: ChartDataPoint[];
}

// UI State Types
export type StatusBadgeVariant = 'success' | 'warning' | 'error' | 'info';

export interface DashboardState {
  status: ValidatorStatus | null;
  metadata: ValidatorMetadata | null;
  health: NodeHealth | null;
  rewards: RewardsInfo | null;
  poc: PoCScore | null;
  upgrade: UpgradeInfo | null;
  slashing: SlashingInfo | null;
  heartbeat: HeartbeatInfo | null;
  loading: boolean;
  error: string | null;
  lastUpdate: number | null;
}
