import React, { useMemo } from 'react';
import { formatProviderName } from '../../lib/format';
import { PieChart } from 'lucide-react';
import type { ComparisonResultRow } from './CompareTemplate';

export interface BentoMetricHighlightsProps {
  results: ComparisonResultRow[];
  currency?: string;
  timeframe?: 'hourly' | 'monthly';
}

export const BentoMetricHighlights: React.FC<BentoMetricHighlightsProps> = ({
  results,
  currency = 'USD',
  timeframe = 'hourly',
}) => {
  const metrics = useMemo(() => {
    if (!results || results.length === 0) return null;

    const validPrices = results
      .map((r) => {
        const hourly = r.normalized_hourly_usd ?? (r.price?.amount ? Number(r.price.amount) : undefined);
        return {
          row: r,
          hourly: hourly !== undefined && !Number.isNaN(hourly) ? hourly : 0,
        };
      })
      .filter((p) => p.hourly > 0)
      .sort((a, b) => a.hourly - b.hourly);

    if (validPrices.length === 0) return null;

    const floor = validPrices[0];
    const ceiling = validPrices[validPrices.length - 1];

    // Median
    const mid = Math.floor(validPrices.length / 2);
    const median =
      validPrices.length % 2 !== 0
        ? validPrices[mid].hourly
        : (validPrices[mid - 1].hourly + validPrices[mid].hourly) / 2;

    // Annual Max Spread: (ceiling - floor) * 730 * 12
    const annualSpread = (ceiling.hourly - floor.hourly) * 730 * 12;

    // Architecture or Provider distribution
    const archCounts: Record<string, number> = {};
    results.forEach((r) => {
      const arch = (r.architecture || (r.matched_spec?.cpu_architecture as string) || (r.spec_summary?.includes('ARM') ? 'ARM' : 'x86_64') || 'x86').toUpperCase();
      archCounts[arch] = (archCounts[arch] || 0) + 1;
    });

    const totalArch = results.length;
    const archDist = Object.entries(archCounts).map(([name, count]) => ({
      name,
      pct: Math.round((count / totalArch) * 100),
    }));

    return {
      floor,
      ceiling,
      median,
      annualSpread,
      archDist,
    };
  }, [results]);

  if (!metrics) return null;

  const mult = timeframe === 'monthly' ? 730 : 1;
  const unitLabel = timeframe === 'monthly' ? '/mo' : '/hr';
  const currSymbol = currency === 'USD' ? '$' : `${currency} `;

  const formatCost = (val: number): string => {
    const adjusted = val * mult;
    if (adjusted >= 100) {
      return adjusted.toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 2 });
    }
    return adjusted.toLocaleString(undefined, { minimumFractionDigits: 4, maximumFractionDigits: 4 });
  };

  const floorRow = metrics.floor.row;
  const floorName = `${formatProviderName(floorRow.provider)} ${floorRow.instance_type || floorRow.sku_id || ''}`.trim();

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 w-full">
      {/* 1. Arbitrage Floor */}
      <div className="bg-surface-card rounded-2xl border border-border-default p-5 shadow-sm">
        <div className="text-[11px] font-semibold text-text-secondary uppercase tracking-wider">
          Arbitrage Floor
        </div>
        <div className="mt-2 flex items-baseline gap-1.5">
          <span className="text-2xl sm:text-3xl font-extrabold text-status-matchExact tracking-tight font-mono tabular-nums">
            {currSymbol}{formatCost(metrics.floor.hourly)}
          </span>
          <span className="text-xs font-medium text-text-secondary">{unitLabel}</span>
        </div>
        <p className="text-xs text-text-secondary mt-2 truncate" title={floorName}>
          {floorName}
        </p>
      </div>

      {/* 2. Median Market Price */}
      <div className="bg-surface-card rounded-2xl border border-border-default p-5 shadow-sm">
        <div className="text-[11px] font-semibold text-text-secondary uppercase tracking-wider">
          Median Market Price
        </div>
        <div className="mt-2 flex items-baseline gap-1.5">
          <span className="text-2xl sm:text-3xl font-extrabold text-text-primary tracking-tight font-mono tabular-nums">
            {currSymbol}{formatCost(metrics.median)}
          </span>
          <span className="text-xs font-medium text-text-secondary">{unitLabel}</span>
        </div>
        <p className="text-xs text-text-secondary mt-2">
          Baseline index across {results.length} matched options
        </p>
      </div>

      {/* 3. Annual Max Spread */}
      <div className="bg-surface-card rounded-2xl border border-border-default p-5 shadow-sm">
        <div className="text-[11px] font-semibold text-text-secondary uppercase tracking-wider">
          Annual Max Spread
        </div>
        <div className="mt-2 flex items-baseline gap-1.5">
          <span className="text-2xl sm:text-3xl font-extrabold text-brand-500 tracking-tight font-mono tabular-nums">
            -{currSymbol}{metrics.annualSpread.toLocaleString(undefined, { maximumFractionDigits: 0 })}
          </span>
          <span className="text-xs font-medium text-text-secondary">/yr node</span>
        </div>
        <p className="text-xs text-text-secondary mt-2">
          {formatProviderName(metrics.floor.row.provider)} vs {formatProviderName(metrics.ceiling.row.provider)} spread
        </p>
      </div>

      {/* 4. Architecture Breakdown */}
      <div className="bg-surface-card rounded-2xl border border-border-default p-5 shadow-sm flex flex-col justify-between">
        <div className="flex items-center justify-between text-[11px] font-semibold text-text-secondary uppercase tracking-wider">
          <span>Architecture Spread</span>
          <PieChart className="w-3.5 h-3.5 text-text-secondary" aria-hidden="true" />
        </div>
        <div className="mt-3">
          <div className="w-full h-2.5 rounded-full overflow-hidden flex bg-surface-raised">
            {metrics.archDist.map((item, idx) => {
              const colors = ['bg-status-matchExact', 'bg-status-matchClose', 'bg-status-matchApproximate', 'bg-brand-500'];
              return (
                <div
                  key={item.name}
                  className={`h-full ${colors[idx % colors.length]}`}
                  style={{ width: `${item.pct}%` }}
                  title={`${item.name}: ${item.pct}%`}
                />
              );
            })}
          </div>
          <div className="flex items-center justify-between text-[11px] font-semibold text-text-secondary mt-2.5">
            {metrics.archDist.slice(0, 3).map((item) => (
              <span key={item.name}>
                {item.pct}% {item.name}
              </span>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
};
