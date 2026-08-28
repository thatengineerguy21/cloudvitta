import React from 'react';
import { cn } from '../../lib/utils';

export interface TextInputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
  helperText?: string;
}

export const TextInput = React.forwardRef<HTMLInputElement, TextInputProps>(
  ({ label, error, helperText, className, id, ...props }, ref) => {
    const inputId = id || (label ? label.toLowerCase().replace(/\s+/g, '-') : undefined);

    return (
      <div className="flex flex-col space-y-1.5 w-full">
        {label && (
          <label
            htmlFor={inputId}
            className="text-xs uppercase font-bold tracking-widest text-text-secondary"
          >
            {label}
          </label>
        )}
        <input
          ref={ref}
          id={inputId}
          className={cn(
            'bg-surface-card border border-border-default text-text-primary px-3 py-2 text-sm',
            'focus:outline-none focus:border-border-accent transition-colors placeholder:text-text-secondary/50',
            error && 'border-status-anomaly text-status-anomaly',
            className
          )}
          {...props}
        />
        {error && <span className="text-xs text-status-anomaly font-medium">{error}</span>}
        {!error && helperText && (
          <span className="text-xs text-text-secondary font-normal">{helperText}</span>
        )}
      </div>
    );
  }
);
TextInput.displayName = 'TextInput';
