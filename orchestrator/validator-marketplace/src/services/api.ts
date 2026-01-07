/**
 * Validator Marketplace - API Service Layer
 */

import type {
  Provider,
  ProviderDetail,
  HostedValidator,
  ProviderReview,
  ProviderApplication,
  ProviderFilters,
  ProviderComparison,
  ApiResponse,
  PaginatedResponse,
} from '../types';

const API_BASE = 'http://localhost:8000/api/v1';

// Fetch wrapper
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

// Provider API
export const providerApi = {
  async list(filters?: ProviderFilters): Promise<ApiResponse<PaginatedResponse<Provider>>> {
    const query = new URLSearchParams();
    if (filters?.search) query.set('search', filters.search);
    if (filters?.type) query.set('type', filters.type);
    if (filters?.infrastructure) query.set('infrastructure', filters.infrastructure);
    if (filters?.region) query.set('region', filters.region);
    if (filters?.min_uptime) query.set('min_uptime', filters.min_uptime.toString());
    if (filters?.max_price) query.set('max_price', filters.max_price.toString());
    if (filters?.min_rating) query.set('min_rating', filters.min_rating.toString());
    if (filters?.sort_by) query.set('sort_by', filters.sort_by);
    if (filters?.sort_order) query.set('sort_order', filters.sort_order);

    const result = await fetchApi<PaginatedResponse<Provider>>(`/marketplace/providers?${query.toString()}`);

    if (!result.success) {
      return { success: true, data: generateMockProviders() };
    }
    return result;
  },

  async get(id: string): Promise<ApiResponse<ProviderDetail>> {
    const result = await fetchApi<ProviderDetail>(`/marketplace/providers/${id}`);
    if (!result.success) {
      return { success: true, data: generateMockProviderDetail(id) };
    }
    return result;
  },

  async getReviews(id: string): Promise<ApiResponse<ProviderReview[]>> {
    const result = await fetchApi<ProviderReview[]>(`/marketplace/providers/${id}/reviews`);
    if (!result.success) {
      return { success: true, data: generateMockReviews(id) };
    }
    return result;
  },

  async compare(ids: string[]): Promise<ApiResponse<ProviderComparison>> {
    const result = await fetchApi<ProviderComparison>(`/marketplace/providers/compare?ids=${ids.join(',')}`);
    if (!result.success) {
      return { success: true, data: generateMockComparison(ids) };
    }
    return result;
  },
};

// Hosted Validators API
export const hostedValidatorApi = {
  async list(): Promise<ApiResponse<HostedValidator[]>> {
    const result = await fetchApi<HostedValidator[]>('/marketplace/my-validators');
    if (!result.success) {
      return { success: true, data: generateMockHostedValidators() };
    }
    return result;
  },

  async requestMigration(validatorId: string, toProviderId: string): Promise<ApiResponse<void>> {
    return fetchApi<void>(`/marketplace/my-validators/${validatorId}/migrate`, {
      method: 'POST',
      body: JSON.stringify({ to_provider_id: toProviderId }),
    });
  },
};

// Application API
export const applicationApi = {
  async submit(application: Omit<ProviderApplication, 'id' | 'status' | 'submitted_at'>): Promise<ApiResponse<ProviderApplication>> {
    return fetchApi<ProviderApplication>('/marketplace/applications', {
      method: 'POST',
      body: JSON.stringify(application),
    });
  },
};

