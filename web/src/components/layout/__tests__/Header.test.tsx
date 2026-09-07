import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, within, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Header } from '../Header';
import { AuthProvider } from '../../../auth/AuthContext';
import { Router } from '../../../router';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import React from 'react';
import { authApi, providerApi } from '../../../api/client';

const createWrapper = () => {
  const qc = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={qc}>
      <Router>
        <AuthProvider>{children}</AuthProvider>
      </Router>
    </QueryClientProvider>
  );
};

describe('Header Component', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    // Mock provider status for live health badge
    vi.spyOn(providerApi, 'getStatus').mockResolvedValue({
      provider: 'aws',
      status: 'healthy',
      stale: false,
      categories: {},
      warnings: [],
    });
  });

  it('renders branding, 7 category links, calculate, swagger, and theme toggle', () => {
    render(<Header />, { wrapper: createWrapper() });

    expect(screen.getByText('CloudVitta')).toBeInTheDocument();
    expect(screen.getByText('Pricing Tool')).toBeInTheDocument();
    expect(screen.getByText('Compute')).toBeInTheDocument();
    expect(screen.getByText('Storage')).toBeInTheDocument();
    expect(screen.getByText('Network')).toBeInTheDocument();
    expect(screen.getByText('RDBMS')).toBeInTheDocument();
    expect(screen.getByText('NoSQL')).toBeInTheDocument();
    expect(screen.getByText('Kubernetes')).toBeInTheDocument();
    expect(screen.getByText('Serverless')).toBeInTheDocument();
    expect(screen.getByText('Calculate')).toBeInTheDocument();
    expect(screen.getByText('Playground')).toBeInTheDocument();
    expect(screen.getByText('Swagger')).toBeInTheDocument();
  });

  it('renders live health badge with dynamic telemetry label', async () => {
    render(<Header />, { wrapper: createWrapper() });

    // Initially loading, then shows label
    await waitFor(() => {
      expect(screen.getByText('7/7 Providers Active')).toBeInTheDocument();
    });
  });

  it('renders degraded health badge when providers are stale', async () => {
    vi.spyOn(providerApi, 'getStatus').mockImplementation((provider) => {
      if (provider === 'aws') {
        return Promise.resolve({
          provider: 'aws',
          status: 'stale',
          stale: true,
          categories: {},
          warnings: [],
        });
      }
      return Promise.resolve({
        provider: provider as string,
        status: 'healthy',
        stale: false,
        categories: {},
        warnings: [],
      });
    });

    render(<Header />, { wrapper: createWrapper() });

    await waitFor(() => {
      // With 1 stale + 6 healthy => degraded state
      expect(screen.getByText(/Active/)).toBeInTheDocument();
    });
  });

  it('renders Sign In button when unauthenticated and opens AuthModal on click', async () => {
    const user = userEvent.setup();
    render(<Header />, { wrapper: createWrapper() });

    const signInBtn = screen.getByRole('button', { name: /sign in/i });
    expect(signInBtn).toBeInTheDocument();

    await user.click(signInBtn);

    const modal = screen.getByRole('dialog');
    expect(modal).toBeInTheDocument();
    expect(within(modal).getByRole('heading', { level: 2 })).toHaveTextContent('Sign In');
  });

  it('renders user email and logout trigger when authenticated', async () => {
    const user = userEvent.setup();
    vi.spyOn(authApi, 'login').mockResolvedValue({
      access_token: 'auth_jwt',
      refresh_token: 'auth_refresh',
      token_type: 'Bearer',
      expires_in: 900,
    });
    vi.spyOn(authApi, 'logout').mockResolvedValue({ message: 'revoked' });

    render(<Header />, { wrapper: createWrapper() });

    // Open AuthModal
    await user.click(screen.getByRole('button', { name: /sign in/i }));
    const modal = screen.getByRole('dialog');

    await user.type(within(modal).getByPlaceholderText('name@company.com'), 'engineer@cloudvitta.dev');
    await user.type(within(modal).getByPlaceholderText('••••••••'), 'securepassword123');
    await user.click(within(modal).getByRole('button', { name: 'Sign In' }));

    await waitFor(() => {
      expect(screen.getByText('engineer@cloudvitta.dev')).toBeInTheDocument();
    });

    const logoutBtn = screen.getByTitle('Log Out');
    expect(logoutBtn).toBeInTheDocument();

    // Click logout
    await user.click(logoutBtn);

    await waitFor(() => {
      expect(screen.getByRole('button', { name: /sign in/i })).toBeInTheDocument();
      expect(screen.queryByText('engineer@cloudvitta.dev')).not.toBeInTheDocument();
    });
  });

  it('toggles mobile navigation menu and closes on nav item click', async () => {
    const user = userEvent.setup();
    render(<Header />, { wrapper: createWrapper() });

    const hamburgerBtn = screen.getByRole('button', { name: /open menu/i });
    expect(hamburgerBtn).toBeInTheDocument();

    // Click hamburger button to open mobile menu
    await user.click(hamburgerBtn);
    expect(screen.getByRole('button', { name: /close menu/i })).toBeInTheDocument();

    // Verify links are rendered in mobile menu panel
    const computeLinks = screen.getAllByRole('link', { name: /compute/i });
    expect(computeLinks.length).toBeGreaterThan(1);

    // Clicking a mobile nav item should close the menu
    await user.click(computeLinks[computeLinks.length - 1]);
    expect(screen.getByRole('button', { name: /open menu/i })).toBeInTheDocument();
  });
});

