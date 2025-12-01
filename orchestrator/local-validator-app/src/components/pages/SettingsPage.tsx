import { useState } from 'react';
import { Card } from '../ui/Card';
import { ValidatorConfig } from '../../types/validator';
import { clsx } from 'clsx';

interface SettingsPageProps {
  onBack?: () => void;
}

export function SettingsPage({ onBack }: SettingsPageProps) {
  const [config, setConfig] = useState<ValidatorConfig>({
    rpc_port: 26657,
    p2p_port: 26656,
    grpc_port: 9090,
    auto_update: true,
    pruning_mode: 'default',
    snapshot_interval: 1000,
    heartbeat_interval: 60,
    data_folder: 'C:\\Users\\herna\\.omniphi',
    log_level: 'info',
  });

  const [isSaving, setIsSaving] = useState(false);
  const [saved, setSaved] = useState(false);
  const [activeSection, setActiveSection] = useState<'general' | 'network' | 'advanced' | 'danger'>('general');

  const handleSave = async () => {
    setIsSaving(true);
    // Simulate save operation
    await new Promise(resolve => setTimeout(resolve, 1000));
    setIsSaving(false);
    setSaved(true);
    setTimeout(() => setSaved(false), 3000);
  };

  const handleReset = () => {
    if (confirm('Reset all settings to default values?')) {
      setConfig({
        rpc_port: 26657,
        p2p_port: 26656,
        grpc_port: 9090,
        auto_update: true,
        pruning_mode: 'default',
        snapshot_interval: 1000,
        heartbeat_interval: 60,
        data_folder: 'C:\\Users\\herna\\.omniphi',
        log_level: 'info',
      });
    }
  };

  const sections = [
    { id: 'general', label: 'General', icon: '⚙️' },
    { id: 'network', label: 'Network', icon: '🌐' },
    { id: 'advanced', label: 'Advanced', icon: '🔧' },
    { id: 'danger', label: 'Danger Zone', icon: '⚠️' },
  ] as const;

  return (
    <div className="min-h-screen bg-gray-50 flex flex-col">
      {/* Header */}
      <header className="bg-white shadow-sm border-b border-gray-200">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-4">
              {onBack && (
                <button onClick={onBack} className="text-gray-500 hover:text-gray-700">
                  <span className="text-xl">&larr;</span>
                </button>
              )}
              <div>
                <h1 className="text-xl font-bold text-gray-900">Settings</h1>
                <p className="text-sm text-gray-500">Configure your validator node</p>
              </div>
            </div>
            <div className="flex items-center space-x-3">
              {saved && (
                <span className="text-green-600 text-sm font-medium">Settings saved!</span>
              )}
              <button onClick={handleReset} className="btn btn-secondary">
                Reset to Default
              </button>
              <button
                onClick={handleSave}
                disabled={isSaving}
                className="btn btn-primary"
              >
                {isSaving ? 'Saving...' : 'Save Changes'}
              </button>
            </div>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <div className="flex gap-8">
          {/* Sidebar Navigation */}
          <nav className="w-48 flex-shrink-0">
            <ul className="space-y-1">
              {sections.map(section => (
                <li key={section.id}>
                  <button
                    onClick={() => setActiveSection(section.id)}
                    className={clsx(
                      'w-full flex items-center space-x-2 px-4 py-2 rounded-lg text-sm font-medium transition-colors',
                      activeSection === section.id
                        ? 'bg-purple-100 text-purple-700'
                        : 'text-gray-600 hover:bg-gray-100'
                    )}
                  >
                    <span>{section.icon}</span>
                    <span>{section.label}</span>
                  </button>
                </li>
              ))}
            </ul>
          </nav>

          {/* Settings Content */}
          <div className="flex-1 space-y-6">
            {/* General Settings */}
            {activeSection === 'general' && (
              <>
                <Card title="General Settings">
                  <div className="space-y-6">
                    {/* Data Folder */}
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-2">
                        Data Folder
                      </label>
                      <div className="flex space-x-2">
                        <input
                          type="text"
                          value={config.data_folder}
                          onChange={(e) => setConfig({ ...config, data_folder: e.target.value })}
                          className="flex-1 px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-purple-500"
                        />
                        <button className="btn btn-secondary">Browse</button>
                      </div>
                      <p className="text-xs text-gray-500 mt-1">
                        Location where blockchain data is stored
                      </p>
                    </div>

                    {/* Log Level */}
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-2">
                        Log Level
                      </label>
                      <select
                        value={config.log_level}
                        onChange={(e) => setConfig({ ...config, log_level: e.target.value as any })}
                        className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-purple-500"
                      >
                        <option value="debug">Debug - Most verbose</option>
                        <option value="info">Info - Standard logging</option>
                        <option value="warn">Warn - Warnings and errors only</option>
                        <option value="error">Error - Errors only</option>
                      </select>
                    </div>

                    {/* Auto Update */}
                    <div className="flex items-center justify-between">
                      <div>
                        <label className="block text-sm font-medium text-gray-700">
                          Auto Update
                        </label>
                        <p className="text-xs text-gray-500">
                          Automatically download and apply updates
                        </p>
                      </div>
                      <button
                        onClick={() => setConfig({ ...config, auto_update: !config.auto_update })}
                        className={clsx(
                          'relative inline-flex h-6 w-11 items-center rounded-full transition-colors',
                          config.auto_update ? 'bg-purple-600' : 'bg-gray-200'
                        )}
                      >
                        <span
                          className={clsx(
                            'inline-block h-4 w-4 transform rounded-full bg-white transition-transform',
                            config.auto_update ? 'translate-x-6' : 'translate-x-1'
                          )}
                        />
                      </button>
                    </div>
                  </div>
                </Card>

                <Card title="Heartbeat Settings">
                  <div className="space-y-4">
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-2">
                        Heartbeat Interval (seconds)
                      </label>
                      <input
                        type="number"
                        value={config.heartbeat_interval}
                        onChange={(e) => setConfig({ ...config, heartbeat_interval: parseInt(e.target.value) })}
                        min={30}
                        max={300}
                        className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-purple-500"
                      />
                      <p className="text-xs text-gray-500 mt-1">
                        How often to send heartbeat to orchestrator (30-300 seconds)
                      </p>
                    </div>
                  </div>
                </Card>
              </>
            )}

            {/* Network Settings */}
            {activeSection === 'network' && (
              <Card title="Network Configuration">
                <div className="space-y-6">
                  <div className="grid grid-cols-3 gap-4">
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-2">
                        RPC Port
                      </label>
                      <input
                        type="number"
                        value={config.rpc_port}
                        onChange={(e) => setConfig({ ...config, rpc_port: parseInt(e.target.value) })}
                        className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-purple-500"
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-2">
                        P2P Port
                      </label>
                      <input
                        type="number"
                        value={config.p2p_port}
                        onChange={(e) => setConfig({ ...config, p2p_port: parseInt(e.target.value) })}
                        className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-purple-500"
                      />
                    </div>
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-2">
                        gRPC Port
                      </label>
                      <input
                        type="number"
                        value={config.grpc_port || 9090}
                        onChange={(e) => setConfig({ ...config, grpc_port: parseInt(e.target.value) })}
                        className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-purple-500"
                      />
                    </div>
                  </div>

                  <div className="bg-blue-50 border border-blue-200 rounded-lg p-4">
                    <p className="text-sm text-blue-800">
                      <strong>Note:</strong> Changing ports requires a node restart to take effect.
                      Make sure the new ports are not in use by other applications.
                    </p>
                  </div>
                </div>
              </Card>
            )}

            {/* Advanced Settings */}
            {activeSection === 'advanced' && (
              <>
                <Card title="Pruning Configuration">
                  <div className="space-y-4">
                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-2">
                        Pruning Mode
                      </label>
                      <select
                        value={config.pruning_mode}
                        onChange={(e) => setConfig({ ...config, pruning_mode: e.target.value as any })}
                        className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-purple-500"
                      >
                        <option value="default">Default - Balanced storage/performance</option>
                        <option value="nothing">Nothing - Keep all history (archive node)</option>
                        <option value="everything">Everything - Minimal storage</option>
                        <option value="custom">Custom - Advanced configuration</option>
                      </select>
                    </div>

                    <div>
                      <label className="block text-sm font-medium text-gray-700 mb-2">
                        Snapshot Interval (blocks)
                      </label>
                      <input
                        type="number"
                        value={config.snapshot_interval}
                        onChange={(e) => setConfig({ ...config, snapshot_interval: parseInt(e.target.value) })}
                        min={0}
                        step={100}
                        className="w-full px-4 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-purple-500"
                      />
                      <p className="text-xs text-gray-500 mt-1">
                        How often to create state snapshots (0 to disable)
                      </p>
                    </div>
                  </div>
                </Card>

                <Card title="Performance Tuning">
                  <div className="space-y-4">
                    <div className="grid grid-cols-2 gap-4">
                      <div className="p-4 bg-gray-50 rounded-lg">
                        <h4 className="font-medium text-gray-900 mb-2">Memory Usage</h4>
                        <p className="text-2xl font-bold text-purple-600">512 MB</p>
                        <p className="text-xs text-gray-500">Current allocation</p>
                      </div>
                      <div className="p-4 bg-gray-50 rounded-lg">
                        <h4 className="font-medium text-gray-900 mb-2">Database Size</h4>
                        <p className="text-2xl font-bold text-purple-600">2.1 GB</p>
                        <p className="text-xs text-gray-500">Current disk usage</p>
                      </div>
                    </div>
                  </div>
                </Card>
              </>
            )}

            {/* Danger Zone */}
            {activeSection === 'danger' && (
              <Card title="Danger Zone" className="border-red-200">
                <div className="space-y-6">
                  <div className="flex items-center justify-between p-4 bg-red-50 rounded-lg border border-red-200">
                    <div>
                      <h4 className="font-medium text-red-800">Reset Node Data</h4>
                      <p className="text-sm text-red-600">
                        Delete all blockchain data and start fresh. This cannot be undone.
                      </p>
                    </div>
                    <button
                      onClick={() => confirm('Are you sure? This will delete ALL node data!') && alert('Reset initiated')}
                      className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700"
                    >
                      Reset Node
                    </button>
                  </div>

                  <div className="flex items-center justify-between p-4 bg-red-50 rounded-lg border border-red-200">
                    <div>
                      <h4 className="font-medium text-red-800">Force Stop Validator</h4>
                      <p className="text-sm text-red-600">
                        Immediately stop the validator without graceful shutdown.
                      </p>
                    </div>
                    <button
                      onClick={() => confirm('Force stop validator?') && alert('Force stopped')}
                      className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700"
                    >
                      Force Stop
                    </button>
                  </div>

                  <div className="flex items-center justify-between p-4 bg-red-50 rounded-lg border border-red-200">
                    <div>
                      <h4 className="font-medium text-red-800">Uninstall Application</h4>
                      <p className="text-sm text-red-600">
                        Remove the application and all associated data.
                      </p>
                    </div>
                    <button
                      onClick={() => confirm('Uninstall the application?') && alert('Uninstalling...')}
                      className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700"
                    >
                      Uninstall
                    </button>
                  </div>
                </div>
              </Card>
            )}
          </div>
        </div>
      </main>
    </div>
  );
}
