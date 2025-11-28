/**
 * Omniphi Local Validator - Router Configuration
 *
 * React Router configuration for page-based navigation.
 * Supports both hash and browser history routing for Electron compatibility.
 */

import { createHashRouter, RouterProvider, Outlet, Navigate, useNavigate } from 'react-router-dom';
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

type Tab = 'overview' | 'staking' | 'security';

/**
 * Dashboard Layout Component
 * Provides the main layout with header, footer, and navigation
 */
function DashboardLayout() {
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<Tab>('overview');
  const [actionError, setActionError] = useState<string | null>(null);

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
  } = useDashboardData();

  const handleStart = async () => {
    setActionError(null);
    const result = await startValidator({});
    if (!result.success) {
      setActionError(result.error || 'Failed to start validator');
    }
  };

  const handleStop = async () => {
    setActionError(null);
    const result = await stopValidator();
    if (!result.success) {
      setActionError(result.error || 'Failed to stop validator');
    }
  };

  return (
    <div className="min-h-screen bg-gray-50 flex flex-col">
      <UpgradeNotification />

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
                <span className="text-sm text-gray-600">{isRunning ? 'Running' : 'Stopped'}</span>
              </div>

              {isRunning ? (
                <button onClick={handleStop} className="btn btn-danger">Stop Validator</button>
              ) : (
                <button onClick={handleStart} className="btn btn-primary">Start Validator</button>
              )}
            </div>
          </div>

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

          {(actionError || error) && (
            <div className="mt-4 p-3 bg-red-50 border border-red-200 rounded-lg">
              <p className="text-sm text-red-700">{actionError || error}</p>
            </div>
          )}
        </div>
      </header>

      <main className="flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 lg:px-8 py-8">
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
              <UpgradeNotificationCompact />
              <div className="card">
                <h3 className="text-lg font-semibold text-gray-900 mb-4">Quick Actions</h3>
                <div className="space-y-3">
                  <button onClick={() => navigate('/logs')} className="w-full btn btn-secondary text-left">
                    View Logs
                  </button>
                  <button onClick={() => navigate('/keys')} className="w-full btn btn-secondary text-left">
                    Manage Keys
                  </button>
                  <button onClick={() => navigate('/settings')} className="w-full btn btn-secondary text-left">
                    Settings
                  </button>
                </div>
              </div>
            </div>
          </div>
        )}

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

/**
 * Logs Page Wrapper with navigation
 */
function LogsPageWrapper() {
  const navigate = useNavigate();
  return <LogsPage onBack={() => navigate('/')} />;
}

/**
 * Keys Page Wrapper with navigation
 */
function KeysPageWrapper() {
  const navigate = useNavigate();
  return <KeysPage onBack={() => navigate('/')} />;
}

/**
 * Settings Page Wrapper with navigation
 */
function SettingsPageWrapper() {
  const navigate = useNavigate();
  return <SettingsPage onBack={() => navigate('/')} />;
}

/**
 * Create Hash Router for Electron compatibility
 * Hash routing works better with Electron's file:// protocol
 */
export const router = createHashRouter([
  {
    path: '/',
    element: <DashboardLayout />,
  },
  {
    path: '/logs',
    element: <LogsPageWrapper />,
  },
  {
    path: '/keys',
    element: <KeysPageWrapper />,
  },
  {
    path: '/settings',
    element: <SettingsPageWrapper />,
  },
  {
    path: '*',
    element: <Navigate to="/" replace />,
  },
]);

/**
 * Router Provider Component
 */
export function AppRouter() {
  return <RouterProvider router={router} />;
}

export default AppRouter;
