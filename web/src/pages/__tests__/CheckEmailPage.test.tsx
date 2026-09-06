import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Router } from '../../router';
import { CheckEmailPage } from '../CheckEmailPage';
import * as client from '../../api/client';
import { ApiError } from '../../api/errors';

describe('CheckEmailPage', () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it('renders check inbox heading and email from query param', () => {
    window.history.replaceState(null, '', '/check-email?email=test%40example.com');
    render(
      <Router>
        <CheckEmailPage />
      </Router>
    );

    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent('Check Your Inbox');
    expect(screen.getByText('test@example.com')).toBeInTheDocument();
  });

  it('renders default text when no email param is provided', () => {
    window.history.replaceState(null, '', '/check-email');
    render(
      <Router>
        <CheckEmailPage />
      </Router>
    );

    expect(screen.getByText('your email')).toBeInTheDocument();
  });

  it('handles resend verification trigger successfully', async () => {
    const user = userEvent.setup();
    const resendSpy = vi.spyOn(client, 'resendVerification').mockResolvedValue({
      message: 'verification email sent',
    });

    window.history.replaceState(null, '', '/check-email?email=test%40example.com');
    render(
      <Router>
        <CheckEmailPage />
      </Router>
    );

    const resendButton = screen.getByRole('button', { name: /resend verification email/i });
    await user.click(resendButton);

    expect(resendSpy).toHaveBeenCalledWith('test@example.com');
    await waitFor(() => {
      expect(
        screen.getByText('A new verification link has been sent if this address is registered and unverified.')
      ).toBeInTheDocument();
    });
  });

  it('handles resend error and displays message', async () => {
    const user = userEvent.setup();
    vi.spyOn(client, 'resendVerification').mockRejectedValue(
      new ApiError({
        type: 'https://cloudvitta.dev/errors/rate-limit-exceeded',
        title: 'Too Many Requests',
        status: 429,
        detail: 'Too many verification attempts',
        instance: '/api/v1/auth/resend-verification',
      })
    );

    window.history.replaceState(null, '', '/check-email?email=test%40example.com');
    render(
      <Router>
        <CheckEmailPage />
      </Router>
    );

    const resendButton = screen.getByRole('button', { name: /resend verification email/i });
    await user.click(resendButton);

    await waitFor(() => {
      expect(screen.getByText(/Rate limit exceeded/i)).toBeInTheDocument();
    });
  });
});
