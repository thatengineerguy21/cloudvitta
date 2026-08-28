import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { TextInput } from '../TextInput';
import { NumberInput } from '../NumberInput';
import { SelectInput } from '../SelectInput';

describe('Form Controls (TextInput, NumberInput, SelectInput)', () => {
  describe('TextInput', () => {
    it('renders with label and handles text change', () => {
      const handleChange = vi.fn();
      render(<TextInput label="Database Name" onChange={handleChange} />);

      const label = screen.getByText('Database Name');
      const input = screen.getByLabelText('Database Name');
      expect(label).toBeInTheDocument();
      expect(input).toBeInTheDocument();

      fireEvent.change(input, { target: { value: 'production-db' } });
      expect(handleChange).toHaveBeenCalledTimes(1);
    });

    it('renders error message and applies anomaly border when error is passed', () => {
      render(<TextInput label="Email" error="Invalid email address" />);
      const errorText = screen.getByText('Invalid email address');
      const input = screen.getByLabelText('Email');

      expect(errorText).toBeInTheDocument();
      expect(input).toHaveClass('border-status-anomaly');
    });

    it('renders helper text when error is absent', () => {
      render(<TextInput label="Username" helperText="Must be unique" />);
      expect(screen.getByText('Must be unique')).toBeInTheDocument();
    });
  });

  describe('NumberInput', () => {
    it('parses numeric values and triggers onChange with number or undefined', () => {
      const handleChange = vi.fn();
      render(<NumberInput label="vCPU Count" onChange={handleChange} min={1} max={128} />);

      const input = screen.getByLabelText('vCPU Count');
      fireEvent.change(input, { target: { value: '16' } });
      expect(handleChange).toHaveBeenCalledWith(16);

      fireEvent.change(input, { target: { value: '' } });
      expect(handleChange).toHaveBeenCalledWith(undefined);
    });
  });

  describe('SelectInput', () => {
    it('renders select with options and handles selection change', () => {
      const handleChange = vi.fn();
      const options = [
        { value: 'us-east', label: 'US East' },
        { value: 'eu-west', label: 'Europe West' },
      ];

      render(<SelectInput label="Region" options={options} onChange={handleChange} />);

      const select = screen.getByLabelText('Region');
      expect(select).toBeInTheDocument();
      expect(screen.getByRole('option', { name: 'US East' })).toBeInTheDocument();
      expect(screen.getByRole('option', { name: 'Europe West' })).toBeInTheDocument();

      fireEvent.change(select, { target: { value: 'eu-west' } });
      expect(handleChange).toHaveBeenCalledTimes(1);
    });
  });
});
