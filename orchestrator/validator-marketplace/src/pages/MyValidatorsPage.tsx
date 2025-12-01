/**
 * My Hosted Validators Page
 */

import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { formatDistanceToNow } from 'date-fns';
import {
  Server,
  Activity,
  RefreshCw,
  ArrowRight,
  CheckCircle,
  AlertTriangle,
  Clock,
  DollarSign,
  Zap,
  Users,
  AlertCircle,
  ChevronRight,
} from 'lucide-react';
import { api } from '../services/api';
import type { HostedValidator, Provider } from '../types';

export function MyValidatorsPage() {
  const [validators, setValidators] = useState<HostedValidator[]>([]);
  const [providers, setProviders] = useState<Provider[]>([]);
  const [loading, setLoading] = useState(true);
  const [migrationModal, setMigrationModal] = useState<string | null>(null);

  useEffect(() => {
    const fetchData = async () => {
      setLoading(true);
      const [valResult, provResult] = await Promise.all([
        api.hostedValidators.list(),
        api.providers.list(),
      ]);
      if (valResult.success && valResult.data) {
        setValidators(valResult.data);
      }
      if (provResult.success && provResult.data) {
        setProviders(provResult.data.items);
      }
      setLoading(false);
    };
    fetchData();
  }, []);

  const handleMigrate = async (validatorId: string, toProviderId: string) => {
    const result = await api.hostedValidators.requestMigration(validatorId, toProviderId);
    if (result.success) {
      setMigrationModal(null);
      // Refresh validators
      const valResult = await api.hostedValidators.list();
      if (valResult.success && valResult.data) {
        setValidators(valResult.data);
      }
    }
  };

  const statusConfig = {
    active: { color: 'badge-success', icon: CheckCircle, label: 'Active' },
    syncing: { color: 'badge-info', icon: RefreshCw, label: 'Syncing' },
    stopped: { color: 'badge-neutral', icon: AlertCircle, label: 'Stopped' },
    error: { color: 'badge-error', icon: AlertTriangle, label: 'Error' },
    migrating: { color: 'badge-warning', icon: ArrowRight, label: 'Migrating' },
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
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-3xl font-bold text-white">My Hosted Validators</h1>
          <p className="text-dark-400 mt-1">Manage and monitor your validators across providers</p>
        </div>
        <Link to="/" className="btn btn-primary">
          <Server className="w-4 h-4 mr-2" />
          Add New Validator
        </Link>
      </div>

      {validators.length === 0 ? (
        <div className="card text-center py-16">
          <Server className="w-16 h-16 text-dark-600 mx-auto mb-4" />
          <h2 className="text-xl font-semibold text-white mb-2">No validators yet</h2>
          <p className="text-dark-400 mb-6">
            Start by choosing a hosting provider from the marketplace
          </p>
          <Link to="/" className="btn btn-primary">
            Browse Providers
          </Link>
        </div>
      ) : (
        <>
          {/* Summary Stats */}
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-8">
            <div className="card">
              <div className="flex items-center space-x-3">
                <div className="w-10 h-10 bg-omniphi-900/30 rounded-lg flex items-center justify-center">
                  <Server className="w-5 h-5 text-omniphi-400" />
                </div>
                <div>
                  <p className="text-2xl font-bold text-white">{validators.length}</p>
                  <p className="text-sm text-dark-400">Total Validators</p>
                </div>
              </div>
            </div>
            <div className="card">
              <div className="flex items-center space-x-3">
                <div className="w-10 h-10 bg-green-900/30 rounded-lg flex items-center justify-center">
                  <Activity className="w-5 h-5 text-green-400" />
                </div>
                <div>
                  <p className="text-2xl font-bold text-white">
                    {validators.filter((v) => v.status === 'active').length}
                  </p>
                  <p className="text-sm text-dark-400">Active</p>
                </div>
              </div>
            </div>
            <div className="card">
              <div className="flex items-center space-x-3">
                <div className="w-10 h-10 bg-blue-900/30 rounded-lg flex items-center justify-center">
                  <Zap className="w-5 h-5 text-blue-400" />
                </div>
                <div>
                  <p className="text-2xl font-bold text-white">
                    {(validators.reduce((sum, v) => sum + v.uptime_percent, 0) / validators.length).toFixed(1)}%
                  </p>
                  <p className="text-sm text-dark-400">Avg Uptime</p>
                </div>
              </div>
            </div>
            <div className="card">
              <div className="flex items-center space-x-3">
                <div className="w-10 h-10 bg-yellow-900/30 rounded-lg flex items-center justify-center">
                  <DollarSign className="w-5 h-5 text-yellow-400" />
                </div>
                <div>
                  <p className="text-2xl font-bold text-white">
                    ${validators.reduce((sum, v) => sum + v.monthly_cost, 0)}
                  </p>
                  <p className="text-sm text-dark-400">Monthly Cost</p>
                </div>
              </div>
            </div>
          </div>

          {/* Validators List */}
          <div className="space-y-4">
            {validators.map((validator) => {
              const status = statusConfig[validator.status];
              const StatusIcon = status.icon;

              return (
                <div key={validator.id} className="card">
                  <div className="flex flex-col lg:flex-row lg:items-center lg:justify-between">
                    {/* Validator Info */}
                    <div className="flex items-start space-x-4 mb-4 lg:mb-0">
                      <div className="w-12 h-12 bg-dark-800 rounded-xl flex items-center justify-center">
                        <Server className="w-6 h-6 text-omniphi-400" />
                      </div>
                      <div>
                        <div className="flex items-center space-x-2">
                          <h3 className="font-semibold text-white">{validator.moniker}</h3>
                          <span className={`badge ${status.color}`}>
                            <StatusIcon className={`w-3 h-3 mr-1 ${validator.status === 'syncing' ? 'animate-spin' : ''}`} />
                            {status.label}
                          </span>
                        </div>
                        <p className="text-sm text-dark-400 font-mono">{validator.wallet_address}</p>
                        <div className="flex items-center space-x-4 mt-2 text-sm text-dark-400">
                          <span className="flex items-center">
                            <Users className="w-4 h-4 mr-1" />
                            {validator.provider_name}
                          </span>
                          <span className="flex items-center">
                            <Clock className="w-4 h-4 mr-1" />
                            {formatDistanceToNow(new Date(validator.last_health_check), { addSuffix: true })}
                          </span>
                        </div>
                      </div>
                    </div>

                    {/* Metrics */}
                    <div className="grid grid-cols-4 gap-4 mb-4 lg:mb-0">
                      <div className="text-center">
                        <p className="text-lg font-bold text-white">{validator.health_score}%</p>
                        <p className="text-xs text-dark-500">Health</p>
                      </div>
                      <div className="text-center">
                        <p className="text-lg font-bold text-white">{validator.uptime_percent.toFixed(1)}%</p>
                        <p className="text-xs text-dark-500">Uptime</p>
                      </div>
                      <div className="text-center">
                        <p className="text-lg font-bold text-white">{validator.block_height.toLocaleString()}</p>
                        <p className="text-xs text-dark-500">Block</p>
                      </div>
                      <div className="text-center">
                        <p className="text-lg font-bold text-white">${validator.monthly_cost}</p>
                        <p className="text-xs text-dark-500">/month</p>
                      </div>
                    </div>

                    {/* Actions */}
                    <div className="flex items-center space-x-2">
                      {validator.migration_available && (
                        <button
                          onClick={() => setMigrationModal(validator.id)}
                          className="btn btn-secondary btn-sm"
                        >
                          <ArrowRight className="w-4 h-4 mr-1" />
                          Migrate
                        </button>
                      )}
                      <Link
                        to={`/validator/${validator.id}`}
                        className="btn btn-ghost btn-sm"
                      >
                        Details
                        <ChevronRight className="w-4 h-4 ml-1" />
                      </Link>
                    </div>
                  </div>

                  {/* Migration in Progress */}
                  {validator.current_migration && (
                    <div className="mt-4 pt-4 border-t border-dark-700">
                      <div className="flex items-center justify-between mb-2">
                        <span className="text-sm text-dark-400">
                          Migrating to {validator.current_migration.to_provider_name}
                        </span>
                        <span className="text-sm text-omniphi-400">
                          {validator.current_migration.progress_percent}%
                        </span>
                      </div>
                      <div className="h-2 bg-dark-700 rounded-full overflow-hidden">
                        <div
                          className="h-full bg-omniphi-500 rounded-full transition-all"
                          style={{ width: `${validator.current_migration.progress_percent}%` }}
                        />
                      </div>
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </>
      )}

      {/* Migration Modal */}
      {migrationModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="card max-w-lg w-full mx-4">
            <h3 className="text-xl font-bold text-white mb-4">Migrate Validator</h3>
            <p className="text-dark-400 mb-6">
              Choose a new hosting provider for your validator. Migration typically takes 15-30 minutes.
            </p>

            <div className="space-y-3 mb-6">
              {providers
                .filter((p) => p.id !== validators.find((v) => v.id === migrationModal)?.provider_id)
                .map((provider) => (
                  <button
                    key={provider.id}
                    onClick={() => handleMigrate(migrationModal, provider.id)}
                    className="w-full p-4 bg-dark-800 hover:bg-dark-700 rounded-lg flex items-center justify-between transition-colors"
                  >
                    <div className="flex items-center space-x-3">
                      <Server className="w-8 h-8 text-omniphi-400" />
                      <div className="text-left">
                        <p className="font-medium text-white">{provider.name}</p>
                        <p className="text-sm text-dark-400">
                          ${provider.price_per_month}/mo • {provider.uptime_percent.toFixed(1)}% uptime
                        </p>
                      </div>
                    </div>
                    <ChevronRight className="w-5 h-5 text-dark-400" />
                  </button>
                ))}
            </div>

            <div className="flex justify-end">
              <button
                onClick={() => setMigrationModal(null)}
                className="btn btn-secondary"
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export default MyValidatorsPage;
