import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { BentoGrid } from '../BentoGrid';

describe('BentoGrid Component', () => {
  it('renders children within a default 12-column grid layout', () => {
    render(
      <BentoGrid data-testid="bento-grid">
        <div>Item 1</div>
        <div>Item 2</div>
      </BentoGrid>
    );

    const grid = screen.getByTestId('bento-grid');
    expect(grid).toBeInTheDocument();
    expect(grid).toHaveClass('grid', 'w-full', 'grid-cols-1', 'md:grid-cols-6', 'lg:grid-cols-12', 'gap-4');
    expect(screen.getByText('Item 1')).toBeInTheDocument();
    expect(screen.getByText('Item 2')).toBeInTheDocument();
  });

  it('applies custom column and gap classes', () => {
    render(
      <BentoGrid columns={6} gap="none" data-testid="bento-grid-6">
        <div>Item</div>
      </BentoGrid>
    );

    const grid = screen.getByTestId('bento-grid-6');
    expect(grid).toHaveClass('grid-cols-1', 'md:grid-cols-3', 'lg:grid-cols-6', 'gap-0');
  });

  it('forwards custom className and html attributes', () => {
    render(
      <BentoGrid className="custom-class" aria-label="Workload Comparison" data-testid="custom-grid">
        <div>Content</div>
      </BentoGrid>
    );

    const grid = screen.getByTestId('custom-grid');
    expect(grid).toHaveClass('custom-class');
    expect(grid).toHaveAttribute('aria-label', 'Workload Comparison');
  });
});
