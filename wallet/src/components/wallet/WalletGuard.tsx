/**
 * Wallet Guard Component
 * Protects routes that require wallet authentication
 */

import React from 'react';
import { Navigate, Outlet } from 'react-router-dom';
import { useWalletStore } from '@/stores/wallet';
import { UnlockWallet } from './UnlockWallet';

export const WalletGuard: React.FC = () => {
  const { encryptedWallet, isUnlocked } = useWalletStore();

  // No wallet exists - redirect to welcome/setup
  if (!encryptedWallet) {
    return <Navigate to="/welcome" replace />;
  }

  // Wallet exists but is locked - show unlock screen
  if (!isUnlocked) {
    return <UnlockWallet />;
  }

  // Wallet is unlocked - render protected routes
  return <Outlet />;
};
