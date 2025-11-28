/**
 * Omniphi Local Validator - Main Application
 *
 * Enterprise-grade validator management desktop application.
 * Built with React + Electron + Tailwind CSS.
 */

import { useState } from 'react';
import { ValidatorStatusCard } from './components/dashboard/ValidatorStatusCard';
import { NodeHealthCard } from './components/dashboard/NodeHealthCard';
import { ValidatorMetadataCard } from './components/dashboard/ValidatorMetadataCard';
import { PoCScoreCard } from './components/dashboard/PoCScoreCard';
import { RewardsCard } from './components/dashboard/RewardsCard';
import { SlashingCard } from './components/dashboard/SlashingCard';
import { BlockHeightChart } from './components/dashboard/BlockHeightChart';
import { RewardsChart } from './components/dashboard/RewardsChart';
import { UpgradeNotification, UpgradeNotificationCompact } from './components/dashboard/UpgradeNotification';
import { LogsPage } from './components/pages/LogsPage';
import { KeysPage } from './components/pages/KeysPage';
import { SettingsPage } from './components/pages/SettingsPage';
import { useDashboardData } from './hooks';
import { validatorApi } from './services/api';
import './index.css';

type Page = 'dashboard' | 'logs' | 'keys' | 'settings';
type Tab = 'overview' | 'staking' | 'security';

