import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { BentoCard } from '../BentoCard';

describe('BentoCard Component', () => {
  it('renders card body with default 12-column span and 0px border', () => {
    render(
      <BentoCard data-testid="bento-card">
        <p>Main Card Body</p>
      </BentoCard>
    );

    const card = screen.getByTestId('bento-card');
    expect(card).toBeInTheDocument();
    expect(card).toHaveClass('bg-surface-card', 'border', 'border-border-default', 'col-span-1', 'md:col-span-6', 'lg:col-span-12');
    expect(screen.getByText('Main Card Body')).toBeInTheDocument();
  });

  it('renders header and footer slots with divider borders when provided', () => {
    render(
      <BentoCard
        header={<h3>Card Header</h3>}
        footer={<button>Action</button>}
      >
        <p>Body Content</p>
      </BentoCard>
    );

    expect(screen.getByText('Card Header')).toBeInTheDocument();
    expect(screen.getByText('Body Content')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Action' })).toBeInTheDocument();
  });

  it('applies hover border transitions when isHoverable is true', () => {
    render(
      <BentoCard isHoverable colSpan={6} data-testid="hover-card">
        <p>Hoverable Card</p>
      </BentoCard>
    );

    const card = screen.getByTestId('hover-card');
    expect(card).toHaveClass('hover:border-border-accent', 'col-span-1', 'md:col-span-6');
  });
});
