/**
 * Unlock Wallet Component
 * Password entry screen for locked wallets
 */

import React, { useState } from 'react';
import { useWalletStore } from '@/stores/wallet';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import toast from 'react-hot-toast';

export const UnlockWallet: React.FC = () => {
  const [password, setPassword] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const { unlockWallet, encryptedWallet, clearWallet } = useWalletStore();

  const handleUnlock = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!password.trim()) {
      toast.error('Please enter your password');
      return;
    }

    setIsLoading(true);

    try {
      await unlockWallet(password);
      toast.success('Wallet unlocked');
    } catch (error) {
      toast.error('Invalid password');
    } finally {
      setIsLoading(false);
    }
  };

  const handleClearWallet = () => {
    if (window.confirm('Are you sure you want to remove this wallet? You will need your recovery phrase to restore it.')) {
      clearWallet();
      toast.success('Wallet removed');
    }
  };

  // Truncate address for display
  const displayAddress = encryptedWallet?.address
    ? `${encryptedWallet.address.slice(0, 12)}...${encryptedWallet.address.slice(-6)}`
    : '';

  return (
    <div className="min-h-screen bg-dark-950 flex items-center justify-center p-4">
      <div className="w-full max-w-md">
        {/* Logo */}
        <div className="text-center mb-8">
          <div className="inline-flex items-center justify-center w-20 h-20 rounded-2xl bg-gradient-to-br from-omniphi-500 to-omniphi-700 mb-4">
            <span className="text-4xl font-bold text-white">O</span>
          </div>
          <h1 className="text-2xl font-bold text-dark-100">Welcome Back</h1>
          <p className="text-dark-400 mt-2">Enter your password to unlock</p>
        </div>

        {/* Wallet info */}
        <div className="card mb-6">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-full bg-omniphi-600/20 flex items-center justify-center">
              <svg className="w-5 h-5 text-omniphi-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 10h18M7 15h1m4 0h1m-7 4h12a3 3 0 003-3V8a3 3 0 00-3-3H6a3 3 0 00-3 3v8a3 3 0 003 3z" />
              </svg>
            </div>
            <div className="flex-1 min-w-0">
              <p className="text-sm text-dark-400">Wallet Address</p>
              <p className="font-mono text-sm text-dark-200 truncate">{displayAddress}</p>
            </div>
          </div>
        </div>

        {/* Unlock form */}
        <form onSubmit={handleUnlock} className="space-y-4">
          <Input
            type="password"
            placeholder="Enter your password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoFocus
          />

          <Button
            type="submit"
            variant="primary"
            size="lg"
            className="w-full"
            isLoading={isLoading}
          >
            Unlock Wallet
          </Button>
        </form>

        {/* Remove wallet option */}
        <div className="mt-8 text-center">
          <button
            type="button"
            onClick={handleClearWallet}
            className="text-sm text-dark-500 hover:text-red-400 transition-colors"
          >
            Remove this wallet
          </button>
        </div>
      </div>
    </div>
  );
};
