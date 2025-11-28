/**
 * Marketplace Home Page
 */

import { useEffect, useState } from 'react';
import { Search, Filter, SlidersHorizontal, RefreshCw } from 'lucide-react';
import { api } from '../services/api';
import { ProviderCard } from '../components/provider/ProviderCard';
import type { Provider, ProviderFilters, ProviderType, InfrastructureType } from '../types';

const typeFilters: { value: ProviderType | ''; label: string }[] = [
  { value: '', label: 'All Types' },
  { value: 'official', label: 'Official' },
  { value: 'community', label: 'Community' },
  { value: 'decentralized', label: 'Decentralized' },
];

const infrastructureFilters: { value: InfrastructureType | ''; label: string }[] = [
  { value: '', label: 'All Infrastructure' },
  { value: 'cloud', label: 'Cloud' },
  { value: 'bare_metal', label: 'Bare Metal' },
  { value: 'decentralized', label: 'Decentralized' },
];

const regionOptions = [
  'All Regions',
  'US-East',
  'US-West',
  'EU-West',
  'EU-Central',
  'Asia-Pacific',
  'Asia-Singapore',
  'Global',
];

export function MarketplacePage() {
  const [providers, setProviders] = useState<Provider[]>([]);
  const [loading, setLoading] = useState(true);
  const [showFilters, setShowFilters] = useState(false);

  const [filters, setFilters] = useState<ProviderFilters>({
    search: '',
    type: undefined,
    infrastructure: undefined,
    region: undefined,
    min_uptime: undefined,
    max_price: undefined,
    min_rating: undefined,
    sort_by: 'reputation',
    sort_order: 'desc',
  });

  const fetchProviders = async () => {
    setLoading(true);
    const result = await api.providers.list(filters);
    if (result.success && result.data) {
      setProviders(result.data.items);
    }
    setLoading(false);
  };

  useEffect(() => {
    fetchProviders();
  }, [filters]);

  const updateFilter = <K extends keyof ProviderFilters>(key: K, value: ProviderFilters[K]) => {
    setFilters((prev) => ({ ...prev, [key]: value }));
  };

  // Filter providers locally based on search
  const filteredProviders = providers.filter((p) => {
    if (filters.search) {
      const search = filters.search.toLowerCase();
      return (
        p.name.toLowerCase().includes(search) ||
        p.description.toLowerCase().includes(search)
      );
    }
    return true;
  });

  // Sort providers
  const sortedProviders = [...filteredProviders].sort((a, b) => {
    const order = filters.sort_order === 'asc' ? 1 : -1;
    switch (filters.sort_by) {
      case 'price':
        return (a.price_per_month - b.price_per_month) * order;
      case 'uptime':
        return (a.uptime_percent - b.uptime_percent) * order;
      case 'rating':
        return (a.avg_user_rating - b.avg_user_rating) * order;
      case 'validators':
        return (a.validators_hosted - b.validators_hosted) * order;
      case 'reputation':
      default:
        return (a.reputation_score - b.reputation_score) * order;
    }
  });

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      {/* Hero Section */}
      <div className="text-center mb-12">
        <h1 className="text-4xl font-bold text-white mb-4">
          Validator Hosting Marketplace
        </h1>
        <p className="text-lg text-dark-400 max-w-2xl mx-auto">
          Compare and choose the best hosting provider for your Omniphi validator.
          From official cloud hosting to decentralized options.
        </p>
      </div>

      {/* Stats Bar */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
        <div className="card text-center">
          <p className="text-3xl font-bold text-omniphi-400">
            {providers.length}
          </p>
          <p className="text-sm text-dark-400">Hosting Providers</p>
        </div>
        <div className="card text-center">
          <p className="text-3xl font-bold text-green-400">
            {providers.reduce((sum, p) => sum + p.validators_hosted, 0).toLocaleString()}
          </p>
          <p className="text-sm text-dark-400">Active Validators</p>
        </div>
        <div className="card text-center">
          <p className="text-3xl font-bold text-blue-400">
            {new Set(providers.flatMap((p) => p.regions)).size}
          </p>
          <p className="text-sm text-dark-400">Global Regions</p>
        </div>
        <div className="card text-center">
          <p className="text-3xl font-bold text-yellow-400">
            {(providers.reduce((sum, p) => sum + p.uptime_percent, 0) / Math.max(providers.length, 1)).toFixed(1)}%
          </p>
          <p className="text-sm text-dark-400">Avg Uptime</p>
        </div>
      </div>

      {/* Search and Filters */}
      <div className="card mb-8">
        <div className="flex flex-col lg:flex-row lg:items-center gap-4">
          {/* Search */}
          <div className="flex-1 relative">
            <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-5 h-5 text-dark-400" />
            <input
              type="text"
              value={filters.search || ''}
              onChange={(e) => updateFilter('search', e.target.value)}
              placeholder="Search providers..."
              className="input pl-10"
            />
          </div>

          {/* Quick Filters */}
          <div className="flex flex-wrap gap-2">
            {typeFilters.map((type) => (
              <button
                key={type.value}
                onClick={() => updateFilter('type', type.value || undefined)}
                className={`filter-chip ${
                  filters.type === type.value || (!filters.type && !type.value)
                    ? 'filter-chip-active'
                    : 'filter-chip-inactive'
                }`}
              >
                {type.label}
              </button>
            ))}
          </div>

          {/* Toggle Advanced Filters */}
          <button
            onClick={() => setShowFilters(!showFilters)}
            className="btn btn-secondary"
          >
            <SlidersHorizontal className="w-4 h-4 mr-2" />
            Filters
          </button>
        </div>

        {/* Advanced Filters */}
        {showFilters && (
          <div className="mt-6 pt-6 border-t border-dark-700">
            <div className="grid grid-cols-1 md:grid-cols-4 gap-4">
              <div>
                <label className="label">Infrastructure</label>
                <select
                  value={filters.infrastructure || ''}
                  onChange={(e) =>
                    updateFilter('infrastructure', e.target.value as InfrastructureType || undefined)
                  }
                  className="select"
                >
                  {infrastructureFilters.map((opt) => (
                    <option key={opt.value} value={opt.value}>
                      {opt.label}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label className="label">Region</label>
                <select
                  value={filters.region || ''}
                  onChange={(e) =>
                    updateFilter('region', e.target.value || undefined)
                  }
                  className="select"
                >
                  {regionOptions.map((region) => (
                    <option key={region} value={region === 'All Regions' ? '' : region}>
                      {region}
                    </option>
                  ))}
                </select>
              </div>

              <div>
                <label className="label">Max Price ($/month)</label>
                <input
                  type="number"
                  value={filters.max_price || ''}
                  onChange={(e) =>
                    updateFilter('max_price', e.target.value ? parseInt(e.target.value) : undefined)
                  }
                  placeholder="No limit"
                  className="input"
                />
              </div>

              <div>
                <label className="label">Min Uptime (%)</label>
                <input
                  type="number"
                  value={filters.min_uptime || ''}
                  onChange={(e) =>
                    updateFilter('min_uptime', e.target.value ? parseFloat(e.target.value) : undefined)
                  }
                  placeholder="Any"
                  className="input"
                  step="0.1"
                  max="100"
                />
              </div>
            </div>

            <div className="flex items-center justify-between mt-4">
              <div className="flex items-center space-x-4">
                <label className="label mb-0">Sort by:</label>
                <select
                  value={filters.sort_by || 'reputation'}
                  onChange={(e) => updateFilter('sort_by', e.target.value as any)}
                  className="select w-auto"
                >
                  <option value="reputation">Reputation</option>
                  <option value="price">Price</option>
                  <option value="uptime">Uptime</option>
                  <option value="rating">User Rating</option>
                  <option value="validators">Validators Hosted</option>
                </select>
                <select
                  value={filters.sort_order || 'desc'}
                  onChange={(e) => updateFilter('sort_order', e.target.value as any)}
                  className="select w-auto"
                >
                  <option value="desc">Highest First</option>
                  <option value="asc">Lowest First</option>
                </select>
              </div>

              <button
                onClick={() =>
                  setFilters({
                    search: '',
                    sort_by: 'reputation',
                    sort_order: 'desc',
                  })
                }
                className="btn btn-ghost text-sm"
              >
                Reset Filters
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Results Header */}
      <div className="flex items-center justify-between mb-6">
        <p className="text-dark-400">
          Showing <span className="text-white font-medium">{sortedProviders.length}</span> providers
        </p>
        <button onClick={fetchProviders} className="btn btn-ghost btn-sm">
          <RefreshCw className={`w-4 h-4 mr-2 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </button>
      </div>

      {/* Provider Grid */}
      {loading ? (
        <div className="flex items-center justify-center py-20">
          <RefreshCw className="w-8 h-8 text-omniphi-500 animate-spin" />
        </div>
      ) : sortedProviders.length === 0 ? (
        <div className="card text-center py-12">
          <Filter className="w-12 h-12 text-dark-600 mx-auto mb-4" />
          <h3 className="text-lg font-medium text-white mb-2">No providers found</h3>
          <p className="text-dark-400">Try adjusting your filters</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          {sortedProviders.map((provider) => (
            <ProviderCard key={provider.id} provider={provider} />
          ))}
        </div>
      )}
    </div>
  );
}

export default MarketplacePage;
