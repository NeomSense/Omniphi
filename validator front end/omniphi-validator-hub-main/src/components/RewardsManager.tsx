import { useState, useEffect } from 'react';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';
import {
  Gift,
  TrendingUp,
  Coins,
  Calendar,
  CheckCircle2,
  Loader2,
  ExternalLink,
} from 'lucide-react';
import { useValidatorStore } from '@/store/validatorStore';
import { toast } from 'sonner';

interface RewardsData {
  totalRewards: string;
  dailyRewards: string;
  weeklyRewards: string;
  monthlyRewards: string;
  apr: number;
  lastClaimTime?: string;
  nextRewardEstimate: string;
}

interface RewardsManagerProps {
  validatorAddress?: string;
  validatorName?: string;
}

export const RewardsManager = ({
  validatorAddress,
  validatorName = 'Validator',
}: RewardsManagerProps) => {
  const { walletAddress } = useValidatorStore();
  const [rewards, setRewards] = useState<RewardsData>({
    totalRewards: '0',
    dailyRewards: '0',
    weeklyRewards: '0',
    monthlyRewards: '0',
    apr: 0,
    nextRewardEstimate: '0',
  });
  const [loading, setLoading] = useState(false);
  const [claiming, setClaiming] = useState(false);

  useEffect(() => {
    if (walletAddress && validatorAddress) {
      fetchRewards();
    }
  }, [walletAddress, validatorAddress]);

  const fetchRewards = async () => {
    setLoading(true);
    try {
      // TODO: Implement actual RPC call to fetch rewards
      // const response = await fetch(`${RPC_URL}/cosmos/distribution/v1beta1/delegators/${walletAddress}/rewards/${validatorAddress}`);

      // Mock data for now
      setRewards({
        totalRewards: '1234.567890',
        dailyRewards: '12.345678',
        weeklyRewards: '86.419746',
        monthlyRewards: '370.370370',
        apr: 15.5,
        lastClaimTime: new Date(Date.now() - 86400000 * 3).toISOString(),
        nextRewardEstimate: '0.5',
      });
    } catch (error) {
      console.error('Failed to fetch rewards:', error);
    } finally {
      setLoading(false);
    }
  };

  const handleClaimRewards = async () => {
    if (!walletAddress || !validatorAddress) {
      toast.error('Wallet or validator address not found');
      return;
    }

    if (parseFloat(rewards.totalRewards) <= 0) {
      toast.error('No rewards available to claim');
      return;
    }

    setClaiming(true);
    try {
      toast.info('Please sign the MsgWithdrawDelegatorReward transaction', {
        description: `Claiming ${rewards.totalRewards} OMNI from ${validatorName}`,
      });

      const msg = {
        typeUrl: '/cosmos.distribution.v1beta1.MsgWithdrawDelegatorReward',
        value: {
          delegatorAddress: walletAddress,
          validatorAddress: validatorAddress,
        },
      };

      console.log('MsgWithdrawDelegatorReward:', msg);

      // TODO: Integrate with actual wallet
      // await window.keplr.signAndBroadcast(...)

      toast.success('Rewards claimed successfully!', {
        description: `${rewards.totalRewards} OMNI has been sent to your wallet`,
      });

      // Refresh rewards after claiming
      setTimeout(fetchRewards, 2000);
    } catch (error: any) {
      toast.error('Failed to claim rewards', {
        description: error?.message || 'Unknown error',
      });
    } finally {
      setClaiming(false);
    }
  };

  const handleClaimAll = async () => {
    if (!walletAddress) {
      toast.error('Wallet not connected');
      return;
    }

    setClaiming(true);
    try {
      toast.info('Please sign the MsgWithdrawAllDelegatorRewards transaction', {
        description: 'Claiming rewards from all validators',
      });

      // TODO: Fetch all validator addresses first
      // Then create msg for each validator

      const msg = {
        typeUrl: '/cosmos.distribution.v1beta1.MsgWithdrawAllDelegatorRewards',
        value: {
          delegatorAddress: walletAddress,
        },
      };

      console.log('MsgWithdrawAllDelegatorRewards:', msg);

      toast.success('All rewards claimed successfully!');

      setTimeout(fetchRewards, 2000);
    } catch (error: any) {
      toast.error('Failed to claim all rewards', {
        description: error?.message || 'Unknown error',
      });
    } finally {
      setClaiming(false);
    }
  };

  const formatAmount = (amount: string) => {
    const num = parseFloat(amount);
    if (isNaN(num)) return '0.00';
    return num.toLocaleString(undefined, {
      minimumFractionDigits: 2,
      maximumFractionDigits: 6,
    });
  };

  const getDaysSinceLastClaim = () => {
    if (!rewards.lastClaimTime) return 0;
    const lastClaim = new Date(rewards.lastClaimTime);
    const now = new Date();
    const diff = now.getTime() - lastClaim.getTime();
    return Math.floor(diff / (1000 * 60 * 60 * 24));
  };

  if (!walletAddress) {
    return (
      <Card className="glass-card p-8 text-center">
        <Gift className="mx-auto h-12 w-12 text-muted-foreground mb-4" />
        <p className="text-muted-foreground">Please connect your wallet to view rewards</p>
      </Card>
    );
  }

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <h3 className="text-2xl font-bold">Staking Rewards</h3>
          <p className="text-muted-foreground">Track and claim your validator rewards</p>
        </div>
        <div className="flex gap-2">
          <Button variant="outline" onClick={fetchRewards} disabled={loading}>
            {loading ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : null}
            Refresh
          </Button>
          <Button onClick={handleClaimAll} disabled={claiming} className="glow-primary">
            {claiming ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Gift className="mr-2 h-4 w-4" />}
            Claim All
          </Button>
        </div>
      </div>

      {/* Total Rewards Card */}
      <Card className="glass-card p-8 text-center space-y-4">
        <div className="inline-flex items-center justify-center w-16 h-16 rounded-full bg-primary/10 mb-2">
          <Gift className="h-8 w-8 text-primary" />
        </div>
        <div>
          <p className="text-sm text-muted-foreground mb-2">Total Unclaimed Rewards</p>
          <p className="text-5xl font-bold bg-gradient-to-r from-primary to-accent bg-clip-text text-transparent">
            {formatAmount(rewards.totalRewards)}
          </p>
          <p className="text-lg text-muted-foreground mt-2">OMNI</p>
        </div>
        <Button
          size="lg"
          onClick={handleClaimRewards}
          disabled={claiming || parseFloat(rewards.totalRewards) <= 0}
          className="glow-primary"
        >
          {claiming ? (
            <>
              <Loader2 className="mr-2 h-5 w-5 animate-spin" />
              Claiming...
            </>
          ) : (
            <>
              <CheckCircle2 className="mr-2 h-5 w-5" />
              Claim Rewards
            </>
          )}
        </Button>
      </Card>

      {/* Rewards Breakdown */}
      <div className="grid md:grid-cols-4 gap-4">
        <Card className="glass-card p-6">
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <h4 className="text-sm font-medium text-muted-foreground">Daily Rewards</h4>
              <Calendar className="h-4 w-4 text-primary" />
            </div>
            <div>
              <p className="text-2xl font-bold">{formatAmount(rewards.dailyRewards)}</p>
              <p className="text-xs text-muted-foreground">OMNI per day</p>
            </div>
          </div>
        </Card>

        <Card className="glass-card p-6">
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <h4 className="text-sm font-medium text-muted-foreground">Weekly Rewards</h4>
              <Calendar className="h-4 w-4 text-accent" />
            </div>
            <div>
              <p className="text-2xl font-bold">{formatAmount(rewards.weeklyRewards)}</p>
              <p className="text-xs text-muted-foreground">OMNI per week</p>
            </div>
          </div>
        </Card>

        <Card className="glass-card p-6">
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <h4 className="text-sm font-medium text-muted-foreground">Monthly Rewards</h4>
              <Calendar className="h-4 w-4 text-green-500" />
            </div>
            <div>
              <p className="text-2xl font-bold">{formatAmount(rewards.monthlyRewards)}</p>
              <p className="text-xs text-muted-foreground">OMNI per month</p>
            </div>
          </div>
        </Card>

        <Card className="glass-card p-6">
          <div className="space-y-3">
            <div className="flex items-center justify-between">
              <h4 className="text-sm font-medium text-muted-foreground">APR</h4>
              <TrendingUp className="h-4 w-4 text-blue-500" />
            </div>
            <div>
              <p className="text-2xl font-bold">{rewards.apr.toFixed(2)}%</p>
              <p className="text-xs text-muted-foreground">Annual return</p>
            </div>
          </div>
        </Card>
      </div>

      {/* Claim Status */}
      <Card className="glass-card p-6 space-y-4">
        <div className="flex items-center justify-between">
          <div>
            <h4 className="font-semibold">Last Claim</h4>
            <p className="text-sm text-muted-foreground">
              {rewards.lastClaimTime
                ? `${getDaysSinceLastClaim()} days ago`
                : 'Never claimed'
              }
            </p>
          </div>
          <Badge variant="outline">
            {getDaysSinceLastClaim() > 7 ? '⚠️ Consider claiming' : '✓ Recent'}
          </Badge>
        </div>

        <div className="space-y-2">
          <div className="flex justify-between text-sm">
            <span className="text-muted-foreground">Accumulation Progress</span>
            <span className="font-medium">{getDaysSinceLastClaim()} / 30 days</span>
          </div>
          <Progress value={(getDaysSinceLastClaim() / 30) * 100} className="h-2" />
        </div>
      </Card>

      {/* Info Cards */}
      <div className="grid md:grid-cols-2 gap-6">
        <Card className="glass-card p-6 space-y-4">
          <h4 className="font-semibold flex items-center gap-2">
            <Coins className="h-5 w-5 text-primary" />
            How Rewards Work
          </h4>
          <ul className="space-y-2 text-sm text-muted-foreground">
            <li className="flex items-start gap-2">
              <span className="text-primary">•</span>
              <span>Rewards accumulate continuously as you stake</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-primary">•</span>
              <span>Claim anytime - rewards are yours to keep</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-primary">•</span>
              <span>Consider claiming and re-staking for compound growth</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-primary">•</span>
              <span>Gas fees apply when claiming rewards</span>
            </li>
          </ul>
        </Card>

        <Card className="glass-card p-6 space-y-4">
          <h4 className="font-semibold flex items-center gap-2">
            <TrendingUp className="h-5 w-5 text-green-500" />
            Maximize Your Rewards
          </h4>
          <ul className="space-y-2 text-sm text-muted-foreground">
            <li className="flex items-start gap-2">
              <span className="text-green-500">•</span>
              <span>Claim and re-delegate for compounding effects</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-green-500">•</span>
              <span>Choose validators with high uptime (99%+)</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-green-500">•</span>
              <span>Diversify across multiple validators for safety</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-green-500">•</span>
              <span>Monitor validator performance regularly</span>
            </li>
          </ul>
        </Card>
      </div>

      {/* Reward History Preview */}
      <Card className="glass-card p-6 space-y-4">
        <div className="flex items-center justify-between">
          <h4 className="font-semibold">Recent Claims</h4>
          <Button variant="ghost" size="sm">
            View All
            <ExternalLink className="ml-2 h-3 w-3" />
          </Button>
        </div>

        <div className="space-y-3">
          {rewards.lastClaimTime ? (
            <div className="flex items-center justify-between p-3 rounded-lg bg-card/50">
              <div className="flex items-center gap-3">
                <CheckCircle2 className="h-5 w-5 text-green-500" />
                <div>
                  <p className="font-medium">Claimed Rewards</p>
                  <p className="text-xs text-muted-foreground">
                    {new Date(rewards.lastClaimTime).toLocaleDateString()}
                  </p>
                </div>
              </div>
              <div className="text-right">
                <p className="font-medium">+{formatAmount(rewards.totalRewards)} OMNI</p>
                <p className="text-xs text-muted-foreground">From {validatorName}</p>
              </div>
            </div>
          ) : (
            <div className="text-center py-8 text-muted-foreground">
              <Gift className="mx-auto h-8 w-8 mb-2 opacity-50" />
              <p className="text-sm">No claim history yet</p>
            </div>
          )}
        </div>
      </Card>
    </div>
  );
};
