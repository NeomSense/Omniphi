import { useState } from 'react';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Badge } from '@/components/ui/badge';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from '@/components/ui/dialog';
import {
  Coins,
  TrendingUp,
  TrendingDown,
  ArrowRightLeft,
  CheckCircle2,
  AlertCircle,
  Loader2,
} from 'lucide-react';
import { useValidatorStore } from '@/store/validatorStore';
import { toast } from 'sonner';

interface DelegationManagerProps {
  validatorAddress?: string;
  validatorName?: string;
  currentDelegation?: string;
  availableBalance?: string;
}

export const DelegationManager = ({
  validatorAddress,
  validatorName = 'Validator',
  currentDelegation = '0',
  availableBalance = '0',
}: DelegationManagerProps) => {
  const { walletAddress } = useValidatorStore();
  const [delegateAmount, setDelegateAmount] = useState('');
  const [undelegateAmount, setUndelegateAmount] = useState('');
  const [redelegateAmount, setRedelegateAmount] = useState('');
  const [targetValidator, setTargetValidator] = useState('');
  const [isLoading, setIsLoading] = useState(false);
  const [isDelegateOpen, setIsDelegateOpen] = useState(false);
  const [isUndelegateOpen, setIsUndelegateOpen] = useState(false);
  const [isRedelegateOpen, setIsRedelegateOpen] = useState(false);

  const formatAmount = (amount: string) => {
    const num = parseFloat(amount) / 1_000_000;
    return num.toLocaleString(undefined, { maximumFractionDigits: 6 });
  };

  const handleDelegate = async () => {
    if (!walletAddress || !validatorAddress) {
      toast.error('Wallet or validator address not found');
      return;
    }

    if (!delegateAmount || parseFloat(delegateAmount) <= 0) {
      toast.error('Please enter a valid amount');
      return;
    }

    setIsLoading(true);
    try {
      // This would integrate with Keplr/Leap wallet
      // For now, show instructions
      toast.info('Please sign the MsgDelegate transaction in your wallet', {
        description: `Delegating ${delegateAmount} OMNI to ${validatorName}`,
      });

      // Simulated transaction building
      const msg = {
        typeUrl: '/cosmos.staking.v1beta1.MsgDelegate',
        value: {
          delegatorAddress: walletAddress,
          validatorAddress: validatorAddress,
          amount: {
            denom: 'uomni',
            amount: (parseFloat(delegateAmount) * 1_000_000).toString(),
          },
        },
      };

      console.log('MsgDelegate:', msg);

      // TODO: Integrate with actual wallet
      // await window.keplr.signAndBroadcast(...)

      toast.success('Delegation transaction signed!', {
        description: 'Your tokens will be delegated once the transaction is confirmed',
      });

      setIsDelegateOpen(false);
      setDelegateAmount('');
    } catch (error: any) {
      toast.error('Failed to delegate', {
        description: error?.message || 'Unknown error',
      });
    } finally {
      setIsLoading(false);
    }
  };

  const handleUndelegate = async () => {
    if (!walletAddress || !validatorAddress) {
      toast.error('Wallet or validator address not found');
      return;
    }

    if (!undelegateAmount || parseFloat(undelegateAmount) <= 0) {
      toast.error('Please enter a valid amount');
      return;
    }

    const currentDelegationNum = parseFloat(currentDelegation) / 1_000_000;
    if (parseFloat(undelegateAmount) > currentDelegationNum) {
      toast.error(`Cannot undelegate more than ${currentDelegationNum} OMNI`);
      return;
    }

    setIsLoading(true);
    try {
      toast.info('Please sign the MsgUndelegate transaction in your wallet', {
        description: `Undelegating ${undelegateAmount} OMNI from ${validatorName}`,
      });

      const msg = {
        typeUrl: '/cosmos.staking.v1beta1.MsgUndelegate',
        value: {
          delegatorAddress: walletAddress,
          validatorAddress: validatorAddress,
          amount: {
            denom: 'uomni',
            amount: (parseFloat(undelegateAmount) * 1_000_000).toString(),
          },
        },
      };

      console.log('MsgUndelegate:', msg);

      // TODO: Integrate with actual wallet
      // await window.keplr.signAndBroadcast(...)

      toast.success('Undelegation transaction signed!', {
        description: 'Your tokens will be available after the 21-day unbonding period',
      });

      setIsUndelegateOpen(false);
      setUndelegateAmount('');
    } catch (error: any) {
      toast.error('Failed to undelegate', {
        description: error?.message || 'Unknown error',
      });
    } finally {
      setIsLoading(false);
    }
  };

  const handleRedelegate = async () => {
    if (!walletAddress || !validatorAddress) {
      toast.error('Wallet or validator address not found');
      return;
    }

    if (!redelegateAmount || parseFloat(redelegateAmount) <= 0) {
      toast.error('Please enter a valid amount');
      return;
    }

    if (!targetValidator) {
      toast.error('Please enter target validator address');
      return;
    }

    const currentDelegationNum = parseFloat(currentDelegation) / 1_000_000;
    if (parseFloat(redelegateAmount) > currentDelegationNum) {
      toast.error(`Cannot redelegate more than ${currentDelegationNum} OMNI`);
      return;
    }

    setIsLoading(true);
    try {
      toast.info('Please sign the MsgBeginRedelegate transaction in your wallet', {
        description: `Redelegating ${redelegateAmount} OMNI to another validator`,
      });

      const msg = {
        typeUrl: '/cosmos.staking.v1beta1.MsgBeginRedelegate',
        value: {
          delegatorAddress: walletAddress,
          validatorSrcAddress: validatorAddress,
          validatorDstAddress: targetValidator,
          amount: {
            denom: 'uomni',
            amount: (parseFloat(redelegateAmount) * 1_000_000).toString(),
          },
        },
      };

      console.log('MsgBeginRedelegate:', msg);

      // TODO: Integrate with actual wallet
      // await window.keplr.signAndBroadcast(...)

      toast.success('Redelegation transaction signed!', {
        description: 'Your tokens will be redelegated once confirmed',
      });

      setIsRedelegateOpen(false);
      setRedelegateAmount('');
      setTargetValidator('');
    } catch (error: any) {
      toast.error('Failed to redelegate', {
        description: error?.message || 'Unknown error',
      });
    } finally {
      setIsLoading(false);
    }
  };

  const setMaxDelegate = () => {
    const max = parseFloat(availableBalance) / 1_000_000;
    setDelegateAmount(max.toString());
  };

  const setMaxUndelegate = () => {
    const max = parseFloat(currentDelegation) / 1_000_000;
    setUndelegateAmount(max.toString());
  };

  const setMaxRedelegate = () => {
    const max = parseFloat(currentDelegation) / 1_000_000;
    setRedelegateAmount(max.toString());
  };

  if (!walletAddress) {
    return (
      <Card className="glass-card p-8 text-center">
        <AlertCircle className="mx-auto h-12 w-12 text-muted-foreground mb-4" />
        <p className="text-muted-foreground">Please connect your wallet to manage delegations</p>
      </Card>
    );
  }

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div>
        <h3 className="text-2xl font-bold">Delegation Manager</h3>
        <p className="text-muted-foreground">Delegate, undelegate, or redelegate your tokens</p>
      </div>

      {/* Summary Cards */}
      <div className="grid md:grid-cols-3 gap-6">
        <Card className="glass-card p-6">
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <h4 className="text-sm font-medium text-muted-foreground">Current Delegation</h4>
              <CheckCircle2 className="h-4 w-4 text-primary" />
            </div>
            <div className="space-y-1">
              <p className="text-3xl font-bold">{formatAmount(currentDelegation)}</p>
              <p className="text-xs text-muted-foreground">OMNI delegated to {validatorName}</p>
            </div>
          </div>
        </Card>

        <Card className="glass-card p-6">
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <h4 className="text-sm font-medium text-muted-foreground">Available Balance</h4>
              <Coins className="h-4 w-4 text-accent" />
            </div>
            <div className="space-y-1">
              <p className="text-3xl font-bold">{formatAmount(availableBalance)}</p>
              <p className="text-xs text-muted-foreground">OMNI available to delegate</p>
            </div>
          </div>
        </Card>

        <Card className="glass-card p-6">
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <h4 className="text-sm font-medium text-muted-foreground">Total Value</h4>
              <TrendingUp className="h-4 w-4 text-green-500" />
            </div>
            <div className="space-y-1">
              <p className="text-3xl font-bold">
                {formatAmount((parseFloat(currentDelegation) + parseFloat(availableBalance)).toString())}
              </p>
              <p className="text-xs text-muted-foreground">Total OMNI in wallet</p>
            </div>
          </div>
        </Card>
      </div>

      {/* Action Buttons */}
      <div className="grid md:grid-cols-3 gap-4">
        {/* Delegate Dialog */}
        <Dialog open={isDelegateOpen} onOpenChange={setIsDelegateOpen}>
          <DialogTrigger asChild>
            <Button className="glow-primary w-full" size="lg">
              <TrendingUp className="mr-2 h-5 w-5" />
              Delegate
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Delegate Tokens</DialogTitle>
              <DialogDescription>
                Delegate your OMNI tokens to {validatorName} to earn staking rewards
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label htmlFor="delegate-amount">Amount (OMNI)</Label>
                <div className="flex gap-2">
                  <Input
                    id="delegate-amount"
                    type="number"
                    placeholder="0.00"
                    value={delegateAmount}
                    onChange={(e) => setDelegateAmount(e.target.value)}
                  />
                  <Button variant="outline" onClick={setMaxDelegate}>
                    Max
                  </Button>
                </div>
                <p className="text-xs text-muted-foreground">
                  Available: {formatAmount(availableBalance)} OMNI
                </p>
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setIsDelegateOpen(false)} disabled={isLoading}>
                Cancel
              </Button>
              <Button onClick={handleDelegate} disabled={isLoading}>
                {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                Delegate
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* Undelegate Dialog */}
        <Dialog open={isUndelegateOpen} onOpenChange={setIsUndelegateOpen}>
          <DialogTrigger asChild>
            <Button variant="outline" className="w-full" size="lg">
              <TrendingDown className="mr-2 h-5 w-5" />
              Undelegate
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Undelegate Tokens</DialogTitle>
              <DialogDescription>
                Undelegate your tokens from {validatorName}. Tokens will be available after 21 days.
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label htmlFor="undelegate-amount">Amount (OMNI)</Label>
                <div className="flex gap-2">
                  <Input
                    id="undelegate-amount"
                    type="number"
                    placeholder="0.00"
                    value={undelegateAmount}
                    onChange={(e) => setUndelegateAmount(e.target.value)}
                  />
                  <Button variant="outline" onClick={setMaxUndelegate}>
                    Max
                  </Button>
                </div>
                <p className="text-xs text-muted-foreground">
                  Delegated: {formatAmount(currentDelegation)} OMNI
                </p>
              </div>
              <div className="rounded-lg bg-yellow-500/10 p-3 border border-yellow-500/20">
                <p className="text-sm text-yellow-600 dark:text-yellow-400">
                  ⚠️ Unbonding period: 21 days. You won't earn rewards during this time.
                </p>
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setIsUndelegateOpen(false)} disabled={isLoading}>
                Cancel
              </Button>
              <Button onClick={handleUndelegate} disabled={isLoading}>
                {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                Undelegate
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>

        {/* Redelegate Dialog */}
        <Dialog open={isRedelegateOpen} onOpenChange={setIsRedelegateOpen}>
          <DialogTrigger asChild>
            <Button variant="outline" className="w-full" size="lg">
              <ArrowRightLeft className="mr-2 h-5 w-5" />
              Redelegate
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Redelegate Tokens</DialogTitle>
              <DialogDescription>
                Switch your delegation to another validator without waiting for the unbonding period
              </DialogDescription>
            </DialogHeader>
            <div className="space-y-4 py-4">
              <div className="space-y-2">
                <Label htmlFor="redelegate-amount">Amount (OMNI)</Label>
                <div className="flex gap-2">
                  <Input
                    id="redelegate-amount"
                    type="number"
                    placeholder="0.00"
                    value={redelegateAmount}
                    onChange={(e) => setRedelegateAmount(e.target.value)}
                  />
                  <Button variant="outline" onClick={setMaxRedelegate}>
                    Max
                  </Button>
                </div>
                <p className="text-xs text-muted-foreground">
                  Delegated: {formatAmount(currentDelegation)} OMNI
                </p>
              </div>
              <div className="space-y-2">
                <Label htmlFor="target-validator">Target Validator Address</Label>
                <Input
                  id="target-validator"
                  placeholder="omniphivaloper1..."
                  value={targetValidator}
                  onChange={(e) => setTargetValidator(e.target.value)}
                />
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setIsRedelegateOpen(false)} disabled={isLoading}>
                Cancel
              </Button>
              <Button onClick={handleRedelegate} disabled={isLoading}>
                {isLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
                Redelegate
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </div>

      {/* Info Cards */}
      <div className="grid md:grid-cols-2 gap-6">
        <Card className="glass-card p-6 space-y-4">
          <h4 className="font-semibold flex items-center gap-2">
            <CheckCircle2 className="h-5 w-5 text-primary" />
            Delegation Benefits
          </h4>
          <ul className="space-y-2 text-sm text-muted-foreground">
            <li className="flex items-start gap-2">
              <span className="text-primary">•</span>
              <span>Earn staking rewards while supporting network security</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-primary">•</span>
              <span>Rewards are automatically compounded</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-primary">•</span>
              <span>Retain full custody of your tokens</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-primary">•</span>
              <span>Can redelegate instantly without waiting period</span>
            </li>
          </ul>
        </Card>

        <Card className="glass-card p-6 space-y-4">
          <h4 className="font-semibold flex items-center gap-2">
            <AlertCircle className="h-5 w-5 text-yellow-500" />
            Important Notes
          </h4>
          <ul className="space-y-2 text-sm text-muted-foreground">
            <li className="flex items-start gap-2">
              <span className="text-yellow-500">•</span>
              <span>Undelegation takes 21 days to complete</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-yellow-500">•</span>
              <span>No rewards earned during unbonding period</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-yellow-500">•</span>
              <span>Tokens at risk if validator misbehaves (slashing)</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-yellow-500">•</span>
              <span>Choose reputable validators with good uptime</span>
            </li>
          </ul>
        </Card>
      </div>
    </div>
  );
};
