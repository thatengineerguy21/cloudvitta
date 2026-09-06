import React from 'react';
import { cn } from '../../lib/utils';

// Constrained strictly to the 3 UI match quality tiers per PRD §10.2 & STAGE-5-FRONTEND.md §3.3.
// ('none' is an internal state surfaced exclusively via WarningsBanner with code 'no_match').
export type UIMatchQuality = 'exact' | 'close' | 'approximate';

export interface MatchQualityBadgeProps extends React.HTMLAttributes<HTMLSpanElement> {
  quality: UIMatchQuality;
  score?: number;
  deltaPct?: number;
}

export const MatchQualityBadge: React.FC<MatchQualityBadgeProps> = ({
  quality,
  score,
  deltaPct,
  className,
  ...props
}) => {
  const styles: Record<UIMatchQuality, string> = {
    exact: 'border-status-matchExact text-status-matchExact bg-status-matchExact/10',
    close: 'border-status-matchClose text-status-matchClose bg-status-matchClose/10',
    approximate: 'border-status-matchApproximate text-status-matchApproximate bg-status-matchApproximate/10',
  };

  const labels: Record<UIMatchQuality, string> = {
    exact: 'Exact Match',
    close: 'Close Match',
    approximate: 'Approximate',
  };

  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 px-2.5 py-0.5 text-xs font-semibold rounded-full border select-none',
        styles[quality],
        className
      )}
      title={
        score !== undefined
          ? `Match Quality: ${labels[quality]} (Score: ${score.toFixed(2)}${
              deltaPct !== undefined ? `, Delta: ${deltaPct}%` : ''
            })`
          : `Match Quality: ${labels[quality]}`
      }
      {...props}
    >
      <span className="w-1.5 h-1.5 mr-1.5 bg-current inline-block" aria-hidden="true" />
      <span>{labels[quality]}</span>
      {deltaPct !== undefined && deltaPct > 0 && (
        <span className="ml-1 opacity-75 text-[10px]">+{deltaPct}%</span>
      )}
    </span>
  );
};
