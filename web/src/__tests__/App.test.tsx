// web/src/__tests__/App.test.tsx
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { App } from '../App';
import { pricesApi, calculateApi } from '../api/client';

describe('App application routing and layout', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.spyOn(pricesApi, 'getCompute').mockResolvedValue({ results: [], meta: {} });
    vi.spyOn(calculateApi, 'calculate').mockResolvedValue({ results: [], meta: {}, warnings: [] });
  });

  it('renders landing page on root path', () => {
    window.history.replaceState(null, '', '/');
    render(<App />);

    expect(screen.getByText('Cloud Pricing Normalization Engine')).toBeInTheDocument();
    expect(screen.getByText('Compute Instances')).toBeInTheDocument();
    expect(screen.getByText('Relational DBs (RDBMS)')).toBeInTheDocument();
  });

  it('renders composite calculate page on /calculate path', () => {
    window.history.replaceState(null, '', '/calculate');
    render(<App />);

    expect(screen.getByRole('heading', { level: 1, name: 'Composite Workload Calculator' })).toBeInTheDocument();
    expect(screen.getByText('Global Architecture Parameters')).toBeInTheDocument();
  });
});
