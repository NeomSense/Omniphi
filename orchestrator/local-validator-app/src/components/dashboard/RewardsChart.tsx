import { useState, useEffect } from 'react';
import { Card } from '../ui/Card';
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, Legend } from 'recharts';

interface RewardsChartProps {
  dailyReward?: string;
}

interface RewardDataPoint {
  date: string;
  rewards: number;
  commission: number;
}

export function RewardsChart({ dailyReward = '12.5 OMNI' }: RewardsChartProps) {
  const [data, setData] = useState<RewardDataPoint[]>([]);
  const [timeRange, setTimeRange] = useState<'7d' | '30d' | '90d'>('7d');

  useEffect(() => {
    // Generate mock historical rewards data
    const days = timeRange === '7d' ? 7 : timeRange === '30d' ? 30 : 90;
    const baseReward = parseFloat(dailyReward) || 12.5;

    const historicalData: RewardDataPoint[] = Array.from({ length: days }, (_, i) => {
      const date = new Date();
      date.setDate(date.getDate() - (days - 1 - i));
      const variance = (Math.random() - 0.5) * 4; // +/- 2 OMNI variance
      const rewards = Math.max(0, baseReward + variance);
      const commission = rewards * 0.1; // 10% commission

      return {
        date: date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' }),
        rewards: parseFloat(rewards.toFixed(2)),
        commission: parseFloat(commission.toFixed(2)),
      };
    });

    setData(historicalData);
  }, [timeRange, dailyReward]);

  const totalRewards = data.reduce((sum, d) => sum + d.rewards, 0);
  const totalCommission = data.reduce((sum, d) => sum + d.commission, 0);
  const avgDaily = data.length > 0 ? totalRewards / data.length : 0;

  return (
    <Card title="Rewards History">
      <div className="space-y-4">
        {/* Time Range Selector */}
        <div className="flex items-center justify-between">
          <div className="flex space-x-2">
            {(['7d', '30d', '90d'] as const).map(range => (
              <button
                key={range}
                onClick={() => setTimeRange(range)}
                className={`px-3 py-1 rounded text-sm font-medium transition-colors ${
                  timeRange === range
                    ? 'bg-green-600 text-white'
                    : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
                }`}
              >
                {range}
              </button>
            ))}
          </div>
          <div className="text-sm text-gray-500">
            Avg: {avgDaily.toFixed(2)} OMNI/day
          </div>
        </div>

        {/* Chart */}
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
            <BarChart data={data} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
              <CartesianGrid strokeDasharray="3 3" stroke="#E5E7EB" />
              <XAxis
                dataKey="date"
                tick={{ fontSize: 11, fill: '#6B7280' }}
                tickLine={false}
                axisLine={{ stroke: '#E5E7EB' }}
                interval={timeRange === '7d' ? 0 : timeRange === '30d' ? 4 : 14}
              />
              <YAxis
                tick={{ fontSize: 12, fill: '#6B7280' }}
                tickLine={false}
                axisLine={{ stroke: '#E5E7EB' }}
                tickFormatter={(value) => `${value}`}
              />
              <Tooltip
                contentStyle={{
                  backgroundColor: '#1F2937',
                  border: 'none',
                  borderRadius: '8px',
                  color: '#F3F4F6',
                }}
                labelStyle={{ color: '#9CA3AF' }}
                formatter={(value: number, name: string) => [
                  `${value.toFixed(2)} OMNI`,
                  name === 'rewards' ? 'Staking Rewards' : 'Commission'
                ]}
              />
              <Legend
                wrapperStyle={{ paddingTop: '10px' }}
                formatter={(value) => value === 'rewards' ? 'Staking Rewards' : 'Commission'}
              />
              <Bar dataKey="rewards" fill="#10B981" radius={[4, 4, 0, 0]} />
              <Bar dataKey="commission" fill="#8B5CF6" radius={[4, 4, 0, 0]} />
            </BarChart>
          </ResponsiveContainer>
        </div>

        {/* Stats */}
        <div className="grid grid-cols-3 gap-4 pt-4 border-t border-gray-100">
          <div className="text-center">
            <p className="text-xs text-gray-500">Total Rewards</p>
            <p className="text-lg font-bold text-green-600">{totalRewards.toFixed(2)} OMNI</p>
          </div>
          <div className="text-center">
            <p className="text-xs text-gray-500">Commission Earned</p>
            <p className="text-lg font-bold text-purple-600">{totalCommission.toFixed(2)} OMNI</p>
          </div>
          <div className="text-center">
            <p className="text-xs text-gray-500">Est. Monthly</p>
            <p className="text-lg font-bold text-gray-900">{(avgDaily * 30).toFixed(2)} OMNI</p>
          </div>
        </div>
      </div>
    </Card>
  );
}
