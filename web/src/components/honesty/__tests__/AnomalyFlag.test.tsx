import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { AnomalyFlag } from '../AnomalyFlag';

describe('AnomalyFlag Component', () => {
  it('renders Under Review indicator with default reason and anomaly styling', () => {
    render(<AnomalyFlag />);
    const flag = screen.getByText('Under Review').parentElement;
    expect(flag).toBeInTheDocument();
    expect(flag).toHaveClass('border-status-anomaly', 'text-status-anomaly', 'bg-status-anomaly/10');
    expect(flag).toHaveAttribute(
      'title',
      'Price observation under review: An unusual rate change was detected from this provider.'
    );
  });

  it('renders custom reason and hides decorative icon from assistive technologies', () => {
    const { container } = render(<AnomalyFlag reason="Custom >20x jump" />);
    const flag = screen.getByText('Under Review').parentElement;
    expect(flag).toHaveAttribute('title', 'Custom >20x jump');

    const svg = container.querySelector('svg');
    expect(svg).toHaveAttribute('aria-hidden', 'true');
  });
});
