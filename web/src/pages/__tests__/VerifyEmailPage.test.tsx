import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Router } from '../../router';
import { VerifyEmailPage } from '../VerifyEmailPage';
import * as client from '../../api/client';
import { ApiError } from '../../api/errors';

describe('VerifyEmailPage', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('renders missing token message when no token is present in URL', () => {
    window.history.replaceState(null, '', '/verify-email');
    render(
      <Router>
        <VerifyEmailPage />
      </Router>
    );

    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Verification Failed');
    expect(screen.getByText('No verification token provided in the URL link.')).toBeInTheDocument();
    expect(screen.getByRole('link', { name: /back to home/i })).toBeInTheDocument();
  });

  it('verifies email successfully and displays success state', async () => {
    const verifySpy = vi.spyOn(client, 'verifyEmail').mockResolvedValue({
      message: 'email verified successfully',
    });

    window.history.replaceState(null, '', '/verify-email?token=valid_token_123');
    render(
      <Router>
        <VerifyEmailPage />
      </Router>
    );

    expect(screen.getByRole('status', { name: 'Verifying your email' })).toBeInTheDocument();

    await waitFor(() => {
      expect(verifySpy).toHaveBeenCalledWith('valid_token_123');
      expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Email Verified!');
    });

    expect(screen.getByRole('link', { name: /continue to cloudvitta/i })).toBeInTheDocument();
  });

  it('handles 410 expired token and provides resend functionality', async () => {
    const user = userEvent.setup();
    vi.spyOn(client, 'verifyEmail').mockRejectedValue(
      new ApiError({
        type: 'https://cloudvitta.dev/errors/verification-expired',
        title: 'Gone',
        status: 410,
        detail: 'The verification token has expired',
        instance: '/api/v1/auth/verify-email',
      })
    );
    const resendSpy = vi.spyOn(client, 'resendVerification').mockResolvedValue({
      message: 'verification email sent',
    });

    window.history.replaceState(null, '', '/verify-email?token=expired_token');
    render(
      <Router>
        <VerifyEmailPage />
      </Router>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Verification Link Expired');
    });

    const emailInput = screen.getByPlaceholderText('you@example.com');
    await user.type(emailInput, 'user@example.com');
    await user.click(screen.getByRole('button', { name: /send new verification link/i }));

    expect(resendSpy).toHaveBeenCalledWith('user@example.com');
    await waitFor(() => {
      expect(screen.getByText('A fresh verification link has been sent if this address is registered.')).toBeInTheDocument();
    });
  });

  it('handles 409 already consumed token state', async () => {
    vi.spyOn(client, 'verifyEmail').mockRejectedValue(
      new ApiError({
        type: 'https://cloudvitta.dev/errors/conflict',
        title: 'Conflict',
        status: 409,
        detail: 'This verification link has already been used',
        instance: '/api/v1/auth/verify-email',
      })
    );

    window.history.replaceState(null, '', '/verify-email?token=consumed_token');
    render(
      <Router>
        <VerifyEmailPage />
      </Router>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Already Verified');
      expect(screen.getByRole('link', { name: /go to cloudvitta/i })).toBeInTheDocument();
    });
  });

  it('handles 404 invalid token state', async () => {
    vi.spyOn(client, 'verifyEmail').mockRejectedValue(
      new ApiError({
        type: 'https://cloudvitta.dev/errors/not-found',
        title: 'Not Found',
        status: 404,
        detail: 'The verification token was not found',
        instance: '/api/v1/auth/verify-email',
      })
    );

    window.history.replaceState(null, '', '/verify-email?token=nonexistent_token');
    render(
      <Router>
        <VerifyEmailPage />
      </Router>
    );

    await waitFor(() => {
      expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Verification Failed');
      expect(screen.getByRole('link', { name: /back to home/i })).toBeInTheDocument();
    });
  });
});
