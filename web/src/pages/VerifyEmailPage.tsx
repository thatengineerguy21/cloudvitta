// web/src/pages/VerifyEmailPage.tsx
import React, { useEffect, useState, useRef } from 'react';
import { useLocation, Link } from '../router';
import { verifyEmail, resendVerification } from '../api/client';
import { ApiError } from '../api/errors';
import { CheckCircle2, AlertCircle, Clock, Loader2, ArrowRight, RefreshCw, Mail } from 'lucide-react';

export const VerifyEmailPage: React.FC = () => {
  const { search } = useLocation();
  const searchParams = new URLSearchParams(search);
  const token = searchParams.get('token');

  const [loading, setLoading] = useState(true);
  const [success, setSuccess] = useState(false);
  const [errorStatus, setErrorStatus] = useState<number | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  // For resend sub-form on expired token
  const [resendEmail, setResendEmail] = useState('');
  const [resending, setResending] = useState(false);
  const [resendSent, setResendSent] = useState(false);
  const [resendError, setResendError] = useState<string | null>(null);

  const verificationAttempted = useRef(false);

  useEffect(() => {
    if (verificationAttempted.current) return;
    verificationAttempted.current = true;

    if (!token || token.trim() === '') {
      setLoading(false);
      setErrorStatus(400);
      setErrorMessage('No verification token provided in the URL link.');
      return;
    }

    const performVerification = async () => {
      try {
        await verifyEmail(token.trim());
        setSuccess(true);
      } catch (err: unknown) {
        if (err instanceof ApiError) {
          setErrorStatus(err.status);
          setErrorMessage(err.detail || err.message);
        } else {
          setErrorStatus(500);
          setErrorMessage((err as Error)?.message || 'Failed to verify email address');
        }
      } finally {
        setLoading(false);
      }
    };

    void performVerification();
  }, [token]);

  const handleResendSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!resendEmail.trim() || resending) return;

    setResending(true);
    setResendError(null);
    setResendSent(false);

    try {
      await resendVerification(resendEmail.trim());
      setResendSent(true);
    } catch (err: unknown) {
      if (err instanceof ApiError) {
        setResendError(err.detail || err.message);
      } else {
        setResendError((err as Error)?.message || 'Failed to send verification email');
      }
    } finally {
      setResending(false);
    }
  };

  return (
    <div className="min-h-[60vh] flex items-center justify-center px-4">
      <div className="w-full max-w-md bg-surface-card border border-border-default/80 rounded-2xl shadow-xl p-6 sm:p-8 text-center relative">
        {loading && (
          <div className="py-8" role="status" aria-label="Verifying your email">
            <Loader2 className="w-10 h-10 animate-spin text-brand mx-auto mb-4" />
            <h2 className="font-display text-xl font-bold text-text-primary mb-2">
              Verifying Your Email
            </h2>
            <p className="text-text-secondary text-sm">
              Please wait while we validate your verification token...
            </p>
          </div>
        )}

        {!loading && success && (
          <div role="status" aria-label="Email verified successfully">
            <div className="w-14 h-14 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 rounded-2xl mx-auto flex items-center justify-center mb-6">
              <CheckCircle2 className="w-8 h-8" />
            </div>

            <h1 className="font-display text-2xl sm:text-3xl font-bold text-text-primary mb-3">
              Email Verified!
            </h1>

            <p className="text-text-secondary text-sm sm:text-base leading-relaxed mb-6">
              Your email address has been verified successfully. Your CloudVitta account is now fully active.
            </p>

            <Link
              to="/"
              className="w-full py-3 px-4 rounded-xl bg-brand text-white font-medium text-sm hover:opacity-90 transition-opacity flex items-center justify-center gap-2"
            >
              <span>Continue to CloudVitta</span>
              <ArrowRight className="w-4 h-4" />
            </Link>
          </div>
        )}

        {!loading && !success && (
          <div role="alert" aria-label="Verification failed">
            {errorStatus === 410 ? (
              <>
                <div className="w-14 h-14 bg-amber-500/10 text-amber-600 dark:text-amber-400 rounded-2xl mx-auto flex items-center justify-center mb-6">
                  <Clock className="w-8 h-8" />
                </div>
                <h1 className="font-display text-2xl sm:text-3xl font-bold text-text-primary mb-3">
                  Verification Link Expired
                </h1>
                <p className="text-text-secondary text-sm mb-6">
                  This verification link has expired. Verification links are valid for 30 minutes.
                  Enter your email below to receive a new link.
                </p>

                {resendSent ? (
                  <div className="p-3 bg-emerald-500/10 border border-emerald-500/30 rounded-xl flex items-center gap-2 text-xs sm:text-sm text-emerald-600 dark:text-emerald-400 text-left mb-6">
                    <CheckCircle2 className="w-4 h-4 flex-shrink-0" />
                    <span>A fresh verification link has been sent if this address is registered.</span>
                  </div>
                ) : (
                  <form onSubmit={handleResendSubmit} className="space-y-3 mb-6 text-left">
                    <div>
                      <label htmlFor="resend-email" className="block text-xs font-semibold uppercase tracking-wider text-text-secondary mb-1">
                        Email Address
                      </label>
                      <div className="relative">
                        <input
                          id="resend-email"
                          type="email"
                          required
                          value={resendEmail}
                          onChange={(e) => setResendEmail(e.target.value)}
                          placeholder="you@example.com"
                          className="w-full pl-9 pr-3 py-2 text-sm bg-surface-raised border border-border-default rounded-xl text-text-primary placeholder:text-text-secondary/60 focus:outline-none focus:ring-2 focus:ring-brand"
                        />
                        <Mail className="w-4 h-4 text-text-secondary absolute left-3 top-1/2 -translate-y-1/2" />
                      </div>
                    </div>

                    {resendError && (
                      <p className="text-xs text-red-500">{resendError}</p>
                    )}

                    <button
                      type="submit"
                      disabled={resending || !resendEmail.trim()}
                      className="w-full py-2.5 px-4 rounded-xl bg-brand text-white font-medium text-sm hover:opacity-90 transition-opacity flex items-center justify-center gap-2 disabled:opacity-50"
                    >
                      {resending ? (
                        <>
                          <Loader2 className="w-4 h-4 animate-spin" />
                          <span>Sending...</span>
                        </>
                      ) : (
                        <>
                          <RefreshCw className="w-4 h-4" />
                          <span>Send New Verification Link</span>
                        </>
                      )}
                    </button>
                  </form>
                )}
              </>
            ) : errorStatus === 409 ? (
              <>
                <div className="w-14 h-14 bg-blue-500/10 text-blue-600 dark:text-blue-400 rounded-2xl mx-auto flex items-center justify-center mb-6">
                  <CheckCircle2 className="w-8 h-8" />
                </div>
                <h1 className="font-display text-2xl sm:text-3xl font-bold text-text-primary mb-3">
                  Already Verified
                </h1>
                <p className="text-text-secondary text-sm mb-6">
                  This verification link has already been used. Your email is already verified and your account is ready.
                </p>
                <Link
                  to="/"
                  className="w-full py-3 px-4 rounded-xl bg-brand text-white font-medium text-sm hover:opacity-90 transition-opacity flex items-center justify-center gap-2"
                >
                  <span>Go to CloudVitta</span>
                  <ArrowRight className="w-4 h-4" />
                </Link>
              </>
            ) : (
              <>
                <div className="w-14 h-14 bg-red-500/10 text-red-600 dark:text-red-400 rounded-2xl mx-auto flex items-center justify-center mb-6">
                  <AlertCircle className="w-8 h-8" />
                </div>
                <h1 className="font-display text-2xl sm:text-3xl font-bold text-text-primary mb-3">
                  Verification Failed
                </h1>
                <p className="text-text-secondary text-sm mb-6">
                  {errorMessage || 'The verification link is invalid or has expired.'}
                </p>
                <Link
                  to="/"
                  className="w-full py-2.5 px-4 rounded-xl border border-border-default font-medium text-sm text-text-primary hover:bg-surface-raised transition-colors inline-block"
                >
                  Back to Home
                </Link>
              </>
            )}
          </div>
        )}
      </div>
    </div>
  );
};
