import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { PriceDisplay } from '../PriceDisplay';

describe('PriceDisplay Component', () => {
  it('renders formatted price in requested currency with unit', () => {
    render(<PriceDisplay amount="0.096" currency="USD" unit="/hr" />);

    expect(screen.getByText('USD 0.0960')).toBeInTheDocument();
    expect(screen.getByText('/hr')).toBeInTheDocument();
  });

  it('renders fallback dash when amount is undefined or empty', () => {
    render(<PriceDisplay currency="EUR" />);
    expect(screen.getByText('EUR —')).toBeInTheDocument();
  });

  it('renders normalized USD rate when currency is non-USD', () => {
    render(
      <PriceDisplay
        amount="14.50"
        currency="EUR"
        unit="/hr"
        normalizedHourlyUSD="15.80"
      />
    );

    expect(screen.getByText('EUR 14.5000')).toBeInTheDocument();
    expect(screen.getByText('≈ $15.8000 USD/hr')).toBeInTheDocument();
  });

  it('mounts inline AnomalyFlag when hasAnomaly is true', () => {
    render(<PriceDisplay amount="12.00" currency="USD" hasAnomaly />);
    expect(screen.getByText('Under Review')).toBeInTheDocument();
  });

  it('renders partial workload indicator when isPartial is true', () => {
    render(<PriceDisplay amount="50.00" currency="USD" isPartial />);
    expect(screen.getByText('Partial Workload Total')).toBeInTheDocument();
  });
});