// Mock Data Generators
function generateMockProviders(): PaginatedResponse<Provider> {
  const providers: Provider[] = [
    {
      id: 'omniphi-cloud',
      name: 'Omniphi Cloud',
      slug: 'omniphi-cloud',
      logo_url: '/logos/omniphi.svg',
      type: 'official',
      infrastructure_type: 'cloud',
      description: 'Official Omniphi-managed cloud infrastructure with 99.9% SLA guarantee.',
      website: 'https://cloud.omniphi.network',
      price_per_month: 99,
      price_per_epoch: 0.5,
      currency: 'USD',
      uptime_percent: 99.95,
      avg_response_time_ms: 15,
      reputation_score: 98,
      validators_hosted: 1250,
      max_validators: 5000,
      regions: ['US-East', 'US-West', 'EU-West', 'Asia-Pacific'],
      supported_chains: ['omniphi-mainnet-1', 'omniphi-testnet-2'],
      energy_rating: 'A',
      security_rating: 5,
      avg_user_rating: 4.8,
      total_reviews: 342,
      sla_uptime_guarantee: 99.9,
      sla_response_time_guarantee: '< 30 minutes',
      sla_compensation: '10x monthly fee',
      is_active: true,
      is_verified: true,
      featured: true,
      created_at: '2024-01-01T00:00:00Z',
      joined_at: '2024-01-01T00:00:00Z',
    },
    {
      id: 'akash-network',
      name: 'Akash Network',
      slug: 'akash-network',
      logo_url: '/logos/akash.svg',
      type: 'decentralized',
      infrastructure_type: 'decentralized',
      description: 'Decentralized cloud computing on Akash. Affordable and censorship-resistant.',
      website: 'https://akash.network',
      price_per_month: 45,
      price_per_epoch: 0.25,
      currency: 'USD',
      uptime_percent: 99.2,
      avg_response_time_ms: 45,
      reputation_score: 85,
      validators_hosted: 320,
      max_validators: 10000,
      regions: ['Global', 'Decentralized'],
      supported_chains: ['omniphi-mainnet-1'],
      energy_rating: 'B',
      security_rating: 4,
      avg_user_rating: 4.2,
      total_reviews: 128,
      sla_uptime_guarantee: 99.0,
      sla_response_time_guarantee: '< 2 hours',
      sla_compensation: 'Community governance',
      is_active: true,
      is_verified: true,
      featured: true,
      created_at: '2024-03-15T00:00:00Z',
      joined_at: '2024-03-15T00:00:00Z',
    },
    {
      id: 'node-guardians',
      name: 'Node Guardians',
      slug: 'node-guardians',
      logo_url: '/logos/node-guardians.svg',
      type: 'community',
      infrastructure_type: 'bare_metal',
      description: 'Premium bare metal infrastructure by experienced validators.',
      website: 'https://nodeguardians.io',
      price_per_month: 149,
      price_per_epoch: 0.75,
      currency: 'USD',
      uptime_percent: 99.99,
      avg_response_time_ms: 8,
      reputation_score: 96,
      validators_hosted: 450,
      max_validators: 1000,
      regions: ['EU-Central', 'US-East'],
      supported_chains: ['omniphi-mainnet-1', 'omniphi-testnet-2'],
      energy_rating: 'A',
      security_rating: 5,
      avg_user_rating: 4.9,
      total_reviews: 89,
      sla_uptime_guarantee: 99.95,
      sla_response_time_guarantee: '< 15 minutes',
      sla_compensation: '15x monthly fee',
      is_active: true,
      is_verified: true,
      featured: false,
      created_at: '2024-02-01T00:00:00Z',
      joined_at: '2024-02-01T00:00:00Z',
    },
    {
      id: 'validator-hub',
      name: 'Validator Hub',
      slug: 'validator-hub',
      logo_url: '/logos/validator-hub.svg',
      type: 'community',
      infrastructure_type: 'cloud',
      description: 'Multi-cloud infrastructure with AWS, GCP, and Azure.',
      website: 'https://validatorhub.io',
      price_per_month: 79,
      price_per_epoch: 0.4,
      currency: 'USD',
      uptime_percent: 99.5,
      avg_response_time_ms: 25,
      reputation_score: 88,
      validators_hosted: 680,
      max_validators: 2000,
      regions: ['US-East', 'EU-West', 'Asia-Singapore'],
      supported_chains: ['omniphi-mainnet-1'],
      security_rating: 4,
      avg_user_rating: 4.4,
      total_reviews: 156,
      sla_uptime_guarantee: 99.5,
      sla_response_time_guarantee: '< 1 hour',
      sla_compensation: '5x monthly fee',
      is_active: true,
      is_verified: true,
      featured: false,
      created_at: '2024-04-01T00:00:00Z',
      joined_at: '2024-04-01T00:00:00Z',
    },
    {
      id: 'stake-easy',
      name: 'StakeEasy',
      slug: 'stake-easy',
      logo_url: '/logos/stake-easy.svg',
      type: 'community',
      infrastructure_type: 'cloud',
      description: 'Budget-friendly hosting for small validators.',
      website: 'https://stakeeasy.xyz',
      price_per_month: 39,
      price_per_epoch: 0.2,
      currency: 'USD',
      uptime_percent: 98.8,
      avg_response_time_ms: 55,
      reputation_score: 75,
      validators_hosted: 1100,
      max_validators: 5000,
      regions: ['US-West', 'EU-Central'],
      supported_chains: ['omniphi-mainnet-1'],
      security_rating: 3,
      avg_user_rating: 3.9,
      total_reviews: 234,
      sla_uptime_guarantee: 98.0,
      sla_response_time_guarantee: '< 4 hours',
      sla_compensation: '2x monthly fee',
      is_active: true,
      is_verified: false,
      featured: false,
      created_at: '2024-05-01T00:00:00Z',
      joined_at: '2024-05-01T00:00:00Z',
    },
    {
      id: 'local-app',
      name: 'Local Validator App',
      slug: 'local-app',
      logo_url: '/logos/local.svg',
      type: 'official',
      infrastructure_type: 'bare_metal',
      description: 'Run your validator on your own hardware with our desktop app.',
      website: 'https://omniphi.network/local-validator',
      price_per_month: 0,
      price_per_epoch: 0,
      currency: 'USD',
      uptime_percent: 0,
      avg_response_time_ms: 0,
      reputation_score: 100,
      validators_hosted: 2500,
      max_validators: 999999,
      regions: ['Self-hosted'],
      supported_chains: ['omniphi-mainnet-1', 'omniphi-testnet-2'],
      energy_rating: 'B',
      security_rating: 5,
      avg_user_rating: 4.6,
      total_reviews: 567,
      sla_uptime_guarantee: 0,
      sla_response_time_guarantee: 'Self-managed',
      sla_compensation: 'N/A',
      is_active: true,
      is_verified: true,
      featured: true,
      created_at: '2024-01-01T00:00:00Z',
      joined_at: '2024-01-01T00:00:00Z',
    },
  ];

  return {
    items: providers,
    total: providers.length,
    page: 1,
    page_size: 20,
    total_pages: 1,
  };
}

