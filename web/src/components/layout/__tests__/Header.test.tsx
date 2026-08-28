import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, within, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Header } from '../Header';
import { AuthProvider } from '../../../auth/AuthContext';
import { Router } from '../../../router';
import { authApi } from '../../../api/client';

describe('Header Component', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('renders branding, 7 category links, calculate, swagger, and theme toggle', () => {
    render(
      <Router>
        <AuthProvider>
          <Header />
        </AuthProvider>
      </Router>
    );

    expect(screen.getByText('CloudVitta')).toBeInTheDocument();
    expect(screen.getByText('API Engine')).toBeInTheDocument();
    expect(screen.getByText('Compute')).toBeInTheDocument();
    expect(screen.getByText('Storage')).toBeInTheDocument();
    expect(screen.getByText('Network')).toBeInTheDocument();
    expect(screen.getByText('RDBMS')).toBeInTheDocument();
    expect(screen.getByText('NoSQL')).toBeInTheDocument();
    expect(screen.getByText('Kubernetes')).toBeInTheDocument();
    expect(screen.getByText('Serverless')).toBeInTheDocument();
    expect(screen.getByText('Calculate')).toBeInTheDocument();
    expect(screen.getByText('Swagger')).toBeInTheDocument();
  });

  it('renders Sign In button when unauthenticated and opens AuthModal on click', async () => {
    const user = userEvent.setup();
    render(
      <Router>
        <AuthProvider>
          <Header />
        </AuthProvider>
      </Router>
    );

    const signInBtn = screen.getByRole('button', { name: /sign in/i });
    expect(signInBtn).toBeInTheDocument();

    await user.click(signInBtn);

    const modal = screen.getByRole('dialog');
    expect(modal).toBeInTheDocument();
    expect(within(modal).getByRole('heading', { level: 2 })).toHaveTextContent('Sign In to Standard Tier');
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

    render(
      <Router>
        <AuthProvider>
          <Header />
        </AuthProvider>
      </Router>
    );

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
});
