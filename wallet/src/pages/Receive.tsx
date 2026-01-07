/**
 * Receive Page
 * Display addresses and QR codes for receiving tokens
 */

import React, { useState } from 'react';
import { useWalletStore } from '@/stores/wallet';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { to1x } from '@/lib/wallet';
import toast from 'react-hot-toast';

type AddressType = 'cosmos' | 'evm';

const Receive: React.FC = () => {
  const { wallet } = useWalletStore();
  const [activeTab, setActiveTab] = useState<AddressType>('cosmos');

  if (!wallet) return null;

  const addresses = {
    cosmos: wallet.cosmos.address,
    evm: wallet.evm.address,
  };

  const displayAddress = activeTab === 'cosmos' ? addresses.cosmos : to1x(addresses.evm);

  const copyAddress = () => {
    navigator.clipboard.writeText(addresses[activeTab]);
    toast.success('Address copied to clipboard');
  };

  return (
    <div className="max-w-lg mx-auto space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-dark-100">Receive</h1>
        <p className="text-dark-400 mt-1">Share your address to receive tokens</p>
      </div>

      {/* Chain selector tabs */}
      <div className="flex bg-dark-800 rounded-lg p-1">
        <button
          className={`flex-1 py-2.5 px-4 rounded-md text-sm font-medium transition-colors ${
            activeTab === 'cosmos'
              ? 'bg-omniphi-600 text-white'
              : 'text-dark-400 hover:text-dark-200'
          }`}
          onClick={() => setActiveTab('cosmos')}
        >
          Core Chain (Cosmos)
        </button>
        <button
          className={`flex-1 py-2.5 px-4 rounded-md text-sm font-medium transition-colors ${
            activeTab === 'evm'
              ? 'bg-omniphi-600 text-white'
              : 'text-dark-400 hover:text-dark-200'
          }`}
          onClick={() => setActiveTab('evm')}
        >
          PoSeQ Chain (EVM)
        </button>
      </div>

      <Card>
        <CardContent className="pt-6 space-y-6">
          {/* QR Code placeholder */}
          <div className="flex justify-center">
            <div className="w-48 h-48 bg-white rounded-xl p-4 flex items-center justify-center">
              {/* Simple QR-like visual - in production, use a QR library */}
              <div className="w-full h-full bg-dark-100 rounded-lg flex items-center justify-center relative overflow-hidden">
                <div className="grid grid-cols-8 gap-0.5 w-full h-full p-2">
                  {Array.from({ length: 64 }).map((_, i) => (
                    <div
                      key={i}
                      className={`aspect-square ${
                        Math.random() > 0.5 ? 'bg-dark-900' : 'bg-white'
                      }`}
                    />
                  ))}
                </div>
                <div className="absolute inset-0 flex items-center justify-center">
                  <div className="w-12 h-12 bg-white rounded-lg flex items-center justify-center">
                    <span className="text-2xl font-bold text-omniphi-600">O</span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <p className="text-xs text-center text-dark-500">
            Scan this QR code to get the address
          </p>

          {/* Address display */}
          <div className="space-y-2">
            <label className="label">
              {activeTab === 'cosmos' ? 'Cosmos Address' : 'EVM Address (1x)'}
            </label>
            <div className="flex items-center gap-2">
              <div className="flex-1 p-3 bg-dark-800 rounded-lg font-mono text-sm text-dark-200 break-all">
                {displayAddress}
              </div>
              <Button variant="secondary" onClick={copyAddress}>
                <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                </svg>
              </Button>
            </div>
          </div>

          {/* Chain info */}
          <div className="p-4 bg-dark-800 rounded-lg">
            {activeTab === 'cosmos' ? (
              <div className="space-y-2 text-sm">
                <p className="text-dark-300">
                  <strong className="text-dark-100">Core Chain</strong> is the main Omniphi blockchain built on Cosmos SDK.
                </p>
                <p className="text-dark-400">
                  Use this address to receive OMNI tokens and interact with Core Chain dApps.
                </p>
              </div>
            ) : (
              <div className="space-y-2 text-sm">
                <p className="text-dark-300">
                  <strong className="text-dark-100">PoSeQ Chain</strong> is the EVM-compatible chain for smart contracts.
                </p>
                <p className="text-dark-400">
                  Use this address to interact with EVM dApps. The <code className="text-omniphi-400">1x</code> prefix is display-only; <code className="text-omniphi-400">0x</code> is used in protocols.
                </p>
              </div>
            )}
          </div>

          {/* Warning */}
          <div className="bg-yellow-500/10 border border-yellow-500/20 rounded-lg p-4">
            <div className="flex gap-3">
              <svg className="w-5 h-5 text-yellow-400 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
              <p className="text-sm text-yellow-300">
                Only send {activeTab === 'cosmos' ? 'Cosmos-compatible' : 'EVM-compatible'} tokens to this address. Sending other tokens may result in permanent loss.
              </p>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
};

export default Receive;
