import React from 'react';
import { cn } from '../../lib/utils';
import { AnomalyFlag } from './AnomalyFlag';

export interface PriceDisplayProps extends React.HTMLAttributes<HTMLDivElement> {
  amount?: string | number;
  currency?: string;
  unit?: '/hr' | '/mo' | '/GB' | '/million';
  normalizedHourlyUSD?: string | number;
  normalizedMonthlyUSD?: string | number;
  hasAnomaly?: boolean;
  isPartial?: boolean;
}

export const PriceDisplay: React.FC<PriceDisplayProps> = ({
  amount,
  currency = 'USD',
  unit = '/hr',
  normalizedHourlyUSD,
  normalizedMonthlyUSD,
  hasAnomaly = false,
  isPartial = false,
  className,
  ...props
}) => {
  const formatValue = (val?: string | number): string => {
    if (val === undefined || val === null || val === '') return '—';
    const num = Number(val);
    if (Number.isNaN(num)) return String(val);
    return num.toLocaleString(undefined, {
      minimumFractionDigits: 4,
      maximumFractionDigits: 4,
    });
  };

  return (
    <div className={cn('flex flex-col space-y-1', className)} {...props}>
      <div className="flex items-baseline space-x-1.5 flex-wrap gap-y-1">
        <span className="font-mono text-xl font-bold text-text-primary">
          {currency} {formatValue(amount)}
        </span>
        <span className="text-xs text-text-secondary font-medium">{unit}</span>
        {hasAnomaly && <AnomalyFlag className="ml-1" />}
      </div>

      {currency !== 'USD' && normalizedHourlyUSD !== undefined && (
        <span className="font-mono text-xs text-text-secondary">
          ≈ ${formatValue(normalizedHourlyUSD)} USD/hr
        </span>
      )}

      {currency !== 'USD' && normalizedHourlyUSD === undefined && normalizedMonthlyUSD !== undefined && (
        <span className="font-mono text-xs text-text-secondary">
          ≈ ${formatValue(normalizedMonthlyUSD)} USD/mo
        </span>
      )}

      {isPartial && (
        <span className="text-[11px] text-status-partial font-bold uppercase tracking-wider">
          Partial Workload Total
        </span>
      )}
    </div>
  );
};
