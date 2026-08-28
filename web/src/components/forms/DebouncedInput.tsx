import React, { useEffect, useRef, useState } from 'react';
import { TextInput, TextInputProps } from './TextInput';

export interface DebouncedInputProps extends Omit<TextInputProps, 'onChange'> {
  value?: string | number;
  onChange: (value: string) => void;
  debounceMs?: number;
}

export const DebouncedInput: React.FC<DebouncedInputProps> = ({
  value: initialValue,
  onChange,
  debounceMs = 400,
  ...props
}) => {
  const [value, setValue] = useState<string>(initialValue !== undefined ? String(initialValue) : '');

  // Stabilize onChange callback reference to prevent parent re-renders from restarting the debounce timer
  const onChangeRef = useRef(onChange);
  useEffect(() => {
    onChangeRef.current = onChange;
  }, [onChange]);

  useEffect(() => {
    setValue(initialValue !== undefined ? String(initialValue) : '');
  }, [initialValue]);

  useEffect(() => {
    const handler = setTimeout(() => {
      onChangeRef.current(value);
    }, debounceMs);

    return () => clearTimeout(handler);
  }, [value, debounceMs]);

  return (
    <TextInput
      value={value}
      onChange={(e) => setValue(e.target.value)}
      {...props}
    />
  );
};
