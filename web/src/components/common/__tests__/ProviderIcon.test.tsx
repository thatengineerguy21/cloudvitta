import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { ProviderIcon } from '../ProviderIcon';

describe('ProviderIcon Component', () => {
  it('renders AWS icon with proper aria-label', () => {
    render(<ProviderIcon provider="aws" data-testid="icon-aws" />);
    const icon = screen.getByTestId('icon-aws');
    expect(icon).toBeInTheDocument();
    expect(icon).toHaveAttribute('aria-label', 'AWS');
  });

  it('renders Microsoft Azure icon with proper aria-label', () => {
    render(<ProviderIcon provider="azure" data-testid="icon-azure" />);
    const icon = screen.getByTestId('icon-azure');
    expect(icon).toBeInTheDocument();
    expect(icon).toHaveAttribute('aria-label', 'Microsoft Azure');
  });

  it('renders Google Cloud Platform icon with proper aria-label', () => {
    render(<ProviderIcon provider="gcp" data-testid="icon-gcp" />);
    const icon = screen.getByTestId('icon-gcp');
    expect(icon).toBeInTheDocument();
    expect(icon).toHaveAttribute('aria-label', 'Google Cloud Platform');
  });

  it('renders Oracle Cloud Infrastructure icon with proper aria-label', () => {
    render(<ProviderIcon provider="oracle" data-testid="icon-oracle" />);
    const icon = screen.getByTestId('icon-oracle');
    expect(icon).toBeInTheDocument();
    expect(icon).toHaveAttribute('aria-label', 'Oracle Cloud Infrastructure');
  });

  it('renders DigitalOcean icon with proper aria-label', () => {
    render(<ProviderIcon provider="digitalocean" data-testid="icon-do" />);
    const icon = screen.getByTestId('icon-do');
    expect(icon).toBeInTheDocument();
    expect(icon).toHaveAttribute('aria-label', 'DigitalOcean');
  });

  it('renders IBM Cloud icon with proper aria-label', () => {
    render(<ProviderIcon provider="ibm" data-testid="icon-ibm" />);
    const icon = screen.getByTestId('icon-ibm');
    expect(icon).toBeInTheDocument();
    expect(icon).toHaveAttribute('aria-label', 'IBM Cloud');
  });

  it('renders Alibaba Cloud icon with proper aria-label', () => {
    render(<ProviderIcon provider="alibaba" data-testid="icon-ali" />);
    const icon = screen.getByTestId('icon-ali');
    expect(icon).toBeInTheDocument();
    expect(icon).toHaveAttribute('aria-label', 'Alibaba Cloud');
  });

  it('renders fallback icon for unknown provider', () => {
    render(<ProviderIcon provider="unknown-cloud" data-testid="icon-fallback" />);
    const icon = screen.getByTestId('icon-fallback');
    expect(icon).toBeInTheDocument();
    expect(icon).toHaveAttribute('aria-label', 'unknown-cloud');
  });
});
