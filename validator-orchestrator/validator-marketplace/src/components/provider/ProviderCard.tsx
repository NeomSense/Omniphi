/**
 * Provider Card Component
 */

import { Link } from 'react-router-dom';
import { Star, MapPin, Server, Shield, Zap, CheckCircle, ExternalLink } from 'lucide-react';
import type { Provider } from '../../types';

interface ProviderCardProps {
  provider: Provider;
}

export function ProviderCard({ provider }: ProviderCardProps) {
  const typeColors = {
    official: 'badge-omniphi',
    community: 'badge-info',
    decentralized: 'badge-success',
  };

  const energyColors = {
    A: 'text-green-400',
    B: 'text-lime-400',
    C: 'text-yellow-400',
    D: 'text-orange-400',
    F: 'text-red-400',
  };

  return (
    <div className="provider-card group">
      {/* Header */}
      <div className="flex items-start justify-between mb-4">
        <div className="flex items-center space-x-3">
          <div className="w-12 h-12 bg-dark-800 rounded-xl flex items-center justify-center overflow-hidden">
            {provider.logo_url ? (
              <img
                src={provider.logo_url}
                alt={provider.name}
                className="w-8 h-8 object-contain"
                onError={(e) => {
                  (e.target as HTMLImageElement).src = '';
                  (e.target as HTMLImageElement).className = 'hidden';
                }}
              />
            ) : (
              <Server className="w-6 h-6 text-omniphi-400" />
            )}
          </div>
          <div>
            <div className="flex items-center space-x-2">
              <h3 className="font-semibold text-white">{provider.name}</h3>
              {provider.is_verified && (
                <CheckCircle className="w-4 h-4 text-blue-400" />
              )}
            </div>
            <span className={`badge ${typeColors[provider.type]}`}>
              {provider.type}
            </span>
          </div>
        </div>

        {provider.featured && (
          <span className="badge badge-warning">Featured</span>
        )}
      </div>

      {/* Description */}
      <p className="text-sm text-dark-400 mb-4 line-clamp-2">
        {provider.description}
      </p>

      {/* Stats Grid */}
      <div className="grid grid-cols-2 gap-3 mb-4">
        <div className="bg-dark-800 rounded-lg p-3">
          <p className="text-xs text-dark-500">Price/Month</p>
          <p className="text-lg font-bold text-white">
            {provider.price_per_month === 0 ? (
              <span className="text-green-400">Free</span>
            ) : (
              `$${provider.price_per_month}`
            )}
          </p>
        </div>
        <div className="bg-dark-800 rounded-lg p-3">
          <p className="text-xs text-dark-500">Uptime</p>
          <p className="text-lg font-bold text-white">
            {provider.uptime_percent > 0 ? (
              <span className={provider.uptime_percent >= 99.9 ? 'text-green-400' : ''}>
                {provider.uptime_percent.toFixed(2)}%
              </span>
            ) : (
              <span className="text-dark-400">N/A</span>
            )}
          </p>
        </div>
      </div>

      {/* Metrics */}
      <div className="flex items-center justify-between mb-4 text-sm">
        <div className="flex items-center text-dark-400">
          <Server className="w-4 h-4 mr-1" />
          <span>{provider.validators_hosted.toLocaleString()} validators</span>
        </div>
        <div className="flex items-center text-dark-400">
          <MapPin className="w-4 h-4 mr-1" />
          <span>{provider.regions.length} regions</span>
        </div>
      </div>

      {/* Ratings */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center space-x-1">
          {[1, 2, 3, 4, 5].map((star) => (
            <Star
              key={star}
              className={`w-4 h-4 ${
                star <= Math.round(provider.avg_user_rating)
                  ? 'text-yellow-400 fill-yellow-400'
                  : 'text-dark-600'
              }`}
            />
          ))}
          <span className="ml-2 text-sm text-dark-400">
            ({provider.total_reviews})
          </span>
        </div>

        <div className="flex items-center space-x-3">
          <div className="flex items-center text-dark-400" title="Security Rating">
            <Shield className="w-4 h-4 mr-1" />
            <span className="text-sm">{provider.security_rating}/5</span>
          </div>

          {provider.energy_rating && (
            <div className="flex items-center" title="Energy Rating">
              <Zap className={`w-4 h-4 mr-1 ${energyColors[provider.energy_rating]}`} />
              <span className={`text-sm font-medium ${energyColors[provider.energy_rating]}`}>
                {provider.energy_rating}
              </span>
            </div>
          )}
        </div>
      </div>

      {/* Reputation Bar */}
      <div className="mb-4">
        <div className="flex items-center justify-between mb-1">
          <span className="text-xs text-dark-500">Reputation</span>
          <span className="text-xs text-dark-300">{provider.reputation_score}/100</span>
        </div>
        <div className="h-1.5 bg-dark-700 rounded-full overflow-hidden">
          <div
            className="h-full bg-gradient-to-r from-omniphi-600 to-omniphi-400 rounded-full"
            style={{ width: `${provider.reputation_score}%` }}
          />
        </div>
      </div>

      {/* Action Button */}
      <Link
        to={`/provider/${provider.slug}`}
        className="btn btn-secondary w-full group-hover:border-omniphi-500 group-hover:text-omniphi-400 transition-colors"
      >
        View Provider
        <ExternalLink className="w-4 h-4 ml-2" />
      </Link>
    </div>
  );
}

export default ProviderCard;
