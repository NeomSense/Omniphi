/**
 * Provider Detail Page
 */

import { useEffect, useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { format } from 'date-fns';
import {
  ArrowLeft,
  ExternalLink,
  MapPin,
  Server,
  Shield,
  Zap,
  CheckCircle,
  Clock,
  Globe,
  MessageCircle,
  ThumbsUp,
  RefreshCw,
  GitCompare,
} from 'lucide-react';
import { AreaChart, Area, XAxis, YAxis, Tooltip, ResponsiveContainer } from 'recharts';
import { api } from '../services/api';
import { StarRating } from '../components/provider/StarRating';
import type { ProviderDetail, ProviderReview } from '../types';

export function ProviderDetailPage() {
  const { slug } = useParams<{ slug: string }>();
  const [provider, setProvider] = useState<ProviderDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<'overview' | 'reviews' | 'sla'>('overview');

  useEffect(() => {
    const fetchProvider = async () => {
      if (!slug) return;
      setLoading(true);
      const result = await api.providers.get(slug);
      if (result.success && result.data) {
        setProvider(result.data);
      }
      setLoading(false);
    };
    fetchProvider();
  }, [slug]);

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <RefreshCw className="w-8 h-8 text-omniphi-500 animate-spin" />
      </div>
    );
  }

  if (!provider) {
    return (
      <div className="max-w-7xl mx-auto px-4 py-16 text-center">
        <h2 className="text-2xl font-bold text-white mb-4">Provider not found</h2>
        <Link to="/" className="btn btn-primary">
          Back to Marketplace
        </Link>
      </div>
    );
  }

  const typeColors = {
    official: 'badge-omniphi',
    community: 'badge-info',
    decentralized: 'badge-success',
  };

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      {/* Back Button */}
      <Link
        to="/"
        className="inline-flex items-center text-dark-400 hover:text-white mb-6"
      >
        <ArrowLeft className="w-4 h-4 mr-2" />
        Back to Marketplace
      </Link>

      {/* Header */}
      <div className="card mb-8">
        <div className="flex flex-col lg:flex-row lg:items-start lg:justify-between">
          <div className="flex items-start space-x-4">
            <div className="w-20 h-20 bg-dark-800 rounded-xl flex items-center justify-center">
              <Server className="w-10 h-10 text-omniphi-400" />
            </div>
            <div>
              <div className="flex items-center space-x-3">
                <h1 className="text-3xl font-bold text-white">{provider.name}</h1>
                {provider.is_verified && (
                  <CheckCircle className="w-6 h-6 text-blue-400" />
                )}
              </div>
              <div className="flex items-center space-x-3 mt-2">
                <span className={`badge ${typeColors[provider.type]}`}>
                  {provider.type}
                </span>
                <span className="badge badge-neutral">{provider.infrastructure_type}</span>
                {provider.featured && <span className="badge badge-warning">Featured</span>}
              </div>
              <p className="text-dark-400 mt-3 max-w-2xl">{provider.description}</p>
            </div>
          </div>

          <div className="mt-6 lg:mt-0 flex flex-col items-end space-y-3">
            <div className="text-right">
              <p className="text-sm text-dark-500">Starting at</p>
              <p className="text-3xl font-bold text-white">
                {provider.price_per_month === 0 ? (
                  <span className="text-green-400">Free</span>
                ) : (
                  <>${provider.price_per_month}<span className="text-lg text-dark-400">/mo</span></>
                )}
              </p>
            </div>
            <Link to={`/setup?provider=${provider.id}`} className="btn btn-primary btn-lg">
              Select this Provider
            </Link>
            <Link to={`/compare?providers=${provider.id}`} className="btn btn-secondary">
              <GitCompare className="w-4 h-4 mr-2" />
              Compare with Others
            </Link>
          </div>
        </div>
      </div>

      {/* Quick Stats */}
      <div className="grid grid-cols-2 md:grid-cols-5 gap-4 mb-8">
        <div className="card text-center">
          <p className="text-2xl font-bold text-green-400">{provider.uptime_percent.toFixed(2)}%</p>
          <p className="text-sm text-dark-400">Uptime</p>
        </div>
        <div className="card text-center">
          <div className="flex justify-center mb-1">
            <StarRating rating={provider.avg_user_rating} />
          </div>
          <p className="text-sm text-dark-400">{provider.total_reviews} reviews</p>
        </div>
        <div className="card text-center">
          <p className="text-2xl font-bold text-blue-400">{provider.validators_hosted.toLocaleString()}</p>
          <p className="text-sm text-dark-400">Validators</p>
        </div>
        <div className="card text-center">
          <p className="text-2xl font-bold text-omniphi-400">{provider.reputation_score}</p>
          <p className="text-sm text-dark-400">Reputation</p>
        </div>
        <div className="card text-center">
          <p className="text-2xl font-bold text-yellow-400">{provider.avg_response_time_ms}ms</p>
          <p className="text-sm text-dark-400">Avg Latency</p>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex space-x-1 border-b border-dark-700 mb-6">
        {[
          { id: 'overview', label: 'Overview' },
          { id: 'reviews', label: `Reviews (${provider.total_reviews})` },
          { id: 'sla', label: 'SLA & Terms' },
        ].map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id as any)}
            className={`px-4 py-3 text-sm font-medium border-b-2 transition-colors ${
              activeTab === tab.id
                ? 'text-omniphi-400 border-omniphi-500'
                : 'text-dark-400 border-transparent hover:text-white'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab Content */}
      {activeTab === 'overview' && (
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
          <div className="lg:col-span-2 space-y-8">
            {/* Uptime Chart */}
            <div className="card">
              <h3 className="text-lg font-semibold text-white mb-4">Uptime History (30 Days)</h3>
              <div className="h-64">
                <ResponsiveContainer width="100%" height="100%">
                  <AreaChart data={provider.uptime_history}>
                    <defs>
                      <linearGradient id="uptimeGradient" x1="0" y1="0" x2="0" y2="1">
                        <stop offset="5%" stopColor="#7c3aed" stopOpacity={0.3} />
                        <stop offset="95%" stopColor="#7c3aed" stopOpacity={0} />
                      </linearGradient>
                    </defs>
                    <XAxis dataKey="date" stroke="#6b7280" fontSize={12} tickFormatter={(d) => format(new Date(d), 'MMM d')} />
                    <YAxis stroke="#6b7280" fontSize={12} domain={[99, 100]} tickFormatter={(v) => `${v}%`} />
                    <Tooltip
                      contentStyle={{ backgroundColor: '#1f2937', border: 'none', borderRadius: '8px' }}
                      labelFormatter={(d) => format(new Date(d), 'MMM d, yyyy')}
                      formatter={(value: number) => [`${value.toFixed(3)}%`, 'Uptime']}
                    />
                    <Area type="monotone" dataKey="uptime_percent" stroke="#7c3aed" fill="url(#uptimeGradient)" />
                  </AreaChart>
                </ResponsiveContainer>
              </div>
            </div>

            {/* Features */}
            <div className="card">
              <h3 className="text-lg font-semibold text-white mb-4">Features</h3>
              <ul className="grid grid-cols-2 gap-3">
                {provider.features.map((feature, i) => (
                  <li key={i} className="flex items-center text-dark-300">
                    <CheckCircle className="w-4 h-4 text-green-400 mr-2 flex-shrink-0" />
                    {feature}
                  </li>
                ))}
              </ul>
            </div>

            {/* Infrastructure */}
            <div className="card">
              <h3 className="text-lg font-semibold text-white mb-4">Infrastructure Details</h3>
              <p className="text-dark-300">{provider.infrastructure_details}</p>
            </div>
          </div>

          {/* Sidebar */}
          <div className="space-y-6">
            {/* Quick Info */}
            <div className="card">
              <h3 className="text-lg font-semibold text-white mb-4">Quick Info</h3>
              <div className="space-y-4">
                <div className="flex items-center justify-between">
                  <span className="text-dark-400 flex items-center">
                    <MapPin className="w-4 h-4 mr-2" />
                    Regions
                  </span>
                  <span className="text-white">{provider.regions.join(', ')}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-dark-400 flex items-center">
                    <Globe className="w-4 h-4 mr-2" />
                    Chains
                  </span>
                  <span className="text-white">{provider.supported_chains.length}</span>
                </div>
                <div className="flex items-center justify-between">
                  <span className="text-dark-400 flex items-center">
                    <Shield className="w-4 h-4 mr-2" />
                    Security
                  </span>
                  <StarRating rating={provider.security_rating} size="sm" />
                </div>
                {provider.energy_rating && (
                  <div className="flex items-center justify-between">
                    <span className="text-dark-400 flex items-center">
                      <Zap className="w-4 h-4 mr-2" />
                      Energy
                    </span>
                    <span className="text-green-400 font-medium">{provider.energy_rating}</span>
                  </div>
                )}
                <div className="flex items-center justify-between">
                  <span className="text-dark-400 flex items-center">
                    <Clock className="w-4 h-4 mr-2" />
                    Since
                  </span>
                  <span className="text-white">{format(new Date(provider.joined_at), 'MMM yyyy')}</span>
                </div>
              </div>
            </div>

            {/* Support */}
            <div className="card">
              <h3 className="text-lg font-semibold text-white mb-4">Support</h3>
              <div className="flex flex-wrap gap-2">
                {provider.support_channels.map((channel) => (
                  <span key={channel} className="badge badge-neutral">
                    <MessageCircle className="w-3 h-3 mr-1" />
                    {channel}
                  </span>
                ))}
              </div>
              <a href={provider.website} target="_blank" rel="noopener noreferrer" className="btn btn-secondary w-full mt-4">
                <ExternalLink className="w-4 h-4 mr-2" />
                Visit Website
              </a>
            </div>
          </div>
        </div>
      )}

      {activeTab === 'reviews' && (
        <div className="space-y-6">
          {provider.reviews.map((review) => (
            <ReviewCard key={review.id} review={review} />
          ))}
        </div>
      )}

      {activeTab === 'sla' && (
        <div className="card max-w-2xl">
          <h3 className="text-lg font-semibold text-white mb-6">Service Level Agreement</h3>
          <div className="space-y-6">
            <div>
              <p className="text-sm text-dark-500">Uptime Guarantee</p>
              <p className="text-2xl font-bold text-green-400">{provider.sla_uptime_guarantee}%</p>
            </div>
            <div>
              <p className="text-sm text-dark-500">Response Time Guarantee</p>
              <p className="text-xl font-medium text-white">{provider.sla_response_time_guarantee}</p>
            </div>
            <div>
              <p className="text-sm text-dark-500">Compensation</p>
              <p className="text-xl font-medium text-white">{provider.sla_compensation}</p>
            </div>
            {provider.limitations.length > 0 && (
              <div>
                <p className="text-sm text-dark-500 mb-2">Limitations</p>
                <ul className="list-disc list-inside text-dark-300 space-y-1">
                  {provider.limitations.map((limitation, i) => (
                    <li key={i}>{limitation}</li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

function ReviewCard({ review }: { review: ProviderReview }) {
  return (
    <div className="card">
      <div className="flex items-start justify-between mb-4">
        <div className="flex items-center space-x-3">
          <div className="w-10 h-10 bg-dark-800 rounded-full flex items-center justify-center">
            <span className="text-lg font-medium text-omniphi-400">
              {review.username[0].toUpperCase()}
            </span>
          </div>
          <div>
            <div className="flex items-center space-x-2">
              <span className="font-medium text-white">{review.username}</span>
              {review.verified_customer && (
                <span className="badge badge-success text-xs">Verified</span>
              )}
            </div>
            <p className="text-sm text-dark-500">
              {format(new Date(review.created_at), 'MMM d, yyyy')}
            </p>
          </div>
        </div>
        <StarRating rating={review.rating} />
      </div>

      <h4 className="font-medium text-white mb-2">{review.title}</h4>
      <p className="text-dark-300 mb-4">{review.content}</p>

      {(review.pros.length > 0 || review.cons.length > 0) && (
        <div className="grid grid-cols-2 gap-4 mb-4">
          {review.pros.length > 0 && (
            <div>
              <p className="text-xs text-dark-500 mb-2">Pros</p>
              <ul className="space-y-1">
                {review.pros.map((pro, i) => (
                  <li key={i} className="text-sm text-green-400 flex items-center">
                    <CheckCircle className="w-3 h-3 mr-1" />
                    {pro}
                  </li>
                ))}
              </ul>
            </div>
          )}
          {review.cons.length > 0 && (
            <div>
              <p className="text-xs text-dark-500 mb-2">Cons</p>
              <ul className="space-y-1">
                {review.cons.map((con, i) => (
                  <li key={i} className="text-sm text-red-400">- {con}</li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}

      <div className="flex items-center text-dark-500 text-sm">
        <button className="flex items-center hover:text-white transition-colors">
          <ThumbsUp className="w-4 h-4 mr-1" />
          Helpful ({review.helpful_count})
        </button>
      </div>
    </div>
  );
}

export default ProviderDetailPage;
