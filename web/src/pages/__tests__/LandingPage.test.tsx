// web/src/pages/__tests__/LandingPage.test.tsx
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';
import { Router } from '../../router';
import { providerApi } from '../../api/client';
import { LandingPage } from '../LandingPage';

const createWrapper = () => {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={qc}>
      <Router>{children}</Router>
    </QueryClientProvider>
  );
};

describe('LandingPage', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.spyOn(providerApi, 'getStatus').mockResolvedValue({
      provider: 'aws',
      status: 'healthy',
      stale: false,
      categories: {},
      warnings: [],
    });
  });

  it('renders hero headline and description', () => {
    render(<LandingPage />, { wrapper: createWrapper() });

    expect(
      screen.getByRole('heading', {
        level: 1,
        name: 'Normalized Cloud Infrastructure Pricing',
      })
    ).toBeInTheDocument();
    expect(screen.getByText(/Compare and calculate cloud infrastructure/)).toBeInTheDocument();
  });

  it('renders dual workload entry choices', () => {
    render(<LandingPage />, { wrapper: createWrapper() });

    expect(screen.getByText('Compare Individual Category')).toBeInTheDocument();
    expect(screen.getByText('Calculate Composite Workload')).toBeInTheDocument();
  });

  it('renders all 7 category cards', () => {
    render(<LandingPage />, { wrapper: createWrapper() });

    expect(screen.getByText('Compute Instances')).toBeInTheDocument();
    expect(screen.getByText('Storage Classes')).toBeInTheDocument();
    expect(screen.getByText('Network Egress')).toBeInTheDocument();
    expect(screen.getByText('Relational DBs (RDBMS)')).toBeInTheDocument();
    expect(screen.getByText('NoSQL Databases')).toBeInTheDocument();
    expect(screen.getByText('Kubernetes Control-Plane')).toBeInTheDocument();
    expect(screen.getByText('Serverless Compute (FaaS)')).toBeInTheDocument();
  });

  it('renders Swagger and GitHub links', () => {
    render(<LandingPage />, { wrapper: createWrapper() });

    const swaggerLink = screen.getByText('Swagger API Docs').closest('a');
    expect(swaggerLink).toHaveAttribute('href', '/docs/');

    const githubLink = screen.getByText('GitHub').closest('a');
    expect(githubLink).toHaveAttribute('href', 'https://github.com/thatengineerguy21/cloudvitta');
  });
});
