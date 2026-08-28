import React from 'react';
import { cn } from '../../lib/utils';

export interface BentoCardProps extends React.HTMLAttributes<HTMLDivElement> {
  children: React.ReactNode;
  colSpan?: 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 | 9 | 10 | 11 | 12;
  header?: React.ReactNode;
  footer?: React.ReactNode;
  isHoverable?: boolean;
}

export const BentoCard: React.FC<BentoCardProps> = ({
  children,
  colSpan = 12,
  header,
  footer,
  isHoverable = false,
  className,
  ...props
}) => {
  const spanClasses: Record<number, string> = {
    1: 'col-span-1',
    2: 'col-span-1 md:col-span-2',
    3: 'col-span-1 md:col-span-3',
    4: 'col-span-1 md:col-span-4',
    5: 'col-span-1 md:col-span-5',
    6: 'col-span-1 md:col-span-6',
    7: 'col-span-1 md:col-span-6 lg:col-span-7',
    8: 'col-span-1 md:col-span-6 lg:col-span-8',
    9: 'col-span-1 md:col-span-6 lg:col-span-9',
    10: 'col-span-1 md:col-span-6 lg:col-span-10',
    11: 'col-span-1 md:col-span-6 lg:col-span-11',
    12: 'col-span-1 md:col-span-6 lg:col-span-12',
  };

  return (
    <div
      className={cn(
        'bg-surface-card border border-border-default flex flex-col p-6 sm:p-8 transition-colors',
        isHoverable && 'hover:border-border-accent',
        spanClasses[colSpan],
        className
      )}
      {...props}
    >
      {header && <div className="border-b border-border-default pb-4 mb-6">{header}</div>}
      <div className="flex-1">{children}</div>
      {footer && <div className="border-t border-border-default pt-4 mt-6">{footer}</div>}
    </div>
  );
};
