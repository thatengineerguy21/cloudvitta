// web/src/__tests__/App.test.tsx
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import { App } from '../App';
import { providerApi, pricesApi, calculateApi } from '../api/client';

describe('App application routing and layout', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    // Mock provider status for Header live badge + landing page health pill
    vi.spyOn(providerApi, 'getStatus').mockResolvedValue({
      provider: 'aws',
      status: 'healthy',
      stale: false,
      categories: {},
      warnings: [],
    });
    vi.spyOn(pricesApi, 'getCompute').mockResolvedValue({ results: [], meta: {} });
    vi.spyOn(calculateApi, 'calculate').mockResolvedValue({ results: [], meta: {}, warnings: [] });
  });

  it('renders landing page on root path', () => {
    window.history.replaceState(null, '', '/');
    render(<App />);

    expect(
      screen.getByRole('heading', {
        level: 1,
        name: 'Normalized Cloud Infrastructure Pricing',
      })
    ).toBeInTheDocument();
    expect(screen.getByText('Compute Instances')).toBeInTheDocument();
    expect(screen.getByText('Relational DBs (RDBMS)')).toBeInTheDocument();
  });

  it('renders provider status page on /status path', async () => {
    window.history.replaceState(null, '', '/status');
    render(<App />);

    await waitFor(() => {
      expect(
        screen.getByRole('heading', {
          level: 1,
          name: /Provider Operational Status/,
        })
      ).toBeInTheDocument();
    });
  });

  it('renders 404 on unknown path', () => {
    window.history.replaceState(null, '', '/unknown');
    render(<App />);

    expect(screen.getByText('Page Not Found')).toBeInTheDocument();
    expect(screen.getByTestId('not-found-page')).toBeInTheDocument();
  });

  it('renders composite calculate page on /calculate path', () => {
    window.history.replaceState(null, '', '/calculate');
    render(<App />);

    expect(
      screen.getByRole('heading', { level: 1, name: 'Composite Workload Calculator' })
    ).toBeInTheDocument();
  });
});
