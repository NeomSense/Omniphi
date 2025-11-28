/**
 * Validator Marketplace - Type Definitions
 */

// Provider Types
export type ProviderType = 'official' | 'community' | 'decentralized';
export type InfrastructureType = 'cloud' | 'bare_metal' | 'decentralized';
export type EnergyRating = 'A' | 'B' | 'C' | 'D' | 'F';

export interface Provider {
  id: string;
  name: string;
  slug: string;
  logo_url: string;
  type: ProviderType;
  infrastructure_type: InfrastructureType;
  description: string;
  website: string;

  // Pricing
  price_per_month: number;
  price_per_epoch: number;
  currency: string;

  // Performance
  uptime_percent: number;
  avg_response_time_ms: number;
  reputation_score: number; // 0-100

  // Capacity
  validators_hosted: number;
  max_validators: number;
  regions: string[];
  supported_chains: string[];

  // Ratings
  energy_rating?: EnergyRating;
  security_rating: number; // 1-5
  avg_user_rating: number; // 1-5
  total_reviews: number;

  // SLA
  sla_uptime_guarantee: number;
  sla_response_time_guarantee: string;
  sla_compensation: string;

  // Status
  is_active: boolean;
  is_verified: boolean;
  featured: boolean;

  // Timestamps
  created_at: string;
  joined_at: string;
}

export interface ProviderDetail extends Provider {
  full_description: string;
  infrastructure_details: string;
  support_channels: string[];
  api_endpoint?: string;
  documentation_url?: string;

  // Historical data
  uptime_history: UptimeDataPoint[];
  validator_count_history: ValidatorCountDataPoint[];

  // Reviews
  reviews: ProviderReview[];

  // Features
  features: string[];
  limitations: string[];
}

export interface UptimeDataPoint {
  date: string;
  uptime_percent: number;
}

export interface ValidatorCountDataPoint {
  date: string;
  count: number;
}

export interface ProviderReview {
  id: string;
  provider_id: string;
  user_id: string;
  username: string;
  rating: number; // 1-5
  title: string;
  content: string;
  pros: string[];
  cons: string[];
  verified_customer: boolean;
  helpful_count: number;
  created_at: string;
}

// Hosted Validator Types
export type ValidatorStatus = 'active' | 'syncing' | 'stopped' | 'error' | 'migrating';
export type MigrationStatus = 'pending' | 'in_progress' | 'completed' | 'failed';

export interface HostedValidator {
  id: string;
  wallet_address: string;
  moniker: string;
  provider_id: string;
  provider_name: string;
  provider_logo: string;

  // Status
  status: ValidatorStatus;
  health_score: number; // 0-100
  uptime_percent: number;

  // Metrics
  block_height: number;
  peers: number;
  syncing: boolean;
  last_signed_block: number;
  missed_blocks_24h: number;

  // Cost
  monthly_cost: number;
  currency: string;
  billing_cycle_start: string;
  billing_cycle_end: string;

  // Migration
  migration_available: boolean;
  current_migration?: Migration;

  // Timestamps
  created_at: string;
  last_health_check: string;
}

export interface Migration {
  id: string;
  validator_id: string;
  from_provider_id: string;
  from_provider_name: string;
  to_provider_id: string;
  to_provider_name: string;
  status: MigrationStatus;
  progress_percent: number;
  started_at: string;
  estimated_completion: string;
  completed_at?: string;
  error_message?: string;
}

// Provider Application Types
export interface ProviderApplication {
  id: string;
  provider_name: string;
  website: string;
  contact_email: string;
  contact_name: string;
  regions: string[];
  infrastructure_type: InfrastructureType;
  pricing_model: string;
  price_per_month: number;
  api_endpoint: string;
  documentation_url?: string;
  capacity: number;
  proof_of_capacity?: string;
  status: 'pending' | 'approved' | 'rejected';
  submitted_at: string;
  reviewed_at?: string;
}

// Comparison Types
export interface ProviderComparison {
  providers: Provider[];
  metrics: ComparisonMetric[];
  recommendations: {
    best_value: string;
    best_uptime: string;
    best_price: string;
    most_popular: string;
  };
}

export interface ComparisonMetric {
  name: string;
  key: string;
  type: 'number' | 'percentage' | 'rating' | 'currency' | 'text' | 'boolean';
  values: Record<string, any>;
  best_value?: string; // provider_id with best value
}

// Filter Types
export interface ProviderFilters {
  search?: string;
  type?: ProviderType;
  infrastructure?: InfrastructureType;
  region?: string;
  min_uptime?: number;
  max_price?: number;
  min_rating?: number;
  min_reputation?: number;
  verified_only?: boolean;
  sort_by?: 'price' | 'uptime' | 'reputation' | 'rating' | 'validators';
  sort_order?: 'asc' | 'desc';
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

// User/Auth Types
export interface User {
  id: string;
  wallet_address: string;
  username?: string;
  email?: string;
  created_at: string;
}

export interface AuthState {
  user: User | null;
  token: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
}
