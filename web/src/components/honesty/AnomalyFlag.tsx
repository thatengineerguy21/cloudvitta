import React from 'react';
import { cn } from '../../lib/utils';
import { AlertTriangle } from 'lucide-react';

export interface AnomalyFlagProps extends React.HTMLAttributes<HTMLSpanElement> {
  reason?: string;
}

export const AnomalyFlag: React.FC<AnomalyFlagProps> = ({
  reason = 'Price observation under review: An unusual rate change was detected from this provider.',
  className,
  ...props
}) => {
  return (
    <span
      className={cn(
        'inline-flex items-center space-x-1 px-1.5 py-0.5 text-[11px] font-bold uppercase tracking-wider',
        'border border-status-anomaly text-status-anomaly bg-status-anomaly/10 cursor-help select-none',
        className
      )}
      title={reason}
      {...props}
    >
      <AlertTriangle className="w-3 h-3 shrink-0" aria-hidden="true" />
      <span>Under Review</span>
    </span>
  );
};
