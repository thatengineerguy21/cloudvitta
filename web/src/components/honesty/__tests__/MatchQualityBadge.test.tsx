import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { MatchQualityBadge } from '../MatchQualityBadge';

describe('MatchQualityBadge Component', () => {
  it('renders exact match badge with Forest Green / Neon token styling', () => {
    render(<MatchQualityBadge quality="exact" score={1.0} />);
    const badge = screen.getByText('Exact Match').parentElement;
    expect(badge).toBeInTheDocument();
    expect(badge).toHaveClass('border-status-matchExact', 'text-status-matchExact', 'bg-status-matchExact/10');
    expect(badge).toHaveAttribute('title', 'Match Quality: Exact Match (Score: 1.00)');
  });

  it('renders close match badge with score and delta percentage', () => {
    render(<MatchQualityBadge quality="close" score={0.85} deltaPct={15} />);
    expect(screen.getByText('Close Match')).toBeInTheDocument();
    expect(screen.getByText('+15%')).toBeInTheDocument();
    const badge = screen.getByText('Close Match').parentElement;
    expect(badge).toHaveClass('border-status-matchClose', 'text-status-matchClose');
    expect(badge).toHaveAttribute('title', 'Match Quality: Close Match (Score: 0.85, Delta: 15%)');
  });

  it('renders approximate badge with approximate token styling', () => {
    render(<MatchQualityBadge quality="approximate" />);
    const badge = screen.getByText('Approximate').parentElement;
    expect(badge).toHaveClass('border-status-matchApproximate', 'text-status-matchApproximate');
  });
});
