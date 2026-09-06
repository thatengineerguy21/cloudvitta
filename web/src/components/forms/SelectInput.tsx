import React from 'react';
import { cn } from '../../lib/utils';
import { ChevronDown } from 'lucide-react';
import { FormField } from './FormField';

export interface SelectOption {
  value: string;
  label: string;
  disabled?: boolean;
}

export interface SelectInputProps extends React.SelectHTMLAttributes<HTMLSelectElement> {
  label?: string;
  options: SelectOption[];
  error?: string;
  helperText?: string;
}

export const SelectInput = React.forwardRef<HTMLSelectElement, SelectInputProps>(
  ({ label, options, error, helperText, className, id, ...props }, ref) => {
    const selectId = id || (label ? label.toLowerCase().replace(/\s+/g, '-') : undefined);

    return (
      <FormField label={label} htmlFor={selectId} error={error} helperText={helperText}>
        <div className="relative">
          <select
            ref={ref}
            id={selectId}
            className={cn(
              'w-full appearance-none bg-stone-50/80 dark:bg-surface-raised border border-border-default/80 text-text-primary px-3 py-2 pr-8 text-sm rounded-xl cursor-pointer',
              'focus:outline-none focus:border-brand-500 focus:ring-2 focus:ring-brand-500/10 transition-all',
              error && 'border-status-anomaly text-status-anomaly focus:ring-status-anomaly/10',
              className
            )}
            {...props}
          >
            {options.map((opt) => (
              <option
                key={opt.value}
                value={opt.value}
                disabled={opt.disabled}
                className="bg-surface-card text-text-primary"
              >
                {opt.label}
              </option>
            ))}
          </select>
          <ChevronDown
            className="w-4 h-4 text-text-secondary absolute right-2.5 top-1/2 -translate-y-1/2 pointer-events-none"
            aria-hidden="true"
          />
        </div>
      </FormField>
    );
  }
);
SelectInput.displayName = 'SelectInput';
