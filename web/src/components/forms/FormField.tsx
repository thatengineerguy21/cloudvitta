import React from 'react';
import { cn } from '../../lib/utils';

export interface FormFieldProps {
  label?: string;
  htmlFor?: string;
  error?: string;
  helperText?: string;
  children: React.ReactNode;
  className?: string;
}

export const FormField: React.FC<FormFieldProps> = ({
  label,
  htmlFor,
  error,
  helperText,
  children,
  className,
}) => {
  return (
    <div className={cn('flex flex-col space-y-1.5 w-full', className)}>
      {label && (
        <label
          htmlFor={htmlFor}
          className="text-xs uppercase font-bold tracking-widest text-text-secondary"
        >
          {label}
        </label>
      )}
      {children}
      {error && <span className="text-xs text-status-anomaly font-medium">{error}</span>}
      {!error && helperText && (
        <span className="text-xs text-text-secondary font-normal">{helperText}</span>
      )}
    </div>
  );
};
