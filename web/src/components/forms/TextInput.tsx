import React from 'react';
import { cn } from '../../lib/utils';
import { FormField } from './FormField';

export interface TextInputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  label?: string;
  error?: string;
  helperText?: string;
}

export const TextInput = React.forwardRef<HTMLInputElement, TextInputProps>(
  ({ label, error, helperText, className, id, ...props }, ref) => {
    const inputId = id || (label ? label.toLowerCase().replace(/\s+/g, '-') : undefined);

    return (
      <FormField label={label} htmlFor={inputId} error={error} helperText={helperText}>
        <input
          ref={ref}
          id={inputId}
          className={cn(
            'bg-surface-raised border border-border-default/80 text-text-primary px-3 py-2 text-sm rounded-xl',
            'focus:outline-none focus:border-brand-500 focus:ring-2 focus:ring-brand-500/10 transition-all placeholder:text-text-secondary/50',
            error && 'border-status-anomaly text-status-anomaly focus:ring-status-anomaly/10',
            className
          )}
          {...props}
        />
      </FormField>
    );
  }
);
TextInput.displayName = 'TextInput';
