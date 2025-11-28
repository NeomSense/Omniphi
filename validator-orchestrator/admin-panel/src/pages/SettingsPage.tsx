/**
 * Orchestrator Settings Page
 */

import { useEffect, useState } from 'react';
import {
  Save,
  RefreshCw,
  Plus,
  Trash2,
  AlertTriangle,
  Settings,
  Server,
  Shield,
} from 'lucide-react';
import { api } from '../services/api';
import type { OrchestratorSettings, CloudProvider } from '../types';

export function SettingsPage() {
  const [settings, setSettings] = useState<OrchestratorSettings | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [hasChanges, setHasChanges] = useState(false);
  const [activeTab, setActiveTab] = useState<'general' | 'rpc' | 'alerts'>('general');

  const fetchSettings = async () => {
    setLoading(true);
    const result = await api.settings.get();
    if (result.success && result.data) {
      setSettings(result.data);
    }
    setLoading(false);
  };

  useEffect(() => {
    fetchSettings();
  }, []);

  const handleSave = async () => {
    if (!settings) return;
    setSaving(true);
    const result = await api.settings.update(settings);
    if (result.success) {
      setHasChanges(false);
      alert('Settings saved successfully!');
    } else {
      alert('Failed to save settings: ' + result.error);
    }
    setSaving(false);
  };

  const updateSettings = <K extends keyof OrchestratorSettings>(
    key: K,
    value: OrchestratorSettings[K]
  ) => {
    if (!settings) return;
    setSettings({ ...settings, [key]: value });
    setHasChanges(true);
  };

  const updateAlertThreshold = <K extends keyof OrchestratorSettings['alert_thresholds']>(
    key: K,
    value: OrchestratorSettings['alert_thresholds'][K]
  ) => {
    if (!settings) return;
    setSettings({
      ...settings,
      alert_thresholds: {
        ...settings.alert_thresholds,
        [key]: value,
      },
    });
    setHasChanges(true);
  };

  if (loading) {
    return (
      <div className="p-8 flex items-center justify-center h-screen">
        <RefreshCw className="w-8 h-8 text-omniphi-500 animate-spin" />
      </div>
    );
  }

  if (!settings) {
    return (
      <div className="p-8">
        <div className="card text-center py-12">
          <AlertTriangle className="w-12 h-12 text-yellow-500 mx-auto mb-4" />
          <h2 className="text-xl font-semibold text-white mb-2">Failed to load settings</h2>
          <button onClick={fetchSettings} className="btn btn-primary mt-4">
            Retry
          </button>
        </div>
      </div>
    );
  }

  const providers: CloudProvider[] = ['aws', 'gcp', 'azure', 'digitalocean', 'hetzner', 'vultr'];

  return (
    <div className="p-8">
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-white">Orchestrator Settings</h1>
          <p className="text-dark-400 mt-1">Configure provisioning and monitoring parameters</p>
        </div>
        <div className="flex items-center space-x-3">
          {hasChanges && (
            <span className="text-yellow-400 text-sm">Unsaved changes</span>
          )}
          <button
            onClick={handleSave}
            disabled={!hasChanges || saving}
            className="btn btn-primary"
          >
            {saving ? (
              <RefreshCw className="w-4 h-4 mr-2 animate-spin" />
            ) : (
              <Save className="w-4 h-4 mr-2" />
            )}
            Save Changes
          </button>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex space-x-1 mb-6 border-b border-dark-700">
        {[
          { id: 'general', label: 'General', icon: Settings },
          { id: 'rpc', label: 'RPC & Snapshots', icon: Server },
          { id: 'alerts', label: 'Alert Thresholds', icon: Shield },
        ].map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id as any)}
            className={`flex items-center px-4 py-3 text-sm font-medium border-b-2 transition-colors ${
              activeTab === tab.id
                ? 'text-omniphi-400 border-omniphi-500'
                : 'text-dark-400 border-transparent hover:text-white hover:border-dark-500'
            }`}
          >
            <tab.icon className="w-4 h-4 mr-2" />
            {tab.label}
          </button>
        ))}
      </div>

      {/* General Settings */}
      {activeTab === 'general' && (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          <div className="card">
            <h3 className="card-title mb-6">Provisioning Settings</h3>
            <div className="space-y-6">
              <div>
                <label className="label">Default Cloud Provider</label>
                <select
                  value={settings.default_provider}
                  onChange={(e) => updateSettings('default_provider', e.target.value as CloudProvider)}
                  className="select"
                >
                  {providers.map((p) => (
                    <option key={p} value={p}>{p.toUpperCase()}</option>
                  ))}
                </select>
                <p className="text-xs text-dark-500 mt-1">
                  Default provider for new cloud validator requests
                </p>
              </div>

              <div>
                <label className="label">Max Parallel Jobs</label>
                <input
                  type="number"
                  value={settings.max_parallel_jobs}
                  onChange={(e) => updateSettings('max_parallel_jobs', parseInt(e.target.value) || 1)}
                  min={1}
                  max={50}
                  className="input"
                />
                <p className="text-xs text-dark-500 mt-1">
                  Maximum concurrent provisioning operations
                </p>
              </div>

              <div>
                <label className="label">Provisioning Retry Limit</label>
                <input
                  type="number"
                  value={settings.provisioning_retry_limit}
                  onChange={(e) => updateSettings('provisioning_retry_limit', parseInt(e.target.value) || 0)}
                  min={0}
                  max={10}
                  className="input"
                />
                <p className="text-xs text-dark-500 mt-1">
                  Number of retry attempts before marking as failed
                </p>
              </div>
            </div>
          </div>

          <div className="card">
            <h3 className="card-title mb-6">Monitoring Settings</h3>
            <div className="space-y-6">
              <div>
                <label className="label">Heartbeat Interval (seconds)</label>
                <input
                  type="number"
                  value={settings.heartbeat_interval_seconds}
                  onChange={(e) => updateSettings('heartbeat_interval_seconds', parseInt(e.target.value) || 30)}
                  min={10}
                  max={300}
                  className="input"
                />
                <p className="text-xs text-dark-500 mt-1">
                  Interval between health checks for validator nodes
                </p>
              </div>

              <div>
                <label className="label">Log Retention (days)</label>
                <input
                  type="number"
                  value={settings.log_retention_days}
                  onChange={(e) => updateSettings('log_retention_days', parseInt(e.target.value) || 7)}
                  min={1}
                  max={365}
                  className="input"
                />
                <p className="text-xs text-dark-500 mt-1">
                  Days to retain orchestrator and node logs
                </p>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* RPC & Snapshots */}
      {activeTab === 'rpc' && (
        <div className="space-y-6">
          {/* Chain RPC Endpoints */}
          <div className="card">
            <div className="flex items-center justify-between mb-6">
              <h3 className="card-title">Chain RPC Endpoints</h3>
              <button
                onClick={() => {
                  const newEndpoint = {
                    chain_id: 'new-chain-id',
                    endpoints: ['https://rpc.example.com'],
                    priority: settings.chain_rpc_endpoints.length + 1,
                  };
                  updateSettings('chain_rpc_endpoints', [...settings.chain_rpc_endpoints, newEndpoint]);
                }}
                className="btn btn-secondary btn-sm"
              >
                <Plus className="w-4 h-4 mr-1" />
                Add Chain
              </button>
            </div>

            <div className="space-y-4">
              {settings.chain_rpc_endpoints.map((chain, index) => (
                <div key={index} className="p-4 bg-dark-800 rounded-lg">
                  <div className="flex items-center justify-between mb-4">
                    <div className="flex items-center space-x-4">
                      <div>
                        <label className="label text-xs">Chain ID</label>
                        <input
                          type="text"
                          value={chain.chain_id}
                          onChange={(e) => {
                            const updated = [...settings.chain_rpc_endpoints];
                            updated[index].chain_id = e.target.value;
                            updateSettings('chain_rpc_endpoints', updated);
                          }}
                          className="input py-1 w-48"
                        />
                      </div>
                      <div>
                        <label className="label text-xs">Priority</label>
                        <input
                          type="number"
                          value={chain.priority}
                          onChange={(e) => {
                            const updated = [...settings.chain_rpc_endpoints];
                            updated[index].priority = parseInt(e.target.value) || 1;
                            updateSettings('chain_rpc_endpoints', updated);
                          }}
                          className="input py-1 w-20"
                          min={1}
                        />
                      </div>
                    </div>
                    <button
                      onClick={() => {
                        const updated = settings.chain_rpc_endpoints.filter((_, i) => i !== index);
                        updateSettings('chain_rpc_endpoints', updated);
                      }}
                      className="btn btn-ghost btn-icon text-red-400"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>

                  <label className="label text-xs">Endpoints (one per line)</label>
                  <textarea
                    value={chain.endpoints.join('\n')}
                    onChange={(e) => {
                      const updated = [...settings.chain_rpc_endpoints];
                      updated[index].endpoints = e.target.value.split('\n').filter(Boolean);
                      updateSettings('chain_rpc_endpoints', updated);
                    }}
                    className="input h-20 resize-none font-mono text-sm"
                    placeholder="https://rpc1.example.com&#10;https://rpc2.example.com"
                  />
                </div>
              ))}
            </div>
          </div>

          {/* Snapshot URLs */}
          <div className="card">
            <div className="flex items-center justify-between mb-6">
              <h3 className="card-title">Snapshot Sources</h3>
              <button
                onClick={() => {
                  const newSnapshot = {
                    chain_id: 'omniphi-mainnet-1',
                    url: 'https://snapshots.example.com/latest.tar.gz',
                    type: 'pruned' as const,
                    provider: 'community',
                  };
                  updateSettings('snapshot_urls', [...settings.snapshot_urls, newSnapshot]);
                }}
                className="btn btn-secondary btn-sm"
              >
                <Plus className="w-4 h-4 mr-1" />
                Add Snapshot
              </button>
            </div>

            <div className="space-y-4">
              {settings.snapshot_urls.map((snapshot, index) => (
                <div key={index} className="p-4 bg-dark-800 rounded-lg">
                  <div className="grid grid-cols-4 gap-4">
                    <div>
                      <label className="label text-xs">Chain ID</label>
                      <input
                        type="text"
                        value={snapshot.chain_id}
                        onChange={(e) => {
                          const updated = [...settings.snapshot_urls];
                          updated[index].chain_id = e.target.value;
                          updateSettings('snapshot_urls', updated);
                        }}
                        className="input py-1"
                      />
                    </div>
                    <div className="col-span-2">
                      <label className="label text-xs">URL</label>
                      <input
                        type="text"
                        value={snapshot.url}
                        onChange={(e) => {
                          const updated = [...settings.snapshot_urls];
                          updated[index].url = e.target.value;
                          updateSettings('snapshot_urls', updated);
                        }}
                        className="input py-1 font-mono text-sm"
                      />
                    </div>
                    <div className="flex items-end space-x-2">
                      <div className="flex-1">
                        <label className="label text-xs">Type</label>
                        <select
                          value={snapshot.type}
                          onChange={(e) => {
                            const updated = [...settings.snapshot_urls];
                            updated[index].type = e.target.value as 'pruned' | 'archive';
                            updateSettings('snapshot_urls', updated);
                          }}
                          className="select py-1"
                        >
                          <option value="pruned">Pruned</option>
                          <option value="archive">Archive</option>
                        </select>
                      </div>
                      <button
                        onClick={() => {
                          const updated = settings.snapshot_urls.filter((_, i) => i !== index);
                          updateSettings('snapshot_urls', updated);
                        }}
                        className="btn btn-ghost btn-icon text-red-400 mb-0.5"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* Alert Thresholds */}
      {activeTab === 'alerts' && (
        <div className="card max-w-2xl">
          <h3 className="card-title mb-6">Alert Thresholds</h3>
          <p className="text-dark-400 text-sm mb-6">
            Configure thresholds that trigger alerts when exceeded
          </p>

          <div className="space-y-6">
            <div>
              <label className="label">Max Provisioning Time (minutes)</label>
              <input
                type="number"
                value={settings.alert_thresholds.max_provisioning_time_minutes}
                onChange={(e) => updateAlertThreshold('max_provisioning_time_minutes', parseInt(e.target.value) || 30)}
                min={5}
                max={120}
                className="input"
              />
              <p className="text-xs text-dark-500 mt-1">
                Alert if provisioning takes longer than this
              </p>
            </div>

            <div>
              <label className="label">Minimum Success Rate (%)</label>
              <input
                type="number"
                value={settings.alert_thresholds.min_success_rate_percent}
                onChange={(e) => updateAlertThreshold('min_success_rate_percent', parseInt(e.target.value) || 90)}
                min={50}
                max={100}
                className="input"
              />
              <p className="text-xs text-dark-500 mt-1">
                Alert if success rate drops below this threshold
              </p>
            </div>

            <div>
              <label className="label">Max Consecutive Failures</label>
              <input
                type="number"
                value={settings.alert_thresholds.max_consecutive_failures}
                onChange={(e) => updateAlertThreshold('max_consecutive_failures', parseInt(e.target.value) || 3)}
                min={1}
                max={20}
                className="input"
              />
              <p className="text-xs text-dark-500 mt-1">
                Alert after this many consecutive provisioning failures
              </p>
            </div>

            <div>
              <label className="label">Health Check Timeout (seconds)</label>
              <input
                type="number"
                value={settings.alert_thresholds.health_check_timeout_seconds}
                onChange={(e) => updateAlertThreshold('health_check_timeout_seconds', parseInt(e.target.value) || 60)}
                min={10}
                max={300}
                className="input"
              />
              <p className="text-xs text-dark-500 mt-1">
                Timeout for individual health check requests
              </p>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export default SettingsPage;
