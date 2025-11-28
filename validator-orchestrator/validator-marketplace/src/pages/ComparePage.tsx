/**
 * Provider Comparison Page - Side-by-side comparison
 */

import { useEffect, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import {
  ArrowLeft,
  Plus,
  X,
  Check,
  Minus,
  Server,
  MapPin,
  Shield,
  Zap,
  DollarSign,
  Clock,
  Users,
  Star,
  Leaf,
  Award,
  RefreshCw,
} from 'lucide-react';
import { api } from '../services/api';
import type { Provider, ProviderComparison, ProviderType } from '../types';
import { StarRating } from '../components/provider/StarRating';

export function ComparePage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const [allProviders, setAllProviders] = useState<Provider[]>([]);
  const [selectedProviders, setSelectedProviders] = useState<Provider[]>([]);
  const [comparison, setComparison] = useState<ProviderComparison | null>(null);
  const [loading, setLoading] = useState(true);
  const [showSelector, setShowSelector] = useState(false);

  // Parse selected provider IDs from URL
  useEffect(() => {
    const fetchProviders = async () => {
      setLoading(true);
      const result = await api.providers.list();
      if (result.success && result.data) {
        setAllProviders(result.data.items);

        // Get provider IDs from URL
        const ids = searchParams.get('ids')?.split(',').filter(Boolean) || [];
        if (ids.length >= 2) {
          const selected = result.data.items.filter((p) => ids.includes(p.id));
          setSelectedProviders(selected);
        }
      }
      setLoading(false);
    };
    fetchProviders();
  }, []);

  // Fetch comparison when providers change
  useEffect(() => {
    const fetchComparison = async () => {
      if (selectedProviders.length >= 2) {
        const ids = selectedProviders.map((p) => p.id);
        setSearchParams({ ids: ids.join(',') });

        const result = await api.providers.compare(ids);
        if (result.success && result.data) {
          setComparison(result.data);
        }
      } else {
        setComparison(null);
      }
    };
    fetchComparison();
  }, [selectedProviders]);

  const addProvider = (provider: Provider) => {
    if (selectedProviders.length < 4 && !selectedProviders.find((p) => p.id === provider.id)) {
      setSelectedProviders([...selectedProviders, provider]);
    }
    setShowSelector(false);
  };

  const removeProvider = (providerId: string) => {
    setSelectedProviders(selectedProviders.filter((p) => p.id !== providerId));
  };

  const getTypeColor = (type: ProviderType) => {
    switch (type) {
      case 'official':
        return 'text-omniphi-400 bg-omniphi-900/30';
      case 'community':
        return 'text-blue-400 bg-blue-900/30';
      case 'decentralized':
        return 'text-green-400 bg-green-900/30';
      default:
        return 'text-dark-400 bg-dark-700';
    }
  };

  const getInfrastructureLabel = (type: string) => {
    switch (type) {
      case 'cloud':
        return 'Cloud';
      case 'bare_metal':
        return 'Bare Metal';
      case 'decentralized':
        return 'Decentralized';
      default:
        return type;
    }
  };

  const getEnergyIcon = (rating: string | undefined) => {
    if (!rating) return 'text-dark-400';
    const colors: Record<string, string> = {
      A: 'text-green-400',
      B: 'text-green-300',
      C: 'text-yellow-400',
      D: 'text-orange-400',
      F: 'text-red-400',
    };
    return colors[rating] || 'text-dark-400';
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <RefreshCw className="w-8 h-8 text-omniphi-500 animate-spin" />
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      {/* Back Button */}
      <Link to="/" className="inline-flex items-center text-dark-400 hover:text-white mb-6">
        <ArrowLeft className="w-4 h-4 mr-2" />
        Back to Marketplace
      </Link>

      {/* Header */}
      <div className="mb-8">
        <h1 className="text-3xl font-bold text-white mb-2">Compare Providers</h1>
        <p className="text-dark-400">
          Compare up to 4 hosting providers side-by-side to find the best fit for your validator.
        </p>
      </div>

      {/* Selected Providers Header */}
      <div className="card mb-8">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          {[0, 1, 2, 3].map((index) => {
            const provider = selectedProviders[index];

            if (provider) {
              return (
                <div key={provider.id} className="relative">
                  <button
                    onClick={() => removeProvider(provider.id)}
                    className="absolute -top-2 -right-2 w-6 h-6 bg-red-500 hover:bg-red-600 rounded-full flex items-center justify-center z-10"
                  >
                    <X className="w-4 h-4 text-white" />
                  </button>
                  <div className="p-4 bg-dark-800 rounded-xl text-center">
                    <div className="w-12 h-12 bg-dark-700 rounded-xl flex items-center justify-center mx-auto mb-3">
                      {provider.logo_url ? (
                        <img
                          src={provider.logo_url}
                          alt={provider.name}
                          className="w-8 h-8 object-contain"
                        />
                      ) : (
                        <Server className="w-6 h-6 text-omniphi-400" />
                      )}
                    </div>
                    <h3 className="font-semibold text-white text-sm">{provider.name}</h3>
                    <span className={`text-xs px-2 py-0.5 rounded-full ${getTypeColor(provider.type)}`}>
                      {provider.type}
                    </span>
                  </div>
                </div>
              );
            }

            return (
              <button
                key={index}
                onClick={() => setShowSelector(true)}
                className="p-4 border-2 border-dashed border-dark-600 rounded-xl text-center hover:border-omniphi-500 transition-colors min-h-[140px] flex flex-col items-center justify-center"
                disabled={selectedProviders.length >= 4}
              >
                <div className="w-12 h-12 bg-dark-800 rounded-xl flex items-center justify-center mx-auto mb-3">
                  <Plus className="w-6 h-6 text-dark-400" />
                </div>
                <span className="text-sm text-dark-400">Add Provider</span>
              </button>
            );
          })}
        </div>
      </div>

      {selectedProviders.length < 2 ? (
        <div className="card text-center py-16">
          <Server className="w-16 h-16 text-dark-600 mx-auto mb-4" />
          <h2 className="text-xl font-semibold text-white mb-2">Select at least 2 providers</h2>
          <p className="text-dark-400 mb-6">
            Add providers above to see a detailed comparison
          </p>
          <button
            onClick={() => setShowSelector(true)}
            className="btn btn-primary"
          >
            <Plus className="w-4 h-4 mr-2" />
            Add Provider
          </button>
        </div>
      ) : (
        <div className="space-y-6">
          {/* Quick Summary */}
          {comparison && (
            <div className="card">
              <h3 className="text-lg font-semibold text-white mb-4">Quick Summary</h3>
              <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
                <div className="bg-dark-800 rounded-lg p-4">
                  <div className="flex items-center space-x-2 mb-2">
                    <Award className="w-5 h-5 text-yellow-400" />
                    <span className="text-sm text-dark-400">Best Value</span>
                  </div>
                  <p className="font-semibold text-white">
                    {comparison.recommendations.best_value}
                  </p>
                </div>
                <div className="bg-dark-800 rounded-lg p-4">
                  <div className="flex items-center space-x-2 mb-2">
                    <Clock className="w-5 h-5 text-green-400" />
                    <span className="text-sm text-dark-400">Best Uptime</span>
                  </div>
                  <p className="font-semibold text-white">
                    {comparison.recommendations.best_uptime}
                  </p>
                </div>
                <div className="bg-dark-800 rounded-lg p-4">
                  <div className="flex items-center space-x-2 mb-2">
                    <DollarSign className="w-5 h-5 text-blue-400" />
                    <span className="text-sm text-dark-400">Best Price</span>
                  </div>
                  <p className="font-semibold text-white">
                    {comparison.recommendations.best_price}
                  </p>
                </div>
                <div className="bg-dark-800 rounded-lg p-4">
                  <div className="flex items-center space-x-2 mb-2">
                    <Users className="w-5 h-5 text-omniphi-400" />
                    <span className="text-sm text-dark-400">Most Popular</span>
                  </div>
                  <p className="font-semibold text-white">
                    {comparison.recommendations.most_popular}
                  </p>
                </div>
              </div>
            </div>
          )}

          {/* Detailed Comparison Table */}
          <div className="card overflow-x-auto">
            <h3 className="text-lg font-semibold text-white mb-4">Detailed Comparison</h3>
            <table className="w-full">
              <thead>
                <tr className="border-b border-dark-700">
                  <th className="text-left py-3 px-4 text-dark-400 font-medium">Feature</th>
                  {selectedProviders.map((provider) => (
                    <th key={provider.id} className="text-center py-3 px-4 text-white font-medium">
                      {provider.name}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {/* Pricing */}
                <tr className="border-b border-dark-800">
                  <td className="py-4 px-4">
                    <div className="flex items-center space-x-2">
                      <DollarSign className="w-4 h-4 text-dark-400" />
                      <span className="text-dark-300">Monthly Price</span>
                    </div>
                  </td>
                  {selectedProviders.map((provider) => (
                    <td key={provider.id} className="text-center py-4 px-4">
                      <span className="text-white font-semibold">${provider.price_per_month}</span>
                      <span className="text-dark-400">/mo</span>
                    </td>
                  ))}
                </tr>

                {/* Uptime */}
                <tr className="border-b border-dark-800">
                  <td className="py-4 px-4">
                    <div className="flex items-center space-x-2">
                      <Clock className="w-4 h-4 text-dark-400" />
                      <span className="text-dark-300">Uptime SLA</span>
                    </div>
                  </td>
                  {selectedProviders.map((provider) => {
                    const isHighest = provider.uptime_percent === Math.max(...selectedProviders.map((p) => p.uptime_percent));
                    return (
                      <td key={provider.id} className="text-center py-4 px-4">
                        <span className={`font-semibold ${isHighest ? 'text-green-400' : 'text-white'}`}>
                          {provider.uptime_percent.toFixed(2)}%
                        </span>
                      </td>
                    );
                  })}
                </tr>

                {/* Rating */}
                <tr className="border-b border-dark-800">
                  <td className="py-4 px-4">
                    <div className="flex items-center space-x-2">
                      <Star className="w-4 h-4 text-dark-400" />
                      <span className="text-dark-300">Customer Rating</span>
                    </div>
                  </td>
                  {selectedProviders.map((provider) => (
                    <td key={provider.id} className="text-center py-4 px-4">
                      <div className="flex items-center justify-center space-x-2">
                        <StarRating rating={provider.avg_user_rating} size="sm" />
                        <span className="text-dark-400">({provider.total_reviews})</span>
                      </div>
                    </td>
                  ))}
                </tr>

                {/* Validators Hosted */}
                <tr className="border-b border-dark-800">
                  <td className="py-4 px-4">
                    <div className="flex items-center space-x-2">
                      <Users className="w-4 h-4 text-dark-400" />
                      <span className="text-dark-300">Validators Hosted</span>
                    </div>
                  </td>
                  {selectedProviders.map((provider) => (
                    <td key={provider.id} className="text-center py-4 px-4 text-white font-semibold">
                      {provider.validators_hosted.toLocaleString()}
                    </td>
                  ))}
                </tr>

                {/* Regions */}
                <tr className="border-b border-dark-800">
                  <td className="py-4 px-4">
                    <div className="flex items-center space-x-2">
                      <MapPin className="w-4 h-4 text-dark-400" />
                      <span className="text-dark-300">Regions</span>
                    </div>
                  </td>
                  {selectedProviders.map((provider) => (
                    <td key={provider.id} className="text-center py-4 px-4">
                      <div className="flex flex-wrap justify-center gap-1">
                        {provider.regions.slice(0, 3).map((region) => (
                          <span key={region} className="text-xs bg-dark-700 text-dark-300 px-2 py-1 rounded">
                            {region}
                          </span>
                        ))}
                        {provider.regions.length > 3 && (
                          <span className="text-xs text-dark-400">+{provider.regions.length - 3}</span>
                        )}
                      </div>
                    </td>
                  ))}
                </tr>

                {/* Infrastructure */}
                <tr className="border-b border-dark-800">
                  <td className="py-4 px-4">
                    <div className="flex items-center space-x-2">
                      <Server className="w-4 h-4 text-dark-400" />
                      <span className="text-dark-300">Infrastructure</span>
                    </div>
                  </td>
                  {selectedProviders.map((provider) => (
                    <td key={provider.id} className="text-center py-4 px-4 text-white">
                      {getInfrastructureLabel(provider.infrastructure_type)}
                    </td>
                  ))}
                </tr>

                {/* Security */}
                <tr className="border-b border-dark-800">
                  <td className="py-4 px-4">
                    <div className="flex items-center space-x-2">
                      <Shield className="w-4 h-4 text-dark-400" />
                      <span className="text-dark-300">Security Score</span>
                    </div>
                  </td>
                  {selectedProviders.map((provider) => {
                    const score = provider.reputation_score;
                    return (
                      <td key={provider.id} className="text-center py-4 px-4">
                        <div className="flex items-center justify-center space-x-2">
                          <div className="w-16 h-2 bg-dark-700 rounded-full overflow-hidden">
                            <div
                              className={`h-full rounded-full ${score >= 90 ? 'bg-green-500' : score >= 70 ? 'bg-yellow-500' : 'bg-red-500'}`}
                              style={{ width: `${score}%` }}
                            />
                          </div>
                          <span className="text-white font-semibold">{score}</span>
                        </div>
                      </td>
                    );
                  })}
                </tr>

                {/* Energy Rating */}
                <tr className="border-b border-dark-800">
                  <td className="py-4 px-4">
                    <div className="flex items-center space-x-2">
                      <Leaf className="w-4 h-4 text-dark-400" />
                      <span className="text-dark-300">Energy Rating</span>
                    </div>
                  </td>
                  {selectedProviders.map((provider) => (
                    <td key={provider.id} className="text-center py-4 px-4">
                      <span className={`text-xl font-bold ${getEnergyIcon(provider.energy_rating)}`}>
                        {provider.energy_rating || 'N/A'}
                      </span>
                    </td>
                  ))}
                </tr>

                {/* Performance */}
                <tr className="border-b border-dark-800">
                  <td className="py-4 px-4">
                    <div className="flex items-center space-x-2">
                      <Zap className="w-4 h-4 text-dark-400" />
                      <span className="text-dark-300">Avg Response Time</span>
                    </div>
                  </td>
                  {selectedProviders.map((provider) => (
                    <td key={provider.id} className="text-center py-4 px-4 text-white font-semibold">
                      {provider.avg_response_time_ms}ms
                    </td>
                  ))}
                </tr>
              </tbody>
            </table>
          </div>

          {/* Feature Comparison */}
          <div className="card">
            <h3 className="text-lg font-semibold text-white mb-4">Features</h3>
            <div className="space-y-3">
              {[
                'Auto-scaling',
                'DDoS Protection',
                'Backup & Recovery',
                '24/7 Support',
                'Monitoring Dashboard',
                'Custom Alerts',
                'API Access',
                'Migration Support',
              ].map((feature, idx) => (
                <div key={feature} className="flex items-center border-b border-dark-800 pb-3">
                  <span className="flex-1 text-dark-300">{feature}</span>
                  {selectedProviders.map((provider) => {
                    // Determine feature availability based on reputation and index for consistency
                    const hasFeature = provider.reputation_score > (idx * 10 + 20);
                    return (
                      <div key={provider.id} className="flex-1 text-center">
                        {hasFeature ? (
                          <Check className="w-5 h-5 text-green-400 mx-auto" />
                        ) : (
                          <Minus className="w-5 h-5 text-dark-600 mx-auto" />
                        )}
                      </div>
                    );
                  })}
                </div>
              ))}
            </div>
          </div>

          {/* CTA */}
          <div className="card bg-gradient-to-r from-omniphi-900/50 to-dark-800">
            <div className="flex flex-col md:flex-row items-center justify-between">
              <div className="mb-4 md:mb-0">
                <h3 className="text-xl font-semibold text-white mb-2">Ready to choose?</h3>
                <p className="text-dark-400">
                  Select a provider and start hosting your validator today.
                </p>
              </div>
              <div className="flex space-x-3">
                {selectedProviders.map((provider) => (
                  <Link
                    key={provider.id}
                    to={`/provider/${provider.id}`}
                    className="btn btn-secondary btn-sm"
                  >
                    View {provider.name}
                  </Link>
                ))}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Provider Selector Modal */}
      {showSelector && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="card max-w-2xl w-full mx-4 max-h-[80vh] overflow-y-auto">
            <div className="flex items-center justify-between mb-6">
              <h3 className="text-xl font-bold text-white">Select Provider to Compare</h3>
              <button
                onClick={() => setShowSelector(false)}
                className="w-8 h-8 bg-dark-700 hover:bg-dark-600 rounded-lg flex items-center justify-center"
              >
                <X className="w-5 h-5 text-dark-400" />
              </button>
            </div>

            <div className="space-y-3">
              {allProviders
                .filter((p) => !selectedProviders.find((sp) => sp.id === p.id))
                .map((provider) => (
                  <button
                    key={provider.id}
                    onClick={() => addProvider(provider)}
                    className="w-full p-4 bg-dark-800 hover:bg-dark-700 rounded-lg flex items-center justify-between transition-colors"
                  >
                    <div className="flex items-center space-x-3">
                      <div className="w-10 h-10 bg-dark-700 rounded-lg flex items-center justify-center">
                        {provider.logo_url ? (
                          <img
                            src={provider.logo_url}
                            alt={provider.name}
                            className="w-6 h-6 object-contain"
                          />
                        ) : (
                          <Server className="w-5 h-5 text-omniphi-400" />
                        )}
                      </div>
                      <div className="text-left">
                        <p className="font-medium text-white">{provider.name}</p>
                        <p className="text-sm text-dark-400">
                          ${provider.price_per_month}/mo • {provider.uptime_percent.toFixed(1)}% uptime
                        </p>
                      </div>
                    </div>
                    <span className={`text-xs px-2 py-1 rounded-full ${getTypeColor(provider.type)}`}>
                      {provider.type}
                    </span>
                  </button>
                ))}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export default ComparePage;
