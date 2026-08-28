// web/src/components/calculate/WorkloadCategoryCard.tsx
import React from 'react';
import { cn } from '../../lib/utils';
import { ChevronDown, ChevronUp, LucideIcon, Check } from 'lucide-react';

export interface WorkloadCategoryCardProps {
  title: string;
  icon: LucideIcon;
  enabled: boolean;
  onToggleEnabled: (enabled: boolean) => void;
  isOpen: boolean;
  onToggleOpen: () => void;
  summary: string;
  children: React.ReactNode;
  className?: string;
}

export const WorkloadCategoryCard: React.FC<WorkloadCategoryCardProps> = ({
  title,
  icon: Icon,
  enabled,
  onToggleEnabled,
  isOpen,
  onToggleOpen,
  summary,
  children,
  className,
}) => {
  return (
    <div
      className={cn(
        'border border-border-default bg-surface-card transition-colors',
        enabled && 'border-border-accent/60',
        className
      )}
    >
      {/* Accordion Header */}
      <div className="flex items-center justify-between p-3.5 bg-surface-raised/40 select-none border-b border-border-default/50">
        <div className="flex items-center space-x-3 min-w-0">
          {/* Custom Rectilinear Checkbox */}
          <label className="flex items-center space-x-2.5 cursor-pointer">
            <button
              type="button"
              role="checkbox"
              aria-checked={enabled}
              aria-label={`Enable ${title}`}
              onClick={(e) => {
                e.stopPropagation();
                onToggleEnabled(!enabled);
              }}
              className={cn(
                'w-4 h-4 border flex items-center justify-center transition-colors',
                enabled
                  ? 'bg-border-accent border-border-accent text-surface-canvas'
                  : 'bg-surface-card border-border-default hover:border-text-secondary'
              )}
            >
              {enabled && <Check className="w-3 h-3 stroke-[3]" />}
            </button>
            <div
              className="flex items-center space-x-2 cursor-pointer"
              onClick={onToggleOpen}
            >
              <Icon className={cn('w-4 h-4', enabled ? 'text-border-accent' : 'text-text-secondary')} />
              <span className={cn('text-sm font-medium truncate', enabled ? 'text-text-primary font-semibold' : 'text-text-secondary')}>
                {title}
              </span>
            </div>
          </label>
        </div>

        {/* Right side summary & toggle button */}
        <div className="flex items-center space-x-2.5 ml-2 shrink-0">
          <span
            onClick={onToggleOpen}
            className={cn(
              'text-[11px] px-2 py-0.5 border font-mono truncate max-w-[150px] sm:max-w-none cursor-pointer',
              enabled
                ? 'border-border-accent/40 text-text-primary bg-surface-card'
                : 'border-border-default text-text-secondary/70 bg-surface-raised'
            )}
          >
            {enabled ? summary : 'Disabled'}
          </span>
          <button
            type="button"
            onClick={onToggleOpen}
            aria-label={isOpen ? `Collapse ${title}` : `Expand ${title}`}
            className="p-1 hover:bg-surface-raised text-text-secondary hover:text-text-primary transition-colors"
          >
            {isOpen ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
          </button>
        </div>
      </div>

      {/* Accordion Body */}
      {isOpen && (
        <div className="p-4 bg-surface-card space-y-3.5">
          {!enabled && (
            <div className="p-2 border border-dashed border-border-default bg-surface-raised text-xs text-text-secondary flex items-center justify-between">
              <span>This category is currently disabled in your workload.</span>
              <button
                type="button"
                onClick={() => onToggleEnabled(true)}
                className="text-xs uppercase font-bold tracking-wider text-border-accent hover:underline ml-2"
              >
                Enable Category
              </button>
            </div>
          )}
          <div className={cn('grid grid-cols-1 sm:grid-cols-2 gap-3', !enabled && 'opacity-60')}>
            {children}
          </div>
        </div>
      )}
    </div>
  );
};
