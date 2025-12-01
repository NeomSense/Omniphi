import { Card } from '../ui/Card';
import { RewardsInfo } from '../../types/validator';

interface RewardsCardProps {
  rewards: RewardsInfo | null;
  loading?: boolean;
}

export function RewardsCard({ rewards, loading }: RewardsCardProps) {
  if (loading) {
    return (
      <Card title="Rewards">
        <div className="animate-pulse space-y-3">
          <div className="h-12 bg-gray-200 rounded"></div>
          <div className="h-8 bg-gray-200 rounded w-3/4"></div>
        </div>
      </Card>
    );
  }

  if (!rewards) {
    return (
      <Card title="Rewards">
        <p className="text-gray-500 text-sm">No rewards data available</p>
      </Card>
    );
  }

  return (
    <Card title="Staking Rewards">
      <div className="space-y-4">
        {/* APR/APY Display */}
        <div className="bg-gradient-to-r from-green-50 to-emerald-50 rounded-lg p-4 border border-green-100">
          <div className="flex items-center justify-between">
            <div>
              <p className="text-sm text-green-700 font-medium">Annual Percentage Rate</p>
              <p className="text-3xl font-bold text-green-600">{rewards.apr}</p>
            </div>
            {rewards.apy && (
              <div className="text-right">
                <p className="text-xs text-green-600">APY (compound)</p>
                <p className="text-lg font-semibold text-green-700">{rewards.apy}</p>
              </div>
            )}
          </div>
        </div>

        {/* Time-based Rewards */}
        <div className="grid grid-cols-3 gap-3">
          <div className="bg-gray-50 rounded-lg p-3 text-center">
            <p className="text-xs text-gray-500 mb-1">Daily</p>
            <p className="text-lg font-semibold text-gray-900">{rewards.daily}</p>
          </div>
          <div className="bg-gray-50 rounded-lg p-3 text-center">
            <p className="text-xs text-gray-500 mb-1">Weekly</p>
            <p className="text-lg font-semibold text-gray-900">{rewards.weekly}</p>
          </div>
          <div className="bg-gray-50 rounded-lg p-3 text-center">
            <p className="text-xs text-gray-500 mb-1">Monthly</p>
            <p className="text-lg font-semibold text-gray-900">{rewards.monthly}</p>
          </div>
        </div>

        {/* Unclaimed Rewards */}
        {rewards.unclaimed_rewards && (
          <div className="pt-3 border-t border-gray-100">
            <div className="flex items-center justify-between">
              <div>
                <p className="text-sm text-gray-600">Unclaimed Rewards</p>
                <p className="text-xl font-bold text-purple-600">{rewards.unclaimed_rewards}</p>
              </div>
              <button className="btn btn-primary text-sm">
                Claim All
              </button>
            </div>
          </div>
        )}

        {/* Total Rewards */}
        {rewards.total_rewards && (
          <div className="pt-3 border-t border-gray-100">
            <div className="flex items-center justify-between text-sm">
              <span className="text-gray-500">Total Earned</span>
              <span className="font-medium text-gray-900">{rewards.total_rewards}</span>
            </div>
          </div>
        )}

        {/* Next Reward */}
        <div className="pt-3 border-t border-gray-100">
          <div className="flex items-center justify-between text-sm">
            <span className="text-gray-500">Next Reward</span>
            <span className="font-medium text-gray-700">{rewards.next_reward}</span>
          </div>
        </div>
      </div>
    </Card>
  );
}
