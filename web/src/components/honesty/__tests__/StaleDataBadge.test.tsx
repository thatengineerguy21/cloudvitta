import { render, screen } from '@testing-library/react';
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { StaleDataBadge } from '../StaleDataBadge';
import { formatRelativeTime } from '../../../lib/format';

describe('StaleDataBadge Component', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-08-28T12:00:00Z'));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('computes relative time accurately for hours and days', () => {
    expect(formatRelativeTime('2026-08-28T08:00:00Z')).toBe('4h ago');
    expect(formatRelativeTime('2026-08-20T12:00:00Z')).toBe('8d ago');
    expect(formatRelativeTime(undefined)).toBe('Age unknown');
  });

  it('renders stale badge with relative timestamp and accessible styling', () => {
    const { container } = render(<StaleDataBadge fetchedAt="2026-08-20T12:00:00Z" />);
    const badge = screen.getByText('Stale (8d ago)').parentElement;

    expect(badge).toBeInTheDocument();
    expect(badge).toHaveClass('border-status-stale', 'text-status-stale', 'bg-status-stale/10');

    const svg = container.querySelector('svg');
    expect(svg).toHaveAttribute('aria-hidden', 'true');
  });
});
