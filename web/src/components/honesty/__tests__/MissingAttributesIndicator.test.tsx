import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { MissingAttributesIndicator } from '../MissingAttributesIndicator';

describe('MissingAttributesIndicator Component', () => {
  it('returns null when missingAttributes array is empty or undefined', () => {
    const { container } = render(<MissingAttributesIndicator missingAttributes={[]} />);
    expect(container.firstChild).toBeNull();
  });

  it('renders trigger button with count and aria-expanded state (Task 4 acceptance criterion)', () => {
    render(<MissingAttributesIndicator missingAttributes={['iops', 'multi_az']} />);

    const button = screen.getByRole('button', { name: '2 unspecified specifications' });
    expect(button).toBeInTheDocument();
    expect(button).toHaveAttribute('aria-expanded', 'false');
    expect(button).toHaveAttribute('aria-haspopup', 'true');

    // Clicking trigger toggles aria-expanded and displays popover with role="tooltip"
    fireEvent.click(button);
    expect(button).toHaveAttribute('aria-expanded', 'true');

    const tooltip = screen.getByRole('tooltip');
    expect(tooltip).toBeInTheDocument();
    expect(screen.getByText('Unspecified Specifications')).toBeInTheDocument();
    expect(
      screen.getByText('The matched cloud configuration does not declare the following requested parameters:')
    ).toBeInTheDocument();
    expect(screen.getByText('iops')).toBeInTheDocument();
    expect(screen.getByText('multi_az')).toBeInTheDocument();

    // Clicking again closes popover
    fireEvent.click(button);
    expect(button).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
  });

  it('opens popover on mouse enter and closes on mouse leave', () => {
    const { container } = render(<MissingAttributesIndicator missingAttributes={['gpu_count']} />);
    const wrapper = container.firstChild as HTMLElement;

    fireEvent.mouseEnter(wrapper);
    expect(screen.getByRole('tooltip')).toBeInTheDocument();
    expect(screen.getByText('gpu_count')).toBeInTheDocument();

    fireEvent.mouseLeave(wrapper);
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
  });
});