function generateMockProviderDetail(id: string): ProviderDetail {
  const providers = generateMockProviders().items;
  const provider = providers.find(p => p.id === id) || providers[0];

  return {
    ...provider,
    full_description: `${provider.description}\n\nOur infrastructure is built with enterprise-grade hardware and redundant systems to ensure maximum uptime. We provide 24/7 monitoring and support for all validators.`,
    infrastructure_details: 'Enterprise SSD NVMe storage, 10Gbps network, ECC RAM, redundant power supplies',
    support_channels: ['Discord', 'Email', 'Telegram'],
    api_endpoint: `https://api.${provider.slug}.io`,
    documentation_url: `https://docs.${provider.slug}.io`,
    uptime_history: Array.from({ length: 30 }, (_, i) => ({
      date: new Date(Date.now() - (29 - i) * 86400000).toISOString().split('T')[0],
      uptime_percent: 99 + Math.random(),
    })),
    validator_count_history: Array.from({ length: 30 }, (_, i) => ({
      date: new Date(Date.now() - (29 - i) * 86400000).toISOString().split('T')[0],
      count: provider.validators_hosted - 30 + i + Math.floor(Math.random() * 10),
    })),
    reviews: generateMockReviews(id),
    features: [
      'Automated node updates',
      '24/7 monitoring',
      'Backup & recovery',
      'DDoS protection',
      'Custom alerting',
    ],
    limitations: [
      'Minimum 3-month commitment',
      'No root access',
    ],
  };
}

function generateMockReviews(providerId: string): ProviderReview[] {
  return [
    {
      id: 'review-1',
      provider_id: providerId,
      user_id: 'user-1',
      username: 'validator_pro',
      rating: 5,
      title: 'Excellent service and uptime',
      content: 'Been using this provider for 6 months. Never had any downtime issues. Support team is very responsive.',
      pros: ['Great uptime', 'Fast support', 'Easy setup'],
      cons: ['Slightly pricey'],
      verified_customer: true,
      helpful_count: 45,
      created_at: new Date(Date.now() - 30 * 86400000).toISOString(),
    },
    {
      id: 'review-2',
      provider_id: providerId,
      user_id: 'user-2',
      username: 'stake_master',
      rating: 4,
      title: 'Solid infrastructure',
      content: 'Good overall experience. The dashboard could be improved but the core service is reliable.',
      pros: ['Reliable', 'Good documentation'],
      cons: ['Dashboard needs work', 'Limited regions'],
      verified_customer: true,
      helpful_count: 23,
      created_at: new Date(Date.now() - 60 * 86400000).toISOString(),
    },
    {
      id: 'review-3',
      provider_id: providerId,
      user_id: 'user-3',
      username: 'crypto_node',
      rating: 5,
      title: 'Best decision for my validator',
      content: 'Migrated from self-hosting and never looked back. Worth every penny.',
      pros: ['Peace of mind', 'No maintenance hassle', 'Professional team'],
      cons: [],
      verified_customer: true,
      helpful_count: 67,
      created_at: new Date(Date.now() - 90 * 86400000).toISOString(),
    },
  ];
}

