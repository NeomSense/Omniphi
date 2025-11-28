import { Card } from '../ui/Card';
import { PoCScore } from '../../types/validator';
import { clsx } from 'clsx';

interface PoCScoreCardProps {
  poc: PoCScore | null;
  loading?: boolean;
}

const tierColors = {
  Bronze: { bg: 'bg-amber-100', text: 'text-amber-800', border: 'border-amber-300', ring: 'ring-amber-500' },
  Silver: { bg: 'bg-gray-100', text: 'text-gray-700', border: 'border-gray-300', ring: 'ring-gray-400' },
  Gold: { bg: 'bg-yellow-100', text: 'text-yellow-800', border: 'border-yellow-400', ring: 'ring-yellow-500' },
  Platinum: { bg: 'bg-purple-100', text: 'text-purple-800', border: 'border-purple-400', ring: 'ring-purple-500' },
};

export function PoCScoreCard({ poc, loading }: PoCScoreCardProps) {
  if (loading) {
    return (
      <Card title="PoC Reputation">
        <div className="animate-pulse space-y-4">
          <div className="h-20 bg-gray-200 rounded-full w-20 mx-auto"></div>
          <div className="h-4 bg-gray-200 rounded w-1/2 mx-auto"></div>
        </div>
      </Card>
    );
  }

  if (!poc) {
    return (
      <Card title="PoC Reputation">
        <p className="text-gray-500 text-sm text-center">No PoC data available</p>
      </Card>
    );
  }

  const colors = tierColors[poc.tier];

  const getScoreColor = (score: number) => {
    if (score >= 80) return 'text-green-600';
    if (score >= 60) return 'text-yellow-600';
    if (score >= 40) return 'text-orange-600';
    return 'text-red-600';
  };

  const getProgressColor = (score: number) => {
    if (score >= 80) return 'bg-green-500';
    if (score >= 60) return 'bg-yellow-500';
    if (score >= 40) return 'bg-orange-500';
    return 'bg-red-500';
  };

  return (
    <Card title="PoC Reputation Score">
      <div className="text-center">
        {/* Main Score Circle */}
        <div className="relative inline-flex items-center justify-center mb-4">
          <div className={clsx(
            'w-28 h-28 rounded-full flex items-center justify-center',
            'ring-4',
            colors.ring,
            colors.bg
          )}>
            <div className="text-center">
              <span className={clsx('text-3xl font-bold', getScoreColor(poc.total_score))}>
                {poc.total_score}
              </span>
              <span className="text-gray-500 text-sm block">/ 100</span>
            </div>
          </div>
        </div>

        {/* Tier Badge */}
        <div className={clsx(
          'inline-flex items-center px-4 py-1.5 rounded-full text-sm font-semibold mb-4',
          colors.bg,
          colors.text,
          colors.border,
          'border'
        )}>
          {poc.tier} Tier
          {poc.rank && <span className="ml-2 opacity-75">#{poc.rank}</span>}
        </div>

        {/* Score Breakdown */}
        <div className="space-y-3 mt-4">
          {/* Reliability */}
          <div>
            <div className="flex justify-between text-sm mb-1">
              <span className="text-gray-600">Reliability</span>
              <span className="font-medium text-gray-900">{poc.reliability}%</span>
            </div>
            <div className="w-full bg-gray-200 rounded-full h-2">
              <div
                className={clsx('h-2 rounded-full transition-all duration-500', getProgressColor(poc.reliability))}
                style={{ width: `${poc.reliability}%` }}
              />
            </div>
          </div>

          {/* Contributions */}
          <div>
            <div className="flex justify-between text-sm mb-1">
              <span className="text-gray-600">Contributions</span>
              <span className="font-medium text-gray-900">{poc.contributions}%</span>
            </div>
            <div className="w-full bg-gray-200 rounded-full h-2">
              <div
                className={clsx('h-2 rounded-full transition-all duration-500', getProgressColor(poc.contributions))}
                style={{ width: `${poc.contributions}%` }}
              />
            </div>
          </div>

          {/* Governance */}
          <div>
            <div className="flex justify-between text-sm mb-1">
              <span className="text-gray-600">Governance</span>
              <span className="font-medium text-gray-900">{poc.governance}%</span>
            </div>
            <div className="w-full bg-gray-200 rounded-full h-2">
              <div
                className={clsx('h-2 rounded-full transition-all duration-500', getProgressColor(poc.governance))}
                style={{ width: `${poc.governance}%` }}
              />
            </div>
          </div>
        </div>

        {/* History Preview */}
        {poc.history && poc.history.length > 0 && (
          <div className="mt-4 pt-4 border-t border-gray-100">
            <p className="text-xs text-gray-500 mb-2">Recent Changes</p>
            <div className="flex justify-center space-x-2">
              {poc.history.slice(0, 5).map((entry, idx) => (
                <div
                  key={idx}
                  className={clsx(
                    'px-2 py-1 rounded text-xs font-medium',
                    entry.change > 0 ? 'bg-green-100 text-green-700' : 'bg-red-100 text-red-700'
                  )}
                >
                  {entry.change > 0 ? '+' : ''}{entry.change}
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </Card>
  );
}
