/**
 * Header Component
 * Top bar with wallet info and actions
 */

import React, { useState } from 'react';
import { useWalletStore, selectTotalBalance } from '@/stores/wallet';
import { formatAmount, truncateAddress } from '@/lib/utils';
import { Button } from '@/components/ui/Button';
import { DISPLAY_DENOM } from '@/lib/constants';
import toast from 'react-hot-toast';

export const Header: React.FC = () => {
  const { wallet, lockWallet, refreshAll, isLoading } = useWalletStore();
  const totalBalance = useWalletStore(selectTotalBalance);
  const [isRefreshing, setIsRefreshing] = useState(false);

  const handleCopyAddress = () => {
    if (wallet?.cosmos.address) {
      navigator.clipboard.writeText(wallet.cosmos.address);
      toast.success('Address copied to clipboard');
    }
  };

  const handleRefresh = async () => {
    setIsRefreshing(true);
    await refreshAll();
    setIsRefreshing(false);
    toast.success('Data refreshed');
  };

  const handleLock = () => {
    lockWallet();
    toast.success('Wallet locked');
  };

  if (!wallet) return null;

  return (
    <header className="h-16 bg-dark-900 border-b border-dark-700 flex items-center justify-between px-6">
      {/* Balance display */}
      <div className="flex items-center gap-4">
        <div>
          <p className="text-xs text-dark-500 uppercase tracking-wide">Total Balance</p>
          <p className="text-xl font-bold text-dark-100 amount">
            {formatAmount(totalBalance)} <span className="text-dark-400 text-sm">{DISPLAY_DENOM}</span>
          </p>
        </div>
      </div>

      {/* Right side actions */}
      <div className="flex items-center gap-4">
        {/* Refresh button */}
        <button
          onClick={handleRefresh}
          disabled={isRefreshing}
          className="p-2 rounded-lg text-dark-400 hover:text-dark-200 hover:bg-dark-800 transition-colors disabled:opacity-50"
          title="Refresh data"
        >
          <svg
            className={`w-5 h-5 ${isRefreshing ? 'animate-spin' : ''}`}
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
        </button>

        {/* Address display */}
        <button
          onClick={handleCopyAddress}
          className="flex items-center gap-2 px-3 py-2 rounded-lg bg-dark-800 hover:bg-dark-700 transition-colors"
          title="Click to copy address"
        >
          <div className="w-6 h-6 rounded-full bg-gradient-to-br from-omniphi-500 to-omniphi-700 flex items-center justify-center">
            <span className="text-xs font-bold text-white">O</span>
          </div>
          <span className="font-mono text-sm text-dark-300">
            {truncateAddress(wallet.cosmos.address)}
          </span>
          <svg className="w-4 h-4 text-dark-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
          </svg>
        </button>

        {/* Lock button */}
        <Button variant="ghost" size="sm" onClick={handleLock}>
          <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
          </svg>
          Lock
        </Button>
      </div>
    </header>
  );
};
