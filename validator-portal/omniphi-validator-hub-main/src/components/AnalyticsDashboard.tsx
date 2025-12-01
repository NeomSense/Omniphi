import { useState, useEffect } from 'react';
import { Card } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from '@/components/ui/chart';
import {
  LineChart,
  Line,
  BarChart,
  Bar,
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  ResponsiveContainer,
  Legend,
} from 'recharts';
import {
  TrendingUp,
  TrendingDown,
  Activity,
  DollarSign,
  Users,
  Award,
  Calendar,
} from 'lucide-react';

type TimeRange = '7d' | '30d' | '90d' | '1y' | 'all';

interface AnalyticsData {
  rewards: { date: string; amount: number }[];
  votingPower: { date: string; power: number }[];
  delegations: { date: string; count: number; amount: number }[];
  uptime: { date: string; percentage: number }[];
  performance: {
    totalRewardsEarned: number;
    averageDailyRewards: number;
    currentVotingPower: number;
    uptimePercentage: number;
    totalDelegators: number;
    rank: number;
  };
}

export const AnalyticsDashboard = () => {
  const [timeRange, setTimeRange] = useState<TimeRange>('30d');
  const [analytics, setAnalytics] = useState<AnalyticsData | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchAnalytics();
  }, [timeRange]);

  const fetchAnalytics = async () => {
    setLoading(true);
    try {
      // TODO: Fetch actual analytics data from backend/blockchain

      // Mock data for demonstration
      const mockData: AnalyticsData = {
        rewards: generateMockTimeSeriesData(30, 10, 15),
        votingPower: generateMockTimeSeriesData(30, 1000, 1500),
        delegations: generateMockDelegationData(30),
        uptime: generateMockUptimeData(30),
        performance: {
          totalRewardsEarned: 12547.89,
          averageDailyRewards: 41.83,
          currentVotingPower: 15234567,
          uptimePercentage: 99.87,
          totalDelegators: 1247,
          rank: 15,
        },
      };

      setAnalytics(mockData);
    } catch (error) {
      console.error('Failed to fetch analytics:', error);
    } finally {
      setLoading(false);
    }
  };

  const generateMockTimeSeriesData = (days: number, min: number, max: number) => {
    const data = [];
    const now = Date.now();
    for (let i = days; i >= 0; i--) {
      const date = new Date(now - i * 86400000);
      data.push({
        date: date.toISOString().split('T')[0],
        amount: Math.random() * (max - min) + min,
      });
    }
    return data;
  };

  const generateMockDelegationData = (days: number) => {
    const data = [];
    const now = Date.now();
    let baseCount = 1000;
    let baseAmount = 5000000;

    for (let i = days; i >= 0; i--) {
      const date = new Date(now - i * 86400000);
      baseCount += Math.floor(Math.random() * 10) - 3;
      baseAmount += Math.floor(Math.random() * 100000) - 50000;

      data.push({
        date: date.toISOString().split('T')[0],
        count: Math.max(900, baseCount),
        amount: Math.max(4500000, baseAmount),
      });
    }
    return data;
  };

  const generateMockUptimeData = (days: number) => {
    const data = [];
    const now = Date.now();
    for (let i = days; i >= 0; i--) {
      const date = new Date(now - i * 86400000);
      data.push({
        date: date.toISOString().split('T')[0],
        percentage: 99 + Math.random() * 1,
      });
    }
    return data;
  };

  const formatNumber = (num: number) => {
    return num.toLocaleString(undefined, { maximumFractionDigits: 2 });
  };

  const formatCurrency = (num: number) => {
    return num.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  };

  if (loading || !analytics) {
    return (
      <Card className="glass-card p-8 text-center">
        <p className="text-muted-foreground">Loading analytics...</p>
      </Card>
    );
  }

  const chartConfig = {
    rewards: {
      label: "Rewards",
      color: "hsl(var(--primary))",
    },
    votingPower: {
      label: "Voting Power",
      color: "hsl(var(--accent))",
    },
    delegations: {
      label: "Delegations",
      color: "hsl(var(--chart-1))",
    },
    uptime: {
      label: "Uptime",
      color: "hsl(var(--chart-2))",
    },
  };

  return (
    <div className="space-y-6 animate-fade-in">
      {/* Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <h2 className="text-3xl font-bold">Analytics Dashboard</h2>
          <p className="text-muted-foreground">Comprehensive performance metrics</p>
        </div>
        <Select value={timeRange} onValueChange={(value) => setTimeRange(value as TimeRange)}>
          <SelectTrigger className="w-[180px]">
            <Calendar className="mr-2 h-4 w-4" />
            <SelectValue placeholder="Select timeframe" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="7d">Last 7 Days</SelectItem>
            <SelectItem value="30d">Last 30 Days</SelectItem>
            <SelectItem value="90d">Last 90 Days</SelectItem>
            <SelectItem value="1y">Last Year</SelectItem>
            <SelectItem value="all">All Time</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {/* Performance Overview */}
      <div className="grid md:grid-cols-3 lg:grid-cols-6 gap-4">
        <Card className="glass-card p-4">
          <div className="flex flex-col gap-2">
            <div className="flex items-center justify-between">
              <p className="text-xs text-muted-foreground">Total Rewards</p>
              <DollarSign className="h-4 w-4 text-green-500" />
            </div>
            <p className="text-2xl font-bold">{formatCurrency(analytics.performance.totalRewardsEarned)}</p>
            <div className="flex items-center gap-1 text-xs text-green-500">
              <TrendingUp className="h-3 w-3" />
              <span>+12.3%</span>
            </div>
          </div>
        </Card>

        <Card className="glass-card p-4">
          <div className="flex flex-col gap-2">
            <div className="flex items-center justify-between">
              <p className="text-xs text-muted-foreground">Daily Avg</p>
              <Activity className="h-4 w-4 text-blue-500" />
            </div>
            <p className="text-2xl font-bold">{formatCurrency(analytics.performance.averageDailyRewards)}</p>
            <p className="text-xs text-muted-foreground">OMNI/day</p>
          </div>
        </Card>

        <Card className="glass-card p-4">
          <div className="flex flex-col gap-2">
            <div className="flex items-center justify-between">
              <p className="text-xs text-muted-foreground">Voting Power</p>
              <Award className="h-4 w-4 text-purple-500" />
            </div>
            <p className="text-2xl font-bold">{formatNumber(analytics.performance.currentVotingPower)}</p>
            <p className="text-xs text-muted-foreground">Rank #{analytics.performance.rank}</p>
          </div>
        </Card>

        <Card className="glass-card p-4">
          <div className="flex flex-col gap-2">
            <div className="flex items-center justify-between">
              <p className="text-xs text-muted-foreground">Uptime</p>
              <TrendingUp className="h-4 w-4 text-green-500" />
            </div>
            <p className="text-2xl font-bold">{analytics.performance.uptimePercentage.toFixed(2)}%</p>
            <div className="flex items-center gap-1 text-xs text-green-500">
              <span>Excellent</span>
            </div>
          </div>
        </Card>

        <Card className="glass-card p-4">
          <div className="flex flex-col gap-2">
            <div className="flex items-center justify-between">
              <p className="text-xs text-muted-foreground">Delegators</p>
              <Users className="h-4 w-4 text-accent" />
            </div>
            <p className="text-2xl font-bold">{formatNumber(analytics.performance.totalDelegators)}</p>
            <div className="flex items-center gap-1 text-xs text-green-500">
              <TrendingUp className="h-3 w-3" />
              <span>+5.2%</span>
            </div>
          </div>
        </Card>

        <Card className="glass-card p-4">
          <div className="flex flex-col gap-2">
            <div className="flex items-center justify-between">
              <p className="text-xs text-muted-foreground">Network Rank</p>
              <Award className="h-4 w-4 text-yellow-500" />
            </div>
            <p className="text-2xl font-bold">#{analytics.performance.rank}</p>
            <p className="text-xs text-muted-foreground">Top 1%</p>
          </div>
        </Card>
      </div>

      {/* Charts Grid */}
      <div className="grid md:grid-cols-2 gap-6">
        {/* Rewards Chart */}
        <Card className="glass-card p-6">
          <div className="space-y-4">
            <div>
              <h3 className="text-lg font-semibold">Rewards Over Time</h3>
              <p className="text-sm text-muted-foreground">Daily rewards earned (OMNI)</p>
            </div>
            <ChartContainer config={chartConfig} className="h-[300px]">
              <AreaChart data={analytics.rewards}>
                <defs>
                  <linearGradient id="rewardsGradient" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="hsl(var(--primary))" stopOpacity={0.8} />
                    <stop offset="95%" stopColor="hsl(var(--primary))" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" opacity={0.1} />
                <XAxis
                  dataKey="date"
                  tickFormatter={(value) => new Date(value).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })}
                />
                <YAxis />
                <ChartTooltip content={<ChartTooltipContent />} />
                <Area
                  type="monotone"
                  dataKey="amount"
                  stroke="hsl(var(--primary))"
                  fillOpacity={1}
                  fill="url(#rewardsGradient)"
                />
              </AreaChart>
            </ChartContainer>
          </div>
        </Card>

        {/* Voting Power Chart */}
        <Card className="glass-card p-6">
          <div className="space-y-4">
            <div>
              <h3 className="text-lg font-semibold">Voting Power Trend</h3>
              <p className="text-sm text-muted-foreground">Historical voting power</p>
            </div>
            <ChartContainer config={chartConfig} className="h-[300px]">
              <LineChart data={analytics.votingPower}>
                <CartesianGrid strokeDasharray="3 3" opacity={0.1} />
                <XAxis
                  dataKey="date"
                  tickFormatter={(value) => new Date(value).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })}
                />
                <YAxis />
                <ChartTooltip content={<ChartTooltipContent />} />
                <Line
                  type="monotone"
                  dataKey="amount"
                  stroke="hsl(var(--accent))"
                  strokeWidth={2}
                  dot={false}
                />
              </LineChart>
            </ChartContainer>
          </div>
        </Card>

        {/* Delegations Chart */}
        <Card className="glass-card p-6">
          <div className="space-y-4">
            <div>
              <h3 className="text-lg font-semibold">Delegator Growth</h3>
              <p className="text-sm text-muted-foreground">Number of delegators over time</p>
            </div>
            <ChartContainer config={chartConfig} className="h-[300px]">
              <BarChart data={analytics.delegations}>
                <CartesianGrid strokeDasharray="3 3" opacity={0.1} />
                <XAxis
                  dataKey="date"
                  tickFormatter={(value) => new Date(value).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })}
                />
                <YAxis />
                <ChartTooltip content={<ChartTooltipContent />} />
                <Bar dataKey="count" fill="hsl(var(--chart-1))" radius={[4, 4, 0, 0]} />
              </BarChart>
            </ChartContainer>
          </div>
        </Card>

        {/* Uptime Chart */}
        <Card className="glass-card p-6">
          <div className="space-y-4">
            <div>
              <h3 className="text-lg font-semibold">Uptime History</h3>
              <p className="text-sm text-muted-foreground">Daily uptime percentage</p>
            </div>
            <ChartContainer config={chartConfig} className="h-[300px]">
              <AreaChart data={analytics.uptime}>
                <defs>
                  <linearGradient id="uptimeGradient" x1="0" y1="0" x2="0" y2="1">
                    <stop offset="5%" stopColor="hsl(var(--chart-2))" stopOpacity={0.8} />
                    <stop offset="95%" stopColor="hsl(var(--chart-2))" stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray="3 3" opacity={0.1} />
                <XAxis
                  dataKey="date"
                  tickFormatter={(value) => new Date(value).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })}
                />
                <YAxis domain={[98, 100]} />
                <ChartTooltip content={<ChartTooltipContent />} />
                <Area
                  type="monotone"
                  dataKey="percentage"
                  stroke="hsl(var(--chart-2))"
                  fillOpacity={1}
                  fill="url(#uptimeGradient)"
                />
              </AreaChart>
            </ChartContainer>
          </div>
        </Card>
      </div>

      {/* Performance Insights */}
      <div className="grid md:grid-cols-2 gap-6">
        <Card className="glass-card p-6 space-y-4">
          <h3 className="text-lg font-semibold flex items-center gap-2">
            <TrendingUp className="h-5 w-5 text-green-500" />
            Performance Highlights
          </h3>
          <ul className="space-y-3 text-sm">
            <li className="flex items-start gap-2">
              <span className="text-green-500">✓</span>
              <span>Consistently earning above-average rewards (+12.3% vs network average)</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-green-500">✓</span>
              <span>Uptime exceeds 99.5% threshold for optimal rewards</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-green-500">✓</span>
              <span>Delegator count growing steadily month-over-month</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-green-500">✓</span>
              <span>Voting power increased by 15% in the selected period</span>
            </li>
          </ul>
        </Card>

        <Card className="glass-card p-6 space-y-4">
          <h3 className="text-lg font-semibold flex items-center gap-2">
            <Activity className="h-5 w-5 text-blue-500" />
            Recommendations
          </h3>
          <ul className="space-y-3 text-sm text-muted-foreground">
            <li className="flex items-start gap-2">
              <span className="text-blue-500">→</span>
              <span>Consider increasing self-delegation to improve ranking</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-blue-500">→</span>
              <span>Monitor commission rate - competitors average 8-10%</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-blue-500">→</span>
              <span>Update validator metadata to attract more delegators</span>
            </li>
            <li className="flex items-start gap-2">
              <span className="text-blue-500">→</span>
              <span>Engage with community through social media and forums</span>
            </li>
          </ul>
        </Card>
      </div>
    </div>
  );
};
