/**
 * Staking Page
 * Validator selection, delegation, and staking rewards management
 */

import React, { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useWalletStore, selectTotalStaked, selectTotalRewards } from '@/stores/wallet';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { Modal } from '@/components/ui/Modal';
import { Badge } from '@/components/ui/Badge';
import { getValidators, Validator } from '@/lib/api';
import { formatAmount, parseAmount, truncateAddress } from '@/lib/utils';
import { DISPLAY_DENOM, DENOM, GAS_PRICE, UNBONDING_DAYS } from '@/lib/constants';
import toast from 'react-hot-toast';

const Staking: React.FC = () => {
  const queryClient = useQueryClient();
  const { wallet, balance, delegations, refreshAll, getSigningClient } = useWalletStore();
  const totalStaked = useWalletStore(selectTotalStaked);
  const totalRewards = useWalletStore(selectTotalRewards);

  const [selectedValidator, setSelectedValidator] = useState<Validator | null>(null);
  const [stakeAmount, setStakeAmount] = useState('');
  const [isStakeModalOpen, setIsStakeModalOpen] = useState(false);
  const [isUnstakeModalOpen, setIsUnstakeModalOpen] = useState(false);

  // Fetch validators
  const { data: validators = [], isLoading } = useQuery({
    queryKey: ['validators'],
    queryFn: () => getValidators(),
  });

  // Delegate mutation
  const delegateMutation = useMutation({
    mutationFn: async ({ validatorAddress, amount }: { validatorAddress: string; amount: string }) => {
      const client = await getSigningClient();
      const result = await client.delegateTokens(
        wallet!.cosmos.address,
        validatorAddress,
        { denom: DENOM, amount },
        { amount: [{ denom: DENOM, amount: '5000' }], gas: '200000' },
        ''
      );
      return result;
    },
    onSuccess: () => {
      toast.success('Delegation successful!');
      refreshAll();
      queryClient.invalidateQueries({ queryKey: ['validators'] });
      setIsStakeModalOpen(false);
      setStakeAmount('');
    },
    onError: (error: Error) => {
      toast.error(`Delegation failed: ${error.message}`);
    },
  });

  // Undelegate mutation
  const undelegateMutation = useMutation({
    mutationFn: async ({ validatorAddress, amount }: { validatorAddress: string; amount: string }) => {
      const client = await getSigningClient();
      const result = await client.undelegateTokens(
        wallet!.cosmos.address,
        validatorAddress,
        { denom: DENOM, amount },
        { amount: [{ denom: DENOM, amount: '5000' }], gas: '200000' },
        ''
      );
      return result;
    },
    onSuccess: () => {
      toast.success(`Undelegation started. Tokens will be available after ${UNBONDING_DAYS} days.`);
      refreshAll();
      queryClient.invalidateQueries({ queryKey: ['validators'] });
      setIsUnstakeModalOpen(false);
      setStakeAmount('');
    },
    onError: (error: Error) => {
      toast.error(`Undelegation failed: ${error.message}`);
    },
  });

  // Claim rewards mutation
  const claimRewardsMutation = useMutation({
    mutationFn: async () => {
      const client = await getSigningClient();
      // Claim from all validators with delegations
      const validatorAddresses = delegations.map(d => d.delegation.validator_address);

      const msgs = validatorAddresses.map(validatorAddress => ({
        typeUrl: '/cosmos.distribution.v1beta1.MsgWithdrawDelegatorReward',
        value: {
          delegatorAddress: wallet!.cosmos.address,
          validatorAddress,
        },
      }));

      const result = await client.signAndBroadcast(
        wallet!.cosmos.address,
        msgs,
        { amount: [{ denom: DENOM, amount: '5000' }], gas: '200000' }
      );
      return result;
    },
    onSuccess: () => {
      toast.success('Rewards claimed successfully!');
      refreshAll();
    },
    onError: (error: Error) => {
      toast.error(`Failed to claim rewards: ${error.message}`);
    },
  });

  const handleStake = () => {
    if (!selectedValidator || !stakeAmount) return;
    const amountInBase = parseAmount(stakeAmount);
    delegateMutation.mutate({
      validatorAddress: selectedValidator.operator_address,
      amount: amountInBase,
    });
  };

  const handleUnstake = () => {
    if (!selectedValidator || !stakeAmount) return;
    const amountInBase = parseAmount(stakeAmount);
    undelegateMutation.mutate({
      validatorAddress: selectedValidator.operator_address,
      amount: amountInBase,
    });
  };

  const openStakeModal = (validator: Validator) => {
    setSelectedValidator(validator);
    setStakeAmount('');
    setIsStakeModalOpen(true);
  };

  const openUnstakeModal = (validator: Validator) => {
    setSelectedValidator(validator);
    setStakeAmount('');
    setIsUnstakeModalOpen(true);
  };

  // Get user's delegation to a specific validator
  const getDelegation = (validatorAddress: string) => {
    return delegations.find(d => d.delegation.validator_address === validatorAddress);
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold text-dark-100">Staking</h1>
        <p className="text-dark-400 mt-1">Delegate your OMNI to validators and earn rewards</p>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <Card>
          <CardContent className="pt-6">
            <p className="text-sm text-dark-400">Available to Stake</p>
            <p className="text-2xl font-bold text-dark-100 mt-1 amount">
              {formatAmount(balance)} {DISPLAY_DENOM}
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-6">
            <p className="text-sm text-dark-400">Total Staked</p>
            <p className="text-2xl font-bold text-green-400 mt-1 amount">
              {formatAmount(totalStaked)} {DISPLAY_DENOM}
            </p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-6">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-dark-400">Pending Rewards</p>
                <p className="text-2xl font-bold text-yellow-400 mt-1 amount">
                  {formatAmount(totalRewards)} {DISPLAY_DENOM}
                </p>
              </div>
              {parseFloat(totalRewards) > 0 && (
                <Button
                  variant="primary"
                  size="sm"
                  onClick={() => claimRewardsMutation.mutate()}
                  isLoading={claimRewardsMutation.isPending}
                >
                  Claim
                </Button>
              )}
            </div>
          </CardContent>
        </Card>
      </div>

      {/* Validators List */}
      <Card>
        <CardHeader>
          <CardTitle>Validators</CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <div className="space-y-4">
              {[1, 2, 3].map((i) => (
                <div key={i} className="h-20 skeleton rounded-lg" />
              ))}
            </div>
          ) : validators.length === 0 ? (
            <p className="text-center text-dark-500 py-8">No validators found</p>
          ) : (
            <div className="space-y-3">
              {validators.map((validator) => {
                const userDelegation = getDelegation(validator.operator_address);
                const commission = parseFloat(validator.commission.commission_rates.rate) * 100;

                return (
                  <div
                    key={validator.operator_address}
                    className="p-4 bg-dark-800 rounded-lg hover:bg-dark-750 transition-colors"
                  >
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-4">
                        <div className="w-10 h-10 rounded-full bg-gradient-to-br from-omniphi-500 to-omniphi-700 flex items-center justify-center">
                          <span className="font-bold text-white text-sm">
                            {validator.description.moniker.charAt(0).toUpperCase()}
                          </span>
                        </div>
                        <div>
                          <div className="flex items-center gap-2">
                            <p className="font-medium text-dark-100">
                              {validator.description.moniker}
                            </p>
                            {validator.jailed && (
                              <Badge variant="error">Jailed</Badge>
                            )}
                            {userDelegation && (
                              <Badge variant="success">Delegated</Badge>
                            )}
                          </div>
                          <p className="text-sm text-dark-500 font-mono">
                            {truncateAddress(validator.operator_address, 12, 8)}
                          </p>
                        </div>
                      </div>

                      <div className="flex items-center gap-6">
                        <div className="text-right">
                          <p className="text-xs text-dark-500">Commission</p>
                          <p className="font-mono text-dark-200">{commission.toFixed(1)}%</p>
                        </div>
                        <div className="text-right">
                          <p className="text-xs text-dark-500">Voting Power</p>
                          <p className="font-mono text-dark-200">
                            {formatAmount(validator.tokens, 6, 0)}
                          </p>
                        </div>
                        {userDelegation && (
                          <div className="text-right">
                            <p className="text-xs text-dark-500">Your Stake</p>
                            <p className="font-mono text-green-400">
                              {formatAmount(userDelegation.balance.amount)}
                            </p>
                          </div>
                        )}
                        <div className="flex gap-2">
                          <Button
                            variant="primary"
                            size="sm"
                            onClick={() => openStakeModal(validator)}
                            disabled={validator.jailed}
                          >
                            Stake
                          </Button>
                          {userDelegation && (
                            <Button
                              variant="secondary"
                              size="sm"
                              onClick={() => openUnstakeModal(validator)}
                            >
                              Unstake
                            </Button>
                          )}
                        </div>
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Stake Modal */}
      <Modal
        isOpen={isStakeModalOpen}
        onClose={() => setIsStakeModalOpen(false)}
        title={`Stake to ${selectedValidator?.description.moniker || 'Validator'}`}
      >
        <div className="space-y-4">
          <div>
            <label className="label">Amount ({DISPLAY_DENOM})</label>
            <Input
              type="number"
              placeholder="0.00"
              value={stakeAmount}
              onChange={(e) => setStakeAmount(e.target.value)}
            />
            <div className="flex items-center justify-between mt-2">
              <span className="text-sm text-dark-500">
                Available: {formatAmount(balance)} {DISPLAY_DENOM}
              </span>
              <button
                type="button"
                className="text-sm text-omniphi-400 hover:text-omniphi-300"
                onClick={() => setStakeAmount(formatAmount(balance))}
              >
                Max
              </button>
            </div>
          </div>

          <div className="bg-dark-800 rounded-lg p-4 space-y-2">
            <div className="flex justify-between text-sm">
              <span className="text-dark-400">Validator Commission</span>
              <span className="text-dark-200">
                {selectedValidator ? (parseFloat(selectedValidator.commission.commission_rates.rate) * 100).toFixed(1) : 0}%
              </span>
            </div>
            <div className="flex justify-between text-sm">
              <span className="text-dark-400">Unbonding Period</span>
              <span className="text-dark-200">{UNBONDING_DAYS} days</span>
            </div>
          </div>

          <div className="flex gap-3">
            <Button
              variant="secondary"
              className="flex-1"
              onClick={() => setIsStakeModalOpen(false)}
            >
              Cancel
            </Button>
            <Button
              variant="primary"
              className="flex-1"
              onClick={handleStake}
              isLoading={delegateMutation.isPending}
              disabled={!stakeAmount || parseFloat(stakeAmount) <= 0}
            >
              Stake
            </Button>
          </div>
        </div>
      </Modal>

      {/* Unstake Modal */}
      <Modal
        isOpen={isUnstakeModalOpen}
        onClose={() => setIsUnstakeModalOpen(false)}
        title={`Unstake from ${selectedValidator?.description.moniker || 'Validator'}`}
      >
        <div className="space-y-4">
          <div className="bg-yellow-500/10 border border-yellow-500/20 rounded-lg p-4">
            <div className="flex gap-3">
              <svg className="w-5 h-5 text-yellow-400 flex-shrink-0 mt-0.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
              </svg>
              <p className="text-sm text-yellow-300">
                Unstaking takes {UNBONDING_DAYS} days. Your tokens will not earn rewards during this period.
              </p>
            </div>
          </div>

          <div>
            <label className="label">Amount ({DISPLAY_DENOM})</label>
            <Input
              type="number"
              placeholder="0.00"
              value={stakeAmount}
              onChange={(e) => setStakeAmount(e.target.value)}
            />
            {selectedValidator && getDelegation(selectedValidator.operator_address) && (
              <div className="flex items-center justify-between mt-2">
                <span className="text-sm text-dark-500">
                  Staked: {formatAmount(getDelegation(selectedValidator.operator_address)!.balance.amount)} {DISPLAY_DENOM}
                </span>
                <button
                  type="button"
                  className="text-sm text-omniphi-400 hover:text-omniphi-300"
                  onClick={() => {
                    const delegation = getDelegation(selectedValidator.operator_address);
                    if (delegation) {
                      setStakeAmount(formatAmount(delegation.balance.amount));
                    }
                  }}
                >
                  Max
                </button>
              </div>
            )}
          </div>

          <div className="flex gap-3">
            <Button
              variant="secondary"
              className="flex-1"
              onClick={() => setIsUnstakeModalOpen(false)}
            >
              Cancel
            </Button>
            <Button
              variant="danger"
              className="flex-1"
              onClick={handleUnstake}
              isLoading={undelegateMutation.isPending}
              disabled={!stakeAmount || parseFloat(stakeAmount) <= 0}
            >
              Unstake
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
};

export default Staking;
