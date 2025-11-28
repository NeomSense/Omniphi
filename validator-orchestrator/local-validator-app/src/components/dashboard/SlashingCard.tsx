import { Card } from '../ui/Card';
import { Badge } from '../ui/Badge';
import { SlashingInfo } from '../../types/validator';
import { clsx } from 'clsx';

interface SlashingCardProps {
  slashing: SlashingInfo | null;
  loading?: boolean;
}

export function SlashingCard({ slashing, loading }: SlashingCardProps) {
  if (loading) {
    return (
      <Card title="Slashing Protection">
        <div className="animate-pulse space-y-3">
          <div className="h-4 bg-gray-200 rounded w-3/4"></div>
          <div className="h-8 bg-gray-200 rounded"></div>
        </div>
      </Card>
    );
  }

  if (!slashing) {
    return (
      <Card title="Slashing Protection">
        <p className="text-gray-500 text-sm">No slashing data available</p>
      </Card>
    );
  }

  const getRiskColor = (risk: string) => {
    switch (risk) {
      case 'low': return { bg: 'bg-green-100', text: 'text-green-700', border: 'border-green-300' };
      case 'medium': return { bg: 'bg-yellow-100', text: 'text-yellow-700', border: 'border-yellow-300' };
      case 'high': return { bg: 'bg-red-100', text: 'text-red-700', border: 'border-red-300' };
      default: return { bg: 'bg-gray-100', text: 'text-gray-700', border: 'border-gray-300' };
    }
  };

  const riskColors = getRiskColor(slashing.slashing_risk);
  const missedPercent = slashing.missed_blocks_percent;

  return (
    <Card title="Slashing Protection">
      <div className="space-y-4">
        {/* Risk Level */}
        <div className={clsx(
          'rounded-lg p-4 border',
          riskColors.bg,
          riskColors.border
        )}>
          <div className="flex items-center justify-between">
            <div>
              <p className={clsx('text-sm font-medium', riskColors.text)}>Slashing Risk</p>
              <p className={clsx('text-2xl font-bold capitalize', riskColors.text)}>
                {slashing.slashing_risk}
              </p>
            </div>
            {slashing.tombstoned && (
              <Badge variant="error">TOMBSTONED</Badge>
            )}
          </div>
        </div>

        {/* Missed Blocks */}
        <div>
          <div className="flex justify-between text-sm mb-2">
            <span className="text-gray-600">Missed Blocks</span>
            <span className="font-medium text-gray-900">
              {slashing.missed_blocks_count} / {slashing.missed_blocks_window}
            </span>
          </div>
          <div className="w-full bg-gray-200 rounded-full h-3">
            <div
              className={clsx(
                'h-3 rounded-full transition-all duration-500',
                missedPercent < 5 ? 'bg-green-500' :
                missedPercent < 10 ? 'bg-yellow-500' : 'bg-red-500'
              )}
              style={{ width: `${Math.min(missedPercent, 100)}%` }}
            />
          </div>
          <p className="text-xs text-gray-500 mt-1">
            {missedPercent.toFixed(2)}% of window
          </p>
        </div>

        {/* Double Sign Protection */}
        <div className="flex items-center justify-between pt-3 border-t border-gray-100">
          <div>
            <p className="text-sm text-gray-600">Double Sign Protection</p>
            {slashing.last_double_sign_check && (
              <p className="text-xs text-gray-400">
                Last check: {new Date(slashing.last_double_sign_check).toLocaleTimeString()}
              </p>
            )}
          </div>
          <div className={clsx(
            'px-3 py-1 rounded-full text-sm font-medium',
            slashing.double_sign_protection
              ? 'bg-green-100 text-green-700'
              : 'bg-red-100 text-red-700'
          )}>
            {slashing.double_sign_protection ? 'Active' : 'Inactive'}
          </div>
        </div>

        {/* Warning if high risk */}
        {slashing.slashing_risk === 'high' && !slashing.tombstoned && (
          <div className="bg-red-50 border border-red-200 rounded-lg p-3">
            <p className="text-sm text-red-700 font-medium">Warning: High Slashing Risk</p>
            <p className="text-xs text-red-600 mt-1">
              Your validator is at risk of being slashed. Check your node connectivity and uptime.
            </p>
          </div>
        )}
      </div>
    </Card>
  );
}
