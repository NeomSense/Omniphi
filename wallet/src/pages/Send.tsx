/**
 * Send Page
 * Send tokens to another address
 */

import React, { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { useWalletStore } from '@/stores/wallet';
import { Card, CardHeader, CardTitle, CardContent } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { formatAmount, parseAmount, isValidAddress } from '@/lib/utils';
import { DISPLAY_DENOM, DENOM, BECH32_PREFIX } from '@/lib/constants';
import toast from 'react-hot-toast';

const Send: React.FC = () => {
  const { wallet, balance, refreshBalance, getSigningClient } = useWalletStore();

  const [recipient, setRecipient] = useState('');
  const [amount, setAmount] = useState('');
  const [memo, setMemo] = useState('');
  const [isConfirming, setIsConfirming] = useState(false);

  // Send mutation
  const sendMutation = useMutation({
    mutationFn: async () => {
      const client = await getSigningClient();
      const amountInBase = parseAmount(amount);

      const result = await client.sendTokens(
        wallet!.cosmos.address,
        recipient,
        [{ denom: DENOM, amount: amountInBase }],
        { amount: [{ denom: DENOM, amount: '5000' }], gas: '100000' },
        memo || undefined
      );
      return result;
    },
    onSuccess: (result) => {
      toast.success(`Transaction successful! Hash: ${result.transactionHash.slice(0, 12)}...`);
      refreshBalance();
      // Reset form
      setRecipient('');
      setAmount('');
      setMemo('');
      setIsConfirming(false);
    },
    onError: (error: Error) => {
      toast.error(`Transaction failed: ${error.message}`);
      setIsConfirming(false);
    },
  });

  const validateForm = (): boolean => {
    if (!recipient.trim()) {
      toast.error('Please enter a recipient address');
      return false;
    }

    if (!isValidAddress(recipient, BECH32_PREFIX)) {
      toast.error('Invalid recipient address');
      return false;
    }

    if (recipient === wallet?.cosmos.address) {
      toast.error('Cannot send to yourself');
      return false;
    }

    if (!amount || parseFloat(amount) <= 0) {
      toast.error('Please enter a valid amount');
      return false;
    }

    const amountInBase = parseAmount(amount);
    if (BigInt(amountInBase) > BigInt(balance)) {
      toast.error('Insufficient balance');
      return false;
    }

    return true;
  };

  const handleReview = () => {
    if (!validateForm()) return;
    setIsConfirming(true);
  };

  const handleSend = () => {
    if (!validateForm()) return;
    sendMutation.mutate();
  };

  const handleBack = () => {
    setIsConfirming(false);
  };

  const handleMaxAmount = () => {
    // Leave some for gas
    const maxAmount = BigInt(balance) - BigInt(10000);
    if (maxAmount > 0n) {
      setAmount(formatAmount(maxAmount.toString()));
    }
  };

  // Confirmation screen
  if (isConfirming) {
    return (
      <div className="max-w-lg mx-auto space-y-6">
        <div>
          <h1 className="text-2xl font-bold text-dark-100">Confirm Transaction</h1>
          <p className="text-dark-400 mt-1">Please review the details before sending</p>
        </div>

        <Card>
          <CardContent className="pt-6 space-y-6">
            {/* Amount */}
            <div className="text-center py-6 border-b border-dark-700">
              <p className="text-sm text-dark-500 mb-2">Sending</p>
              <p className="text-4xl font-bold text-dark-100 amount">
                {amount} <span className="text-xl text-dark-400">{DISPLAY_DENOM}</span>
              </p>
            </div>

            {/* Details */}
            <div className="space-y-4">
              <div>
                <p className="text-sm text-dark-500">From</p>
                <p className="font-mono text-sm text-dark-200 break-all">{wallet?.cosmos.address}</p>
              </div>
              <div>
                <p className="text-sm text-dark-500">To</p>
                <p className="font-mono text-sm text-dark-200 break-all">{recipient}</p>
              </div>
              {memo && (
                <div>
                  <p className="text-sm text-dark-500">Memo</p>
                  <p className="text-dark-200">{memo}</p>
                </div>
              )}
              <div>
                <p className="text-sm text-dark-500">Estimated Fee</p>
                <p className="font-mono text-dark-200">0.005 {DISPLAY_DENOM}</p>
              </div>
            </div>

            {/* Actions */}
            <div className="flex gap-3 pt-4">
              <Button
                variant="secondary"
                className="flex-1"
                onClick={handleBack}
                disabled={sendMutation.isPending}
              >
                Back
              </Button>
              <Button
                variant="primary"
                className="flex-1"
                onClick={handleSend}
                isLoading={sendMutation.isPending}
              >
                Confirm & Send
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    );
  }

  // Send form
  return (
    <div className="max-w-lg mx-auto space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-dark-100">Send</h1>
        <p className="text-dark-400 mt-1">Send OMNI to another address</p>
      </div>

      <Card>
        <CardContent className="pt-6 space-y-6">
          {/* Balance */}
          <div className="p-4 bg-dark-800 rounded-lg">
            <p className="text-sm text-dark-500">Available Balance</p>
            <p className="text-xl font-bold text-dark-100 amount">
              {formatAmount(balance)} {DISPLAY_DENOM}
            </p>
          </div>

          {/* Recipient */}
          <div>
            <label className="label">Recipient Address</label>
            <Input
              type="text"
              placeholder={`${BECH32_PREFIX}1...`}
              value={recipient}
              onChange={(e) => setRecipient(e.target.value)}
            />
          </div>

          {/* Amount */}
          <div>
            <label className="label">Amount ({DISPLAY_DENOM})</label>
            <div className="relative">
              <Input
                type="number"
                placeholder="0.00"
                value={amount}
                onChange={(e) => setAmount(e.target.value)}
              />
              <button
                type="button"
                className="absolute right-3 top-1/2 -translate-y-1/2 text-sm text-omniphi-400 hover:text-omniphi-300"
                onClick={handleMaxAmount}
              >
                Max
              </button>
            </div>
          </div>

          {/* Memo (optional) */}
          <div>
            <label className="label">
              Memo <span className="text-dark-600">(optional)</span>
            </label>
            <Input
              type="text"
              placeholder="Add a note..."
              value={memo}
              onChange={(e) => setMemo(e.target.value)}
            />
          </div>

          {/* Estimated fee */}
          <div className="flex items-center justify-between p-4 bg-dark-800 rounded-lg">
            <span className="text-sm text-dark-400">Estimated Fee</span>
            <span className="font-mono text-dark-200">~0.005 {DISPLAY_DENOM}</span>
          </div>

          {/* Submit */}
          <Button
            variant="primary"
            size="lg"
            className="w-full"
            onClick={handleReview}
            disabled={!recipient || !amount}
          >
            Review Transaction
          </Button>
        </CardContent>
      </Card>
    </div>
  );
};

export default Send;
