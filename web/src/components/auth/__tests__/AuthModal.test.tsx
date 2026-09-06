import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AuthModal } from '../AuthModal';
import { AuthProvider } from '../../../auth/AuthContext';
import { authApi } from '../../../api/client';
import { ApiError } from '../../../api/errors';

describe('AuthModal Component', () => {
  const onCloseMock = vi.fn();

  beforeEach(() => {
    vi.restoreAllMocks();
    onCloseMock.mockClear();
  });

  it('renders nothing when isOpen is false', () => {
    const { container } = render(
      <AuthProvider>
        <AuthModal isOpen={false} onClose={onCloseMock} />
      </AuthProvider>
    );

    expect(container).toBeEmptyDOMElement();
  });

  it('renders login modal when isOpen is true', () => {
    render(
      <AuthProvider>
        <AuthModal isOpen={true} onClose={onCloseMock} initialMode="login" />
      </AuthProvider>
    );

    expect(screen.getByRole('dialog')).toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('Sign In');
    expect(screen.getByPlaceholderText('name@company.com')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('••••••••')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Sign In' })).toBeInTheDocument();
  });

  it('closes when close button is clicked or Escape key is pressed', async () => {
    const user = userEvent.setup();
    render(
      <AuthProvider>
        <AuthModal isOpen={true} onClose={onCloseMock} />
      </AuthProvider>
    );

    await user.click(screen.getByLabelText('Close dialog'));
    expect(onCloseMock).toHaveBeenCalledTimes(1);

    fireEvent.keyDown(window, { key: 'Escape' });
    expect(onCloseMock).toHaveBeenCalledTimes(2);
  });

  it('shows Zod validation errors on empty submission in login mode', async () => {
    const user = userEvent.setup();
    render(
      <AuthProvider>
        <AuthModal isOpen={true} onClose={onCloseMock} initialMode="login" />
      </AuthProvider>
    );

    await user.click(screen.getByRole('button', { name: 'Sign In' }));

    expect(screen.getByText('Email is required')).toBeInTheDocument();
    expect(screen.getByText('Password must be at least 8 characters')).toBeInTheDocument();
  });

  it('submits valid login credentials and closes modal on success', async () => {
    const user = userEvent.setup();
    vi.spyOn(authApi, 'login').mockResolvedValue({
      access_token: 'valid_jwt',
      refresh_token: 'valid_refresh',
      token_type: 'Bearer',
      expires_in: 900,
    });

    render(
      <AuthProvider>
        <AuthModal isOpen={true} onClose={onCloseMock} initialMode="login" />
      </AuthProvider>
    );

    await user.type(screen.getByPlaceholderText('name@company.com'), 'valid@company.com');
    await user.type(screen.getByPlaceholderText('••••••••'), 'validpassword123');
    await user.click(screen.getByRole('button', { name: 'Sign In' }));

    await waitFor(() => {
      expect(onCloseMock).toHaveBeenCalled();
    });
  });

  it('displays API error banner when login returns 401', async () => {
    const user = userEvent.setup();
    vi.spyOn(authApi, 'login').mockRejectedValue(
      new ApiError({
        type: 'https://cloudvitta.dev/errors/unauthorized',
        title: 'Unauthorized',
        status: 401,
        detail: 'Invalid email or password',
        instance: '/api/v1/auth/login',
      })
    );

    render(
      <AuthProvider>
        <AuthModal isOpen={true} onClose={onCloseMock} initialMode="login" />
      </AuthProvider>
    );

    await user.type(screen.getByPlaceholderText('name@company.com'), 'wrong@company.com');
    await user.type(screen.getByPlaceholderText('••••••••'), 'wrongpassword');
    await user.click(screen.getByRole('button', { name: 'Sign In' }));

    expect(await screen.findByText('Invalid email or password')).toBeInTheDocument();
    expect(onCloseMock).not.toHaveBeenCalled();
  });

  it('switches between Login and Signup modes', async () => {
    const user = userEvent.setup();
    render(
      <AuthProvider>
        <AuthModal isOpen={true} onClose={onCloseMock} initialMode="login" />
      </AuthProvider>
    );

    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('Sign In');

    await user.click(screen.getByRole('button', { name: /sign up/i }));
    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('Create an Account');
    expect(screen.getByText('Confirm Password')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /sign in/i }));
    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent('Sign In');
  });

  it('validates password mismatch on signup', async () => {
    const user = userEvent.setup();
    render(
      <AuthProvider>
        <AuthModal isOpen={true} onClose={onCloseMock} initialMode="signup" />
      </AuthProvider>
    );

    await user.type(screen.getByPlaceholderText('name@company.com'), 'newuser@company.com');
    const passwordInputs = screen.getAllByPlaceholderText('••••••••');
    await user.type(passwordInputs[0], 'password123');
    await user.type(passwordInputs[1], 'differentpassword');

    await user.click(screen.getByRole('button', { name: 'Create Account' }));

    expect(screen.getByText('Passwords do not match')).toBeInTheDocument();
  });

  it('submits valid signup and closes modal on success', async () => {
    const user = userEvent.setup();
    vi.spyOn(authApi, 'signup').mockResolvedValue({
      id: 'usr_new',
      email: 'newuser@company.com',
      created_at: '2026-08-27T00:00:00Z',
    });
    vi.spyOn(authApi, 'login').mockResolvedValue({
      access_token: 'signup_jwt',
      refresh_token: 'signup_refresh',
      token_type: 'Bearer',
      expires_in: 900,
    });

    render(
      <AuthProvider>
        <AuthModal isOpen={true} onClose={onCloseMock} initialMode="signup" />
      </AuthProvider>
    );

    await user.type(screen.getByPlaceholderText('name@company.com'), 'newuser@company.com');
    const passwordInputs = screen.getAllByPlaceholderText('••••••••');
    await user.type(passwordInputs[0], 'securepassword123');
    await user.type(passwordInputs[1], 'securepassword123');

    await user.click(screen.getByRole('button', { name: 'Create Account' }));

    await waitFor(() => {
      expect(onCloseMock).toHaveBeenCalled();
    });
  });
});
