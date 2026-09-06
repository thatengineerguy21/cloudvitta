import React, { useState } from 'react';
import { cn } from '../../lib/utils';
import { HelpCircle } from 'lucide-react';

export interface MissingAttributesIndicatorProps extends React.HTMLAttributes<HTMLDivElement> {
  missingAttributes: string[];
}

export const MissingAttributesIndicator: React.FC<MissingAttributesIndicatorProps> = ({
  missingAttributes,
  className,
  ...props
}) => {
  const [isOpen, setIsOpen] = useState(false);

  if (!missingAttributes || missingAttributes.length === 0) {
    return null;
  }

  return (
    <div
      className={cn('relative inline-block', className)}
      onMouseEnter={() => setIsOpen(true)}
      onMouseLeave={() => setIsOpen(false)}
      {...props}
    >
      <button
        type="button"
        onClick={() => setIsOpen((prev) => !prev)}
        aria-expanded={isOpen}
        aria-haspopup="true"
        aria-controls="missing-attrs-tooltip"
        className="inline-flex items-center space-x-1 px-1.5 py-0.5 text-xs font-medium border border-border-default hover:border-border-accent text-text-secondary bg-surface-raised cursor-pointer"
        aria-label={`${missingAttributes.length} unspecified specifications`}
      >
        <HelpCircle className="w-3 h-3 text-border-accent shrink-0" aria-hidden="true" />
        <span>{missingAttributes.length} unspecified specs</span>
      </button>

      {isOpen && (
        <div
          id="missing-attrs-tooltip"
          role="tooltip"
          className="absolute z-50 left-0 bottom-full mb-1.5 w-56 p-3 bg-surface-card border border-border-default shadow-none"
        >
          <p className="text-xs font-bold uppercase tracking-wider text-text-primary mb-1">
            Unspecified Specifications
          </p>
          <p className="text-xs text-text-secondary mb-2">
            The matched cloud configuration does not declare the following requested parameters:
          </p>
          <ul className="list-disc list-inside text-xs text-text-primary space-y-0.5">
            {missingAttributes.map((attr) => (
              <li key={attr} className="font-mono text-[11px]">
                {attr}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
};
