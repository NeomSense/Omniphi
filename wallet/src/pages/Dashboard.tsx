/**
 * Dashboard Page
 * Main wallet overview with balances and quick actions
 */

import React from 'react';
import { Link } from 'react-router-dom';
import { useWalletStore, selectTotalStaked, selectTotalRewards } from '@/stores/wallet';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { formatAmount, truncateAddress } from '@/lib/utils';
import { DISPLAY_DENOM, UNBONDING_DAYS } from '@/lib/constants';
import { to1x } from '@/lib/wallet';
import toast from 'react-hot-toast';

const Dashboard: React.FC = () => {
  const { wallet, balance, delegations } = useWalletStore();
  const totalStaked = useWalletStore(selectTotalStaked);
  const totalRewards = useWalletStore(selectTotalRewards);

  if (!wallet) return null;

  const copyAddress = (address: string) => {
    navigator.clipboard.writeText(address);
    toast.success('Address copied');
  };

  return (
    <div className="space-y-6">
      {/* Balance Cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        {/* Available Balance */}
        <Card className="bg-gradient-to-br from-omniphi-600/20 to-dark-900 border-omniphi-500/30">
          <CardContent className="pt-6">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-dark-400">Available Balance</p>
                <p className="text-3xl font-bold text-dark-100 mt-1 amount">
                  {formatAmount(balance)}
                </p>
                <p className="text-sm text-dark-500">{DISPLAY_DENOM}</p>
              </div>
              <div className="w-12 h-12 rounded-xl bg-omniphi-600/30 flex items-center justify-center">
                <svg className="w-6 h-6 text-omniphi-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 10h18M7 15h1m4 0h1m-7 4h12a3 3 0 003-3V8a3 3 0 00-3-3H6a3 3 0 00-3 3v8a3 3 0 003 3z" />
                </svg>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Staked Balance */}
        <Card>
          <CardContent className="pt-6">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-dark-400">Staked Balance</p>
                <p className="text-3xl font-bold text-dark-100 mt-1 amount">
                  {formatAmount(totalStaked)}
                </p>
                <p className="text-sm text-dark-500">{DISPLAY_DENOM}</p>
              </div>
              <div className="w-12 h-12 rounded-xl bg-green-600/20 flex items-center justify-center">
                <svg className="w-6 h-6 text-green-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
                </svg>
              </div>
            </div>
          </CardContent>
        </Card>

        {/* Rewards */}
        <Card>
          <CardContent className="pt-6">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-dark-400">Pending Rewards</p>
                <p className="text-3xl font-bold text-dark-100 mt-1 amount">
                  {formatAmount(totalRewards)}
                </p>
                <p className="text-sm text-dark-500">{DISPLAY_DENOM}</p>
              </div>
              <div className="w-12 h-12 rounded-xl bg-yellow-600/20 flex items-center justify-center">
                <svg className="w-6 h-6 text-yellow-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              </div>
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Quick Actions */}
      <Card>
        <CardHeader>
          <CardTitle>Quick Actions</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <Link to="/send">
              <Button variant="secondary" className="w-full h-20 flex-col gap-2">
                <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 19l9 2-9-18-9 18 9-2zm0 0v-8" />
                </svg>
                Send
              </Button>
            </Link>
            <Link to="/receive">
              <Button variant="secondary" className="w-full h-20 flex-col gap-2">
                <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
                </svg>
                Receive
              </Button>
            </Link>
            <Link to="/staking">
              <Button variant="secondary" className="w-full h-20 flex-col gap-2">
                <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
                </svg>
                Stake
              </Button>
            </Link>
            <Link to="/governance">
              <Button variant="secondary" className="w-full h-20 flex-col gap-2">
                <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
                Vote
              </Button>
            </Link>
          </div>
        </CardContent>
      </Card>

      {/* Addresses */}
      <Card>
        <CardHeader>
          <CardTitle>Your Addresses</CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          {/* Cosmos Address */}
          <div className="flex items-center justify-between p-4 bg-dark-800 rounded-lg">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-full bg-omniphi-600/20 flex items-center justify-center">
                <span className="font-bold text-omniphi-400">O</span>
              </div>
              <div>
                <p className="text-sm text-dark-400">Core Chain (Cosmos)</p>
                <p className="font-mono text-sm text-dark-200">{truncateAddress(wallet.cosmos.address, 14, 8)}</p>
              </div>
            </div>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => copyAddress(wallet.cosmos.address)}
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
              </svg>
              Copy
            </Button>
          </div>

          {/* EVM Address */}
          <div className="flex items-center justify-between p-4 bg-dark-800 rounded-lg">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 rounded-full bg-blue-600/20 flex items-center justify-center">
                <span className="font-bold text-blue-400">1x</span>
              </div>
              <div>
                <p className="text-sm text-dark-400">PoSeQ Chain (EVM)</p>
                <p className="font-mono text-sm text-dark-200">{truncateAddress(to1x(wallet.evm.address), 12, 8)}</p>
              </div>
            </div>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => copyAddress(wallet.evm.address)}
            >
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
              </svg>
              Copy
            </Button>
          </div>
        </CardContent>
      </Card>

      {/* Delegations */}
      {delegations.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>Your Delegations</CardTitle>
          </CardHeader>
          <CardContent>
            <div className="space-y-3">
              {delegations.map((delegation) => (
                <div
                  key={delegation.delegation.validator_address}
                  className="flex items-center justify-between p-4 bg-dark-800 rounded-lg"
                >
                  <div>
                    <p className="font-medium text-dark-200">
                      {truncateAddress(delegation.delegation.validator_address, 16, 8)}
                    </p>
                    <p className="text-sm text-dark-500">
                      {UNBONDING_DAYS} days unbonding period
                    </p>
                  </div>
                  <div className="text-right">
                    <p className="font-mono font-medium text-dark-100">
                      {formatAmount(delegation.balance.amount)} {DISPLAY_DENOM}
                    </p>
                  </div>
                </div>
              ))}
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
};

export default Dashboard;
