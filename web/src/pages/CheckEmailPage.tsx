// web/src/pages/CheckEmailPage.tsx
import React, { useState } from 'react';
import { useLocation } from '../router';
import { resendVerification } from '../api/client';
import { getErrorMessage } from '../api/errors';
import { Mail, CheckCircle2, AlertCircle, ArrowLeft, RefreshCw, Loader2 } from 'lucide-react';
import { Link } from '../router';

export const CheckEmailPage: React.FC = () => {
  const { search } = useLocation();
  const searchParams = new URLSearchParams(search);
  const email = searchParams.get('email') || 'your email';

  const [resending, setResending] = useState(false);
  const [resendSuccess, setResendSuccess] = useState(false);
  const [resendError, setResendError] = useState<string | null>(null);

  const handleResend = async () => {
    if (resending) return;
    setResending(true);
    setResendSuccess(false);
    setResendError(null);

    try {
      await resendVerification(email);
      setResendSuccess(true);
    } catch (err) {
      setResendError(getErrorMessage(err));
    } finally {
      setResending(false);
    }
  };

  return (
    <div className="min-h-[60vh] flex items-center justify-center px-4">
      <div className="w-full max-w-md bg-surface-card border border-border-default/80 rounded-2xl shadow-xl p-6 sm:p-8 text-center relative">
        <div className="w-14 h-14 bg-brand/10 text-brand rounded-2xl mx-auto flex items-center justify-center mb-6">
          <Mail className="w-7 h-7" />
        </div>

        <h1 className="font-display text-2xl sm:text-3xl font-bold text-text-primary mb-3">
          Check Your Inbox
        </h1>

        <p className="text-text-secondary text-sm sm:text-base leading-relaxed mb-6">
          We sent a verification email to <span className="font-semibold text-text-primary">{email}</span>.
          Please click the link in that email to activate your account. The link expires in 30 minutes.
        </p>

        {resendSuccess && (
          <div
            role="status"
            className="mb-6 p-3 bg-emerald-500/10 border border-emerald-500/30 rounded-xl flex items-center gap-2 text-xs sm:text-sm text-emerald-600 dark:text-emerald-400 text-left"
          >
            <CheckCircle2 className="w-4 h-4 flex-shrink-0" />
            <span>A new verification link has been sent if this address is registered and unverified.</span>
          </div>
        )}

        {resendError && (
          <div
            role="alert"
            className="mb-6 p-3 bg-red-500/10 border border-red-500/30 rounded-xl flex items-center gap-2 text-xs sm:text-sm text-red-600 dark:text-red-400 text-left"
          >
            <AlertCircle className="w-4 h-4 flex-shrink-0" />
            <span>{resendError}</span>
          </div>
        )}

        <div className="space-y-3">
          <button
            type="button"
            onClick={handleResend}
            disabled={resending}
            className="w-full py-2.5 px-4 rounded-xl border border-border-default font-medium text-sm text-text-primary hover:bg-surface-raised transition-colors flex items-center justify-center gap-2 disabled:opacity-50"
          >
            {resending ? (
              <>
                <Loader2 className="w-4 h-4 animate-spin" />
                <span>Sending new link...</span>
              </>
            ) : (
              <>
                <RefreshCw className="w-4 h-4" />
                <span>Resend verification email</span>
              </>
            )}
          </button>

          <Link
            to="/"
            className="inline-flex items-center justify-center gap-1.5 text-xs sm:text-sm text-text-secondary hover:text-text-primary transition-colors py-2"
          >
            <ArrowLeft className="w-4 h-4" />
            <span>Back to Home</span>
          </Link>
        </div>
      </div>
    </div>
  );
};
