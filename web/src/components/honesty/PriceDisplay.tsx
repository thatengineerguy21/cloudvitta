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
  deltaBadge?: string;
  approxMonthly?: string | number;
}

export const PriceDisplay: React.FC<PriceDisplayProps> = ({
  amount,
  currency = 'USD',
  unit = '/hr',
  normalizedHourlyUSD,
  normalizedMonthlyUSD,
  hasAnomaly = false,
  isPartial = false,
  deltaBadge,
  approxMonthly,
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
      <div className="flex items-baseline justify-between gap-2 flex-wrap">
        <div className="flex items-baseline space-x-1.5 flex-wrap">
          <span className="font-mono tabular-nums text-xl font-bold text-text-primary">
            {currency} {formatValue(amount)}
          </span>
          <span className="text-xs text-text-secondary font-medium">{unit}</span>
          {hasAnomaly && <AnomalyFlag className="ml-1" />}
        </div>
        {deltaBadge && (
          <span className="px-2 py-0.5 rounded-full text-[10px] font-bold bg-amber-50 dark:bg-amber-950/40 text-amber-800 dark:text-amber-300 border border-amber-200 dark:border-amber-800/40">
            {deltaBadge}
          </span>
        )}
      </div>

      {approxMonthly !== undefined && (
        <div className="text-xs text-text-secondary">
          Approx. <strong className="text-text-primary font-semibold font-mono">${formatValue(approxMonthly)}</strong> / mo
        </div>
      )}

      {currency !== 'USD' && normalizedHourlyUSD !== undefined && (
        <span className="font-mono tabular-nums text-xs text-text-secondary">
          ≈ ${formatValue(normalizedHourlyUSD)} USD/hr
        </span>
      )}

      {currency !== 'USD' && normalizedHourlyUSD === undefined && normalizedMonthlyUSD !== undefined && (
        <span className="font-mono tabular-nums text-xs text-text-secondary">
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
