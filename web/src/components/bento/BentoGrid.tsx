import React from 'react';
import { cn } from '../../lib/utils';

export interface BentoGridProps extends React.HTMLAttributes<HTMLDivElement> {
  children: React.ReactNode;
  columns?: 12 | 6 | 4 | 3;
  gap?: 'none' | 'sm' | 'md' | 'lg';
}

export const BentoGrid: React.FC<BentoGridProps> = ({
  children,
  columns = 12,
  gap = 'md',
  className,
  ...props
}) => {
  const columnClasses: Record<number, string> = {
    12: 'grid-cols-1 md:grid-cols-6 lg:grid-cols-12',
    6: 'grid-cols-1 md:grid-cols-3 lg:grid-cols-6',
    4: 'grid-cols-1 md:grid-cols-2 lg:grid-cols-4',
    3: 'grid-cols-1 md:grid-cols-3',
  };

  const gapClasses: Record<string, string> = {
    none: 'gap-0',
    sm: 'gap-2',
    md: 'gap-4',
    lg: 'gap-6',
  };

  return (
    <div
      className={cn('grid w-full', columnClasses[columns], gapClasses[gap], className)}
      {...props}
    >
      {children}
    </div>
  );
};