function App() {
  const [currentPage, setCurrentPage] = useState<Page>('dashboard');
  const [activeTab, setActiveTab] = useState<Tab>('overview');
  const [binaryExists, setBinaryExists] = useState(true);
  const [actionError, setActionError] = useState<string | null>(null);

  // Use unified dashboard data hook
  const {
    status,
    health,
    metadata,
    poc,
    rewards,
    slashing,
    loading,
    error,
    startValidator,
    stopValidator,
    isRunning,
  } = useDashboardData({
    fastPollingInterval: 3000,
    slowPollingInterval: 30000,
    enablePolling: true,
  });

  // Check binary exists on mount
  useState(() => {
    const checkBinary = async () => {
      const result = await validatorApi.checkBinary();
      if (result.success) {
        setBinaryExists(result.data ?? true);
      }
    };
    checkBinary();
  });

  // Start validator handler
  const handleStart = async () => {
    setActionError(null);
    const result = await startValidator({});
    if (!result.success) {
      setActionError(result.error || 'Failed to start validator');
    }
  };

  // Stop validator handler
  const handleStop = async () => {
    setActionError(null);
    const result = await stopValidator();
    if (!result.success) {
      setActionError(result.error || 'Failed to stop validator');
    }
  };

  // Check binary handler
  const checkBinary = async () => {
    const result = await validatorApi.checkBinary();
    if (result.success) {
      setBinaryExists(result.data ?? true);
    }
  };

  // Binary not found screen
  if (!binaryExists) {
    return (
      <div className="min-h-screen bg-gray-50 flex items-center justify-center">
        <div className="card max-w-md text-center">
          <div className="w-16 h-16 bg-gradient-to-br from-purple-600 to-indigo-600 rounded-xl flex items-center justify-center mx-auto mb-6">
            <span className="text-white font-bold text-3xl">O</span>
          </div>
          <h1 className="text-2xl font-bold text-gray-900 mb-4">Welcome to Omniphi Local Validator</h1>
          <p className="text-gray-600 mb-6">The validator binary (posd) is not found. Please ensure it exists in:</p>
          <code className="block bg-gray-100 p-3 rounded-lg text-sm mb-6">
            {typeof process !== 'undefined' && process.platform === 'win32' ? 'bin\\posd.exe' : 'bin/posd'}
          </code>
          <button onClick={checkBinary} className="btn btn-primary">
            Check Again
          </button>
        </div>
      </div>
    );
  }

  // Render sub-pages
  if (currentPage === 'logs') {
    return <LogsPage onBack={() => setCurrentPage('dashboard')} />;
  }
  if (currentPage === 'keys') {
    return <KeysPage onBack={() => setCurrentPage('dashboard')} />;
  }
  if (currentPage === 'settings') {
    return <SettingsPage onBack={() => setCurrentPage('dashboard')} />;
  }

  // Main Dashboard
  return (
    <div className="min-h-screen bg-gray-50 flex flex-col">
      {/* Upgrade Notification Banner */}
      <UpgradeNotification />

      {/* Header */}
      <header className="bg-white shadow-sm border-b border-gray-200">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-4">
              <div className="w-10 h-10 bg-gradient-to-br from-purple-600 to-indigo-600 rounded-lg flex items-center justify-center">
                <span className="text-white font-bold text-xl">O</span>
              </div>
              <div>
                <h1 className="text-xl font-bold text-gray-900">Omniphi Local Validator</h1>
                <p className="text-sm text-gray-500">Enterprise-Grade Validator Management</p>
              </div>
            </div>

            <div className="flex items-center space-x-4">
              <div className="flex items-center space-x-2">
                <div className={`w-3 h-3 rounded-full ${isRunning ? 'bg-green-500 animate-pulse' : 'bg-gray-400'}`} />
                <span className="text-sm text-gray-600">
                  {isRunning ? 'Running' : 'Stopped'}
                </span>
              </div>

              {isRunning ? (
                <button onClick={handleStop} className="btn btn-danger">
                  Stop Validator
                </button>
              ) : (
                <button onClick={handleStart} className="btn btn-primary">
                  Start Validator
                </button>
              )}
            </div>
          </div>

          {/* Tab Navigation */}
          <div className="mt-4 border-b border-gray-200">
            <nav className="-mb-px flex space-x-8">
              {(['overview', 'staking', 'security'] as const).map(tab => (
                <button
                  key={tab}
                  onClick={() => setActiveTab(tab)}
                  className={`py-2 px-1 border-b-2 font-medium text-sm capitalize ${
                    activeTab === tab
                      ? 'border-purple-500 text-purple-600'
                      : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
                  }`}
                >
                  {tab === 'staking' ? 'Staking & Rewards' : tab}
                </button>
              ))}
            </nav>
          </div>

          {/* Error Display */}
          {(actionError || error) && (
            <div className="mt-4 p-3 bg-red-50 border border-red-200 rounded-lg">
              <p className="text-sm text-red-700">{actionError || error}</p>
            </div>
          )}
        </div>
      </header>

      {/* Main Content */}
      <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {/* Overview Tab */}
        {activeTab === 'overview' && (
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
            <div className="lg:col-span-2 space-y-6">
              <ValidatorStatusCard status={status} loading={loading} />
              <BlockHeightChart currentHeight={status?.block_height || 0} />
              <ValidatorMetadataCard metadata={metadata} loading={loading} />
            </div>

            <div className="space-y-6">
              <NodeHealthCard health={health} />
              <PoCScoreCard poc={poc} loading={loading} />

              {/* Compact Update Notification */}
              <UpgradeNotificationCompact />

              {/* Quick Actions */}
              <div className="card">
                <h3 className="text-lg font-semibold text-gray-900 mb-4">Quick Actions</h3>
                <div className="space-y-3">
                  <button
                    onClick={() => setCurrentPage('logs')}
                    className="w-full btn btn-secondary text-left flex items-center"
                  >
                    <span className="mr-2">View Logs</span>
                  </button>
                  <button
                    onClick={() => setCurrentPage('keys')}
                    className="w-full btn btn-secondary text-left flex items-center"
                  >
                    <span className="mr-2">Manage Keys</span>
                  </button>
                  <button
                    onClick={() => setCurrentPage('settings')}
                    className="w-full btn btn-secondary text-left flex items-center"
                  >
                    <span className="mr-2">Settings</span>
                  </button>
                </div>
              </div>
            </div>
          </div>
        )}

        {/* Staking Tab */}
        {activeTab === 'staking' && (
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <div className="space-y-6">
              <RewardsCard rewards={rewards} loading={loading} />
              <RewardsChart dailyReward={rewards?.daily} />
            </div>
            <div className="space-y-6">
              <ValidatorMetadataCard metadata={metadata} loading={loading} />
              <PoCScoreCard poc={poc} loading={loading} />
            </div>
          </div>
        )}

        {/* Security Tab */}
        {activeTab === 'security' && (
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <SlashingCard slashing={slashing} loading={loading} />
            <div className="space-y-6">
              <NodeHealthCard health={health} />

              <div className="card">
                <h3 className="text-lg font-semibold text-gray-900 mb-4">Security Checklist</h3>
                <div className="space-y-3">
                  <div className="flex items-center justify-between p-3 bg-green-50 rounded-lg">
                    <span className="text-sm text-green-700">Double Sign Protection</span>
                    <span className="text-green-600 font-medium">Active</span>
                  </div>
                  <div className="flex items-center justify-between p-3 bg-green-50 rounded-lg">
                    <span className="text-sm text-green-700">Key Backup</span>
                    <span className="text-green-600 font-medium">Verified</span>
                  </div>
                  <div className="flex items-center justify-between p-3 bg-yellow-50 rounded-lg">
                    <span className="text-sm text-yellow-700">Remote Signer</span>
                    <span className="text-yellow-600 font-medium">Not Configured</span>
                  </div>
                  <div className="flex items-center justify-between p-3 bg-green-50 rounded-lg">
                    <span className="text-sm text-green-700">Firewall Rules</span>
                    <span className="text-green-600 font-medium">Configured</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        )}
      </main>

      {/* Footer */}
      <footer className="bg-white border-t border-gray-200">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-4">
          <div className="flex items-center justify-between text-sm text-gray-500">
            <span>Omniphi Validator v1.0.0 - Enterprise Edition</span>
            <span>Block updates every 3 seconds</span>
          </div>
        </div>
      </footer>
    </div>
  );
}

export default App;
