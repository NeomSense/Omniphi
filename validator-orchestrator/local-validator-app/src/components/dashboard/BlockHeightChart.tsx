import { useState, useEffect } from 'react';
import { Card } from '../ui/Card';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, AreaChart, Area } from 'recharts';

interface BlockHeightChartProps {
  currentHeight: number;
}

interface ChartDataPoint {
  time: string;
  height: number;
  blocksPerMinute: number;
}

export function BlockHeightChart({ currentHeight }: BlockHeightChartProps) {
  const [data, setData] = useState<ChartDataPoint[]>([]);
  const [timeRange, setTimeRange] = useState<'1h' | '6h' | '24h'>('1h');

  useEffect(() => {
    // Generate historical data based on current height
    const now = Date.now();
    const points = 60; // 60 data points
    const intervalMs = timeRange === '1h' ? 60000 : timeRange === '6h' ? 360000 : 1440000;
    const blocksPerInterval = timeRange === '1h' ? 10 : timeRange === '6h' ? 60 : 240;

    const historicalData: ChartDataPoint[] = Array.from({ length: points }, (_, i) => {
      const timestamp = now - (points - 1 - i) * intervalMs;
      const height = currentHeight - (points - 1 - i) * blocksPerInterval + Math.floor(Math.random() * 5);
      return {
        time: new Date(timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
        height: Math.max(0, height),
        blocksPerMinute: 8 + Math.random() * 4, // ~8-12 blocks per minute
      };
    });

    setData(historicalData);
  }, [currentHeight, timeRange]);

  // Add new data point every few seconds
  useEffect(() => {
    const interval = setInterval(() => {
      setData(prev => {
        if (prev.length === 0) return prev;
        const newPoint: ChartDataPoint = {
          time: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
          height: currentHeight,
          blocksPerMinute: 8 + Math.random() * 4,
        };
        return [...prev.slice(1), newPoint];
      });
    }, 5000);

    return () => clearInterval(interval);
  }, [currentHeight]);

  const avgBlockTime = data.length > 0
    ? (data[data.length - 1].height - data[0].height) / data.length
    : 0;

  return (
    <Card title="Block Height History">
      <div className="space-y-4">
        {/* Time Range Selector */}
        <div className="flex items-center justify-between">
          <div className="flex space-x-2">
            {(['1h', '6h', '24h'] as const).map(range => (
              <button
                key={range}
                onClick={() => setTimeRange(range)}
                className={`px-3 py-1 rounded text-sm font-medium transition-colors ${
                  timeRange === range
                    ? 'bg-purple-600 text-white'
                    : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
                }`}
              >
                {range}
              </button>
            ))}
          </div>
          <div className="text-sm text-gray-500">
            Avg: {avgBlockTime.toFixed(1)} blocks/interval
          </div>
        </div>

        {/* Chart */}
        <div className="h-64">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={data} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
              <defs>
                <linearGradient id="colorHeight" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#8B5CF6" stopOpacity={0.3} />
                  <stop offset="95%" stopColor="#8B5CF6" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" stroke="#E5E7EB" />
              <XAxis
                dataKey="time"
                tick={{ fontSize: 12, fill: '#6B7280' }}
                tickLine={false}
                axisLine={{ stroke: '#E5E7EB' }}
              />
              <YAxis
                tick={{ fontSize: 12, fill: '#6B7280' }}
                tickLine={false}
                axisLine={{ stroke: '#E5E7EB' }}
                tickFormatter={(value) => value.toLocaleString()}
              />
              <Tooltip
                contentStyle={{
                  backgroundColor: '#1F2937',
                  border: 'none',
                  borderRadius: '8px',
                  color: '#F3F4F6',
                }}
                labelStyle={{ color: '#9CA3AF' }}
                formatter={(value: number) => [value.toLocaleString(), 'Block Height']}
              />
              <Area
                type="monotone"
                dataKey="height"
                stroke="#8B5CF6"
                strokeWidth={2}
                fillOpacity={1}
                fill="url(#colorHeight)"
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>

        {/* Stats */}
        <div className="grid grid-cols-3 gap-4 pt-4 border-t border-gray-100">
          <div className="text-center">
            <p className="text-xs text-gray-500">Current</p>
            <p className="text-lg font-bold text-gray-900">{currentHeight.toLocaleString()}</p>
          </div>
          <div className="text-center">
            <p className="text-xs text-gray-500">Start ({timeRange})</p>
            <p className="text-lg font-bold text-gray-900">
              {data.length > 0 ? data[0].height.toLocaleString() : '-'}
            </p>
          </div>
          <div className="text-center">
            <p className="text-xs text-gray-500">Produced</p>
            <p className="text-lg font-bold text-green-600">
              +{data.length > 0 ? (currentHeight - data[0].height).toLocaleString() : 0}
            </p>
          </div>
        </div>
      </div>
    </Card>
  );
}