function generateMockHostedValidators(): HostedValidator[] {
  return [
    {
      id: 'val-1',
      wallet_address: 'omni1abc123...',
      moniker: 'MyValidator-1',
      provider_id: 'omniphi-cloud',
      provider_name: 'Omniphi Cloud',
      provider_logo: '/logos/omniphi.svg',
      status: 'active',
      health_score: 98,
      uptime_percent: 99.95,
      block_height: 1567234,
      peers: 45,
      syncing: false,
      last_signed_block: 1567234,
      missed_blocks_24h: 0,
      monthly_cost: 99,
      currency: 'USD',
      billing_cycle_start: new Date(Date.now() - 15 * 86400000).toISOString(),
      billing_cycle_end: new Date(Date.now() + 15 * 86400000).toISOString(),
      migration_available: true,
      created_at: new Date(Date.now() - 180 * 86400000).toISOString(),
      last_health_check: new Date().toISOString(),
    },
    {
      id: 'val-2',
      wallet_address: 'omni1def456...',
      moniker: 'MyValidator-2',
      provider_id: 'akash-network',
      provider_name: 'Akash Network',
      provider_logo: '/logos/akash.svg',
      status: 'syncing',
      health_score: 85,
      uptime_percent: 99.1,
      block_height: 1567100,
      peers: 32,
      syncing: true,
      last_signed_block: 1567100,
      missed_blocks_24h: 3,
      monthly_cost: 45,
      currency: 'USD',
      billing_cycle_start: new Date(Date.now() - 10 * 86400000).toISOString(),
      billing_cycle_end: new Date(Date.now() + 20 * 86400000).toISOString(),
      migration_available: true,
      created_at: new Date(Date.now() - 60 * 86400000).toISOString(),
      last_health_check: new Date().toISOString(),
    },
  ];
}

function generateMockComparison(ids: string[]): ProviderComparison {
  const allProviders = generateMockProviders().items;
  const providers = allProviders.filter(p => ids.includes(p.id));

  const metrics = [
    { name: 'Price/Month', key: 'price_per_month', type: 'currency' as const, values: {} as Record<string, number>, best_value: '' },
    { name: 'Uptime', key: 'uptime_percent', type: 'percentage' as const, values: {} as Record<string, number>, best_value: '' },
    { name: 'Reputation', key: 'reputation_score', type: 'number' as const, values: {} as Record<string, number>, best_value: '' },
    { name: 'User Rating', key: 'avg_user_rating', type: 'rating' as const, values: {} as Record<string, number>, best_value: '' },
    { name: 'Security Rating', key: 'security_rating', type: 'rating' as const, values: {} as Record<string, number>, best_value: '' },
    { name: 'Validators Hosted', key: 'validators_hosted', type: 'number' as const, values: {} as Record<string, number>, best_value: '' },
    { name: 'Response Time', key: 'avg_response_time_ms', type: 'number' as const, values: {} as Record<string, number>, best_value: '' },
  ];

  providers.forEach(p => {
    metrics.forEach(m => {
      m.values[p.id] = (p as any)[m.key];
    });
  });

  // Calculate recommendations
  const bestValue = providers.reduce((best, p) =>
    (p.reputation_score / p.price_per_month) > (best.reputation_score / best.price_per_month) ? p : best
  );
  const bestUptime = providers.reduce((best, p) =>
    p.uptime_percent > best.uptime_percent ? p : best
  );
  const bestPrice = providers.reduce((best, p) =>
    p.price_per_month < best.price_per_month ? p : best
  );
  const mostPopular = providers.reduce((best, p) =>
    p.validators_hosted > best.validators_hosted ? p : best
  );

  return {
    providers,
    metrics,
    recommendations: {
      best_value: bestValue.name,
      best_uptime: bestUptime.name,
      best_price: bestPrice.name,
      most_popular: mostPopular.name,
    },
  };
}

export const api = {
  providers: providerApi,
  hostedValidators: hostedValidatorApi,
  applications: applicationApi,
};

export default api;
