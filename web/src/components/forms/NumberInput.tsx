import React from 'react';
import { TextInput, TextInputProps } from './TextInput';

export interface NumberInputProps extends Omit<TextInputProps, 'type' | 'onChange' | 'value'> {
  value?: number | string;
  onChange?: (value: number | undefined) => void;
  min?: number;
  max?: number;
  step?: number;
}

export const NumberInput: React.FC<NumberInputProps> = ({
  value,
  onChange,
  min,
  max,
  step = 1,
  ...props
}) => {
  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const raw = e.target.value;
    if (raw === '') {
      onChange?.(undefined);
      return;
    }
    const parsed = Number(raw);
    if (!Number.isNaN(parsed)) {
      onChange?.(parsed);
    }
  };

  return (
    <TextInput
      type="number"
      {...(value !== undefined ? { value } : {})}
      onChange={handleChange}
      min={min}
      max={max}
      step={step}
      {...props}
    />
  );
};
