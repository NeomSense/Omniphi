/**
 * Omniphi Wallet Application
 * Main application component with routing and layout
 */

import React, { Suspense, lazy } from 'react';
import { Routes, Route, Navigate } from 'react-router-dom';
import { Layout } from './components/layout/Layout';
import { WalletGuard } from './components/wallet/WalletGuard';
import { LoadingScreen } from './components/ui/LoadingScreen';

// Lazy load pages for better performance
const Dashboard = lazy(() => import('./pages/Dashboard'));
const Governance = lazy(() => import('./pages/Governance'));
const Staking = lazy(() => import('./pages/Staking'));
const Send = lazy(() => import('./pages/Send'));
const Receive = lazy(() => import('./pages/Receive'));
const Settings = lazy(() => import('./pages/Settings'));
const Welcome = lazy(() => import('./pages/Welcome'));

// Governance sub-routes
const ProposalDetailPage = lazy(() => import('./pages/ProposalDetail'));

const App: React.FC = () => {
  return (
    <Suspense fallback={<LoadingScreen />}>
      <Routes>
        {/* Public routes */}
        <Route path="/welcome" element={<Welcome />} />

        {/* Protected routes - require wallet */}
        <Route element={<WalletGuard />}>
          <Route element={<Layout />}>
            <Route index element={<Dashboard />} />
            <Route path="/governance" element={<Governance />} />
            <Route path="/governance/:proposalId" element={<ProposalDetailPage />} />
            <Route path="/staking" element={<Staking />} />
            <Route path="/send" element={<Send />} />
            <Route path="/receive" element={<Receive />} />
            <Route path="/settings" element={<Settings />} />
          </Route>
        </Route>

        {/* Fallback */}
        <Route path="*" element={<Navigate to="/" replace />} />
      </Routes>
    </Suspense>
  );
};

export default App;
