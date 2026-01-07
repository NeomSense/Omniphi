/**
 * Settings Page
 * Wallet settings and preferences
 */

import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useWalletStore } from '@/stores/wallet';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Modal } from '@/components/ui/Modal';
import { CHAIN_NAME, CHAIN_ID, RPC_ENDPOINTS, REST_ENDPOINTS } from '@/lib/constants';
import toast from 'react-hot-toast';

const Settings: React.FC = () => {
  const navigate = useNavigate();
  const { wallet, encryptedWallet, clearWallet } = useWalletStore();

  const [showClearConfirm, setShowClearConfirm] = useState(false);
  const [clearConfirmText, setClearConfirmText] = useState('');

  const handleClearWallet = () => {
    if (clearConfirmText !== 'DELETE') {
      toast.error('Please type DELETE to confirm');
      return;
    }

    clearWallet();
    toast.success('Wallet removed successfully');
    navigate('/welcome');
  };

  const copyAddress = (address: string, label: string) => {
    navigator.clipboard.writeText(address);
    toast.success(`${label} copied to clipboard`);
  };

  return (
    <div className="max-w-2xl mx-auto space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-dark-100">Settings</h1>
        <p className="text-dark-400 mt-1">Manage your wallet settings</p>
      </div>

      {/* Wallet Info */}
      <Card>
        <CardHeader>
          <CardTitle>Wallet Information</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="p-4 bg-dark-800 rounded-lg">
              <p className="text-sm text-dark-500 mb-1">Cosmos Address</p>
              <p className="font-mono text-sm text-dark-200 break-all">
                {wallet?.cosmos.address || 'Not available'}
              </p>
              {wallet?.cosmos.address && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="mt-2"
                  onClick={() => copyAddress(wallet.cosmos.address, 'Cosmos address')}
                >
                  Copy
                </Button>
              )}
            </div>
            <div className="p-4 bg-dark-800 rounded-lg">
              <p className="text-sm text-dark-500 mb-1">EVM Address</p>
              <p className="font-mono text-sm text-dark-200 break-all">
                {wallet?.evm.address || 'Not available'}
              </p>
              {wallet?.evm.address && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="mt-2"
                  onClick={() => copyAddress(wallet.evm.address, 'EVM address')}
                >
                  Copy
                </Button>
              )}
            </div>
          </div>

          {encryptedWallet && (
            <div className="p-4 bg-dark-800 rounded-lg">
              <p className="text-sm text-dark-500 mb-1">Wallet Created</p>
              <p className="text-dark-200">
                {new Date(encryptedWallet.createdAt).toLocaleDateString('en-US', {
                  year: 'numeric',
                  month: 'long',
                  day: 'numeric',
                  hour: '2-digit',
                  minute: '2-digit',
                })}
              </p>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Network Info */}
      <Card>
        <CardHeader>
          <CardTitle>Network Information</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="p-4 bg-dark-800 rounded-lg">
              <p className="text-sm text-dark-500 mb-1">Network</p>
              <p className="text-dark-200">{CHAIN_NAME}</p>
            </div>
            <div className="p-4 bg-dark-800 rounded-lg">
              <p className="text-sm text-dark-500 mb-1">Chain ID</p>
              <p className="font-mono text-dark-200">{CHAIN_ID}</p>
            </div>
            <div className="p-4 bg-dark-800 rounded-lg">
              <p className="text-sm text-dark-500 mb-1">RPC Endpoint</p>
              <p className="font-mono text-sm text-dark-200 break-all">{RPC_ENDPOINTS.primary}</p>
            </div>
            <div className="p-4 bg-dark-800 rounded-lg">
              <p className="text-sm text-dark-500 mb-1">REST Endpoint</p>
              <p className="font-mono text-sm text-dark-200 break-all">{REST_ENDPOINTS.primary}</p>
            </div>
          </div>

          <div className="flex items-center gap-2 p-4 bg-green-500/10 border border-green-500/20 rounded-lg">
            <div className="w-2 h-2 rounded-full bg-green-500 animate-pulse" />
            <span className="text-sm text-green-400">Connected to {CHAIN_NAME}</span>
          </div>
        </CardContent>
      </Card>

      {/* Security */}
      <Card>
        <CardHeader>
          <CardTitle>Security</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="p-4 bg-dark-800 rounded-lg">
            <div className="flex items-center justify-between">
              <div>
                <p className="font-medium text-dark-200">Recovery Phrase</p>
                <p className="text-sm text-dark-500">Your recovery phrase is encrypted and stored locally</p>
              </div>
              <svg className="w-5 h-5 text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
              </svg>
            </div>
          </div>

          <div className="p-4 bg-dark-800 rounded-lg">
            <div className="flex items-center justify-between">
              <div>
                <p className="font-medium text-dark-200">Password Protection</p>
                <p className="text-sm text-dark-500">Wallet requires password to unlock</p>
              </div>
              <svg className="w-5 h-5 text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
              </svg>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* About */}
      <Card>
        <CardHeader>
          <CardTitle>About</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-2 gap-4 text-sm">
            <div>
              <p className="text-dark-500">Version</p>
              <p className="text-dark-200">1.0.0-beta</p>
            </div>
            <div>
              <p className="text-dark-500">Build</p>
              <p className="text-dark-200">Production</p>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Danger Zone */}
      <Card className="border-red-500/30">
        <CardHeader>
          <CardTitle className="text-red-400">Danger Zone</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="p-4 bg-red-500/10 border border-red-500/20 rounded-lg">
            <div className="flex items-center justify-between">
              <div>
                <p className="font-medium text-red-300">Remove Wallet</p>
                <p className="text-sm text-red-400/80">
                  This will delete your wallet from this device. Make sure you have your recovery phrase backed up.
                </p>
              </div>
              <Button
                variant="danger"
                onClick={() => setShowClearConfirm(true)}
              >
                Remove
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>

      {/* Clear Wallet Confirmation Modal */}
      <Modal
        isOpen={showClearConfirm}
        onClose={() => {
          setShowClearConfirm(false);
          setClearConfirmText('');
        }}
        title="Remove Wallet"
      >
        <div className="space-y-4">
          <div className="bg-red-500/10 border border-red-500/20 rounded-lg p-4">
            <div className="flex gap-3">
              <svg className="w-5 h-5 text-red-400 flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
              <div className="text-sm text-red-300">
                <strong>Warning:</strong> This action cannot be undone. Your wallet will be permanently removed from this device. Without your recovery phrase, you will lose access to your funds forever.
              </div>
            </div>
          </div>

          <div>
            <label className="label">Type DELETE to confirm</label>
            <Input
              type="text"
              placeholder="DELETE"
              value={clearConfirmText}
              onChange={(e) => setClearConfirmText(e.target.value)}
            />
          </div>

          <div className="flex gap-3">
            <Button
              variant="secondary"
              className="flex-1"
              onClick={() => {
                setShowClearConfirm(false);
                setClearConfirmText('');
              }}
            >
              Cancel
            </Button>
            <Button
              variant="danger"
              className="flex-1"
              onClick={handleClearWallet}
              disabled={clearConfirmText !== 'DELETE'}
            >
              Remove Wallet
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
};

export default Settings;
