import React from 'react';
import { cn } from '../../lib/utils';
import { Clock } from 'lucide-react';
import { formatRelativeTime } from '../../lib/format';

export interface StaleDataBadgeProps extends React.HTMLAttributes<HTMLSpanElement> {
  fetchedAt?: string;
}

export const StaleDataBadge: React.FC<StaleDataBadgeProps> = ({
  fetchedAt,
  className,
  ...props
}) => {
  const relativeTime = formatRelativeTime(fetchedAt);

  return (
    <span
      className={cn(
        'inline-flex items-center space-x-1 px-1.5 py-0.5 text-xs font-bold uppercase tracking-wider',
        'border border-status-stale text-status-stale bg-status-stale/10 select-none',
        className
      )}
      title={fetchedAt ? `Pricing data fetched on ${fetchedAt} (>168 hours old)` : 'Pricing data is stale'}
      {...props}
    >
      <Clock className="w-3 h-3 shrink-0" aria-hidden="true" />
      <span>Stale ({relativeTime})</span>
    </span>
  );
};
