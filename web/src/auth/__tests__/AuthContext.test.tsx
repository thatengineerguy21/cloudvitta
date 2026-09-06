import React from 'react';
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AuthProvider, useAuth } from '../AuthContext';
import { authApi } from '../../api/client';
import { getAuthTokenProvider } from '../../api/refreshQueue';

// Test consumer component
const TestConsumer: React.FC = () => {
  const { user, tokens, isAuthenticated, isLoading, login, signup, logout } = useAuth();

  return (
    <div>
      <div data-testid="auth-status">{isAuthenticated ? 'authenticated' : 'unauthenticated'}</div>
      <div data-testid="user-email">{user?.email || 'none'}</div>
      <div data-testid="access-token">{tokens?.accessToken || 'none'}</div>
      <div data-testid="loading">{isLoading ? 'loading' : 'idle'}</div>

      <button onClick={() => login('test@example.com', 'password123')}>Login</button>
      <button onClick={() => signup('signup@example.com', 'password123')}>Signup</button>
      <button onClick={() => logout()}>Logout</button>
    </div>
  );
};

describe('AuthContext & AuthProvider', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    if (typeof window !== 'undefined') {
      window.localStorage?.clear();
      window.sessionStorage?.clear();
    }
  });

  it('throws error when useAuth is called outside AuthProvider', () => {
    // Suppress console.error from React boundary error
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {});
    expect(() => render(<TestConsumer />)).toThrow(
      'useAuth must be used within an AuthProvider'
    );
    spy.mockRestore();
  });

  it('starts in unauthenticated anonymous state with zero web storage footprint', () => {
    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>
    );

    expect(screen.getByTestId('auth-status')).toHaveTextContent('unauthenticated');
    expect(screen.getByTestId('user-email')).toHaveTextContent('none');
    expect(screen.getByTestId('access-token')).toHaveTextContent('none');
    expect(screen.getByTestId('loading')).toHaveTextContent('idle');
    expect(window.localStorage.length).toBe(0);
    expect(window.sessionStorage.length).toBe(0);
  });

  it('handles login successfully, setting in-memory state and token provider', async () => {
    const user = userEvent.setup();
    vi.spyOn(authApi, 'login').mockResolvedValue({
      access_token: 'mock_access_jwt',
      refresh_token: 'mock_refresh_token',
      token_type: 'Bearer',
      expires_in: 900,
    });

    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>
    );

    await user.click(screen.getByText('Login'));

    expect(screen.getByTestId('auth-status')).toHaveTextContent('authenticated');
    expect(screen.getByTestId('user-email')).toHaveTextContent('test@example.com');
    expect(screen.getByTestId('access-token')).toHaveTextContent('mock_access_jwt');

    // Verify token provider
    const tokenProvider = getAuthTokenProvider();
    expect(tokenProvider?.getAccessToken()).toBe('mock_access_jwt');
    expect(tokenProvider?.getRefreshToken()).toBe('mock_refresh_token');

    // Invariant: zero storage footprint
    expect(window.localStorage.length).toBe(0);
    expect(window.sessionStorage.length).toBe(0);
  });

  it('handles signup without automatic login pending verification', async () => {
    const user = userEvent.setup();
    const signupSpy = vi.spyOn(authApi, 'signup').mockResolvedValue({
      id: 'usr_123',
      email: 'signup@example.com',
      created_at: '2026-08-27T00:00:00Z',
    });
    const loginSpy = vi.spyOn(authApi, 'login');

    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>
    );

    await user.click(screen.getByText('Signup'));

    expect(signupSpy).toHaveBeenCalledWith({ email: 'signup@example.com', password: 'password123' });
    expect(loginSpy).not.toHaveBeenCalled();

    expect(screen.getByTestId('auth-status')).toHaveTextContent('unauthenticated');
  });

  it('handles logout and clears in-memory state cleanly even if backend fails', async () => {
    const user = userEvent.setup();
    vi.spyOn(authApi, 'login').mockResolvedValue({
      access_token: 'jwt_to_logout',
      refresh_token: 'refresh_to_logout',
      token_type: 'Bearer',
      expires_in: 900,
    });
    const logoutSpy = vi.spyOn(authApi, 'logout').mockRejectedValue(new Error('Network error on logout'));

    render(
      <AuthProvider>
        <TestConsumer />
      </AuthProvider>
    );

    await user.click(screen.getByText('Login'));
    expect(screen.getByTestId('auth-status')).toHaveTextContent('authenticated');

    await user.click(screen.getByText('Logout'));

    expect(logoutSpy).toHaveBeenCalledWith({ refresh_token: 'refresh_to_logout' });
    expect(screen.getByTestId('auth-status')).toHaveTextContent('unauthenticated');
    expect(screen.getByTestId('user-email')).toHaveTextContent('none');
    expect(screen.getByTestId('access-token')).toHaveTextContent('none');

    const tokenProvider = getAuthTokenProvider();
    expect(tokenProvider?.getAccessToken()).toBeNull();
    expect(tokenProvider?.getRefreshToken()).toBeNull();
  });
});
