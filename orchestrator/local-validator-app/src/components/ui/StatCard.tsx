import { Card } from './Card';
import { clsx } from 'clsx';

interface StatCardProps {
  label: string;
  value: string | number;
  subValue?: string;
  trend?: 'up' | 'down' | 'neutral';
  trendValue?: string;
  icon?: React.ReactNode;
  variant?: 'default' | 'gradient';
}

export function StatCard({
  label,
  value,
  subValue,
  trend,
  trendValue,
  icon,
  variant = 'default'
}: StatCardProps) {
  const isGradient = variant === 'gradient';

  return (
    <Card dark={isGradient} className="relative overflow-hidden">
      {icon && (
        <div className={clsx(
          'absolute top-4 right-4 opacity-20',
          isGradient ? 'text-white' : 'text-omniphi-500'
        )}>
          {icon}
        </div>
      )}

      <div className="relative">
        <p className={clsx(
          'text-sm font-medium mb-2',
          isGradient ? 'text-gray-200' : 'text-gray-500'
        )}>
          {label}
        </p>

        <p className={clsx(
          'text-3xl font-bold mb-1',
          isGradient ? 'text-white' : 'text-gray-900'
        )}>
          {value}
        </p>

        {subValue && (
          <p className={clsx(
            'text-sm',
            isGradient ? 'text-gray-300' : 'text-gray-600'
          )}>
            {subValue}
          </p>
        )}

        {trend && trendValue && (
          <div className="flex items-center mt-2 space-x-1">
            <span className={clsx(
              'text-sm font-medium',
              trend === 'up' && 'text-green-500',
              trend === 'down' && 'text-red-500',
              trend === 'neutral' && 'text-gray-500'
            )}>
              {trend === 'up' && '↑'}
              {trend === 'down' && '↓'}
              {trend === 'neutral' && '→'}
              {' '}{trendValue}
            </span>
          </div>
        )}
      </div>
    </Card>
  );
}
