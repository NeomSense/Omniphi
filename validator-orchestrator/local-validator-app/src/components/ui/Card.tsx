import { clsx } from 'clsx';

interface CardProps {
  children: React.ReactNode;
  title?: string;
  subtitle?: string;
  className?: string;
  dark?: boolean;
  actions?: React.ReactNode;
}

export function Card({ children, title, subtitle, className, dark, actions }: CardProps) {
  return (
    <div className={clsx(dark ? 'card-dark' : 'card', className)}>
      {(title || actions) && (
        <div className="flex items-center justify-between mb-4">
          <div>
            {title && (
              <h3 className={clsx(
                'text-lg font-semibold',
                dark ? 'text-white' : 'text-gray-900'
              )}>
                {title}
              </h3>
            )}
            {subtitle && (
              <p className={clsx(
                'text-sm mt-1',
                dark ? 'text-gray-300' : 'text-gray-500'
              )}>
                {subtitle}
              </p>
            )}
          </div>
          {actions && <div>{actions}</div>}
        </div>
      )}
      {children}
    </div>
  );
}
