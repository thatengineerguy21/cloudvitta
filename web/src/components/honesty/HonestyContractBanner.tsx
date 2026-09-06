import React from 'react';
import { cn } from '../../lib/utils';
import { ShieldCheck, FileText } from 'lucide-react';

export interface HonestyContractBannerProps extends React.HTMLAttributes<HTMLDivElement> {
  auditHref?: string;
}

export const HonestyContractBanner: React.FC<HonestyContractBannerProps> = ({
  auditHref = '/docs/',
  className,
  ...props
}) => {
  return (
    <div
      className={cn(
        'bg-emerald-50/90 dark:bg-surface-card border border-emerald-200 dark:border-status-matchExact/30 rounded-2xl p-4 flex flex-col md:flex-row md:items-center justify-between gap-3 shadow-sm relative overflow-hidden',
        className
      )}
      {...props}
    >
      <div className="flex items-start md:items-center gap-3">
        <div className="p-2 bg-emerald-100/90 dark:bg-status-matchExact/15 text-emerald-700 dark:text-status-matchExact rounded-xl shrink-0">
          <ShieldCheck className="w-5 h-5" aria-hidden="true" />
        </div>
        <div>
          <div className="flex items-center gap-2">
            <span className="font-bold text-text-primary text-sm">Honesty Contract Active</span>
            <span className="text-[10px] font-bold uppercase tracking-wider px-1.5 py-0.5 rounded bg-emerald-200/80 dark:bg-status-matchExact/20 text-emerald-900 dark:text-status-matchExact border border-emerald-300 dark:border-status-matchExact/40">
              Deterministic
            </span>
          </div>
          <p className="text-xs text-text-secondary mt-0.5">
            Live catalog ingestion across 7 public clouds. Zero affiliate-driven biasing, zero synthetic extrapolations. Region egress taxes factored automatically.
          </p>
        </div>
      </div>
      <a
        href={auditHref}
        target="_blank"
        rel="noreferrer"
        className="inline-flex items-center gap-1.5 text-xs font-semibold text-emerald-800 dark:text-status-matchExact hover:underline underline-offset-4 whitespace-nowrap self-start md:self-auto shrink-0 transition-colors"
      >
        <FileText className="w-3.5 h-3.5" aria-hidden="true" />
        <span>Audit Pipeline Logs</span>
      </a>
    </div>
  );
};
