// web/src/components/auth/AuthModal.tsx
import React, { useState, useEffect, useCallback } from 'react';
import { useAuth } from '../../auth/AuthContext';
import { loginSchema, signupSchema, formatZodErrors } from '../../lib/validation/auth';
import { getErrorMessage } from '../../api/errors';
import { X, Lock, Mail, Loader2, AlertCircle } from 'lucide-react';

export interface AuthModalProps {
  isOpen: boolean;
  onClose: () => void;
  initialMode?: 'login' | 'signup';
}

export const AuthModal: React.FC<AuthModalProps> = ({
  isOpen,
  onClose,
  initialMode = 'login',
}) => {
  const { login, signup, isLoading } = useAuth();
  const [mode, setMode] = useState<'login' | 'signup'>(initialMode);
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [apiError, setApiError] = useState<string | null>(null);

  // Sync modal mode when initialMode changes
  useEffect(() => {
    setMode(initialMode);
  }, [initialMode]);

  // Reset form state on open/close
  useEffect(() => {
    if (isOpen) {
      setEmail('');
      setPassword('');
      setConfirmPassword('');
      setFieldErrors({});
      setApiError(null);
    }
  }, [isOpen, mode]);

  // Handle Esc key
  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      if (e.key === 'Escape' && !isLoading) {
        onClose();
      }
    },
    [isLoading, onClose]
  );

  useEffect(() => {
    if (isOpen) {
      window.addEventListener('keydown', handleKeyDown);
      return () => window.removeEventListener('keydown', handleKeyDown);
    }
  }, [isOpen, handleKeyDown]);

  if (!isOpen) return null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setFieldErrors({});
    setApiError(null);

    const isLogin = mode === 'login';
    const formConfig = isLogin
      ? {
          schema: loginSchema,
          data: { email, password },
          action: () => login(email, password),
        }
      : {
          schema: signupSchema,
          data: { email, password, confirmPassword },
          action: () => signup(email, password),
        };

    const validation = formConfig.schema.safeParse(formConfig.data);
    if (!validation.success) {
      setFieldErrors(formatZodErrors(validation.error.issues));
      return;
    }

    try {
      await formConfig.action();
      onClose();
    } catch (err) {
      setApiError(getErrorMessage(err));
    }
  };

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-none"
      role="dialog"
      aria-modal="true"
      aria-labelledby="auth-modal-title"
    >
      {/* Rectilinear 0px Bento Card */}
      <div className="w-full max-w-md bg-surface-card border border-border-default shadow-none p-6 sm:p-8 relative">
        {/* Close Button */}
        <button
          onClick={onClose}
          disabled={isLoading}
          className="absolute top-4 right-4 p-1.5 text-text-secondary hover:text-text-primary hover:bg-surface-raised transition-colors disabled:opacity-50 focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none"
          aria-label="Close dialog"
        >
          <X className="w-4 h-4" />
        </button>

        {/* Modal Header */}
        <div className="mb-6">
          <div className="inline-flex items-center space-x-2 text-xs uppercase font-bold tracking-widest text-border-accent mb-2">
            <Lock className="w-3.5 h-3.5" />
            <span>{mode === 'login' ? 'Account Sign In' : 'Create Account'}</span>
          </div>
          <h2 id="auth-modal-title" className="font-display text-2xl font-bold text-text-primary">
            {mode === 'login' ? 'Sign In' : 'Create an Account'}
          </h2>
          <p className="text-xs text-text-secondary mt-1">
            Sign in to access higher request limits, save workloads, and configure multi-cloud environments.
          </p>
        </div>

        {/* API Error Alert */}
        {apiError && (
          <div className="mb-4 p-3 bg-surface-raised border-l-2 border-status-anomaly flex items-start space-x-2.5">
            <AlertCircle className="w-4 h-4 text-status-anomaly shrink-0 mt-0.5" />
            <p className="text-xs font-semibold text-status-anomaly">{apiError}</p>
          </div>
        )}

        {/* Form Inputs */}
        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label htmlFor="auth-email" className="block text-xs font-bold uppercase tracking-wider text-text-secondary mb-1">
              Email Address
            </label>
            <div className="relative">
              <input
                id="auth-email"
                type="email"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                disabled={isLoading}
                placeholder="name@company.com"
                className="w-full px-3 py-2 bg-surface-raised border border-border-default text-text-primary text-sm focus:outline-none focus:border-border-accent transition-colors pl-9 rounded-none"
              />
              <Mail className="w-4 h-4 text-text-secondary absolute left-3 top-2.5" />
            </div>
            {fieldErrors.email && (
              <p className="text-xs text-status-anomaly mt-1 font-semibold">{fieldErrors.email}</p>
            )}
          </div>

          <div>
            <label htmlFor="auth-password" className="block text-xs font-bold uppercase tracking-wider text-text-secondary mb-1">
              Password
            </label>
            <div className="relative">
              <input
                id="auth-password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                disabled={isLoading}
                placeholder="••••••••"
                className="w-full px-3 py-2 bg-surface-raised border border-border-default text-text-primary text-sm focus:outline-none focus:border-border-accent transition-colors pl-9 rounded-none"
              />
              <Lock className="w-4 h-4 text-text-secondary absolute left-3 top-2.5" />
            </div>
            {fieldErrors.password && (
              <p className="text-xs text-status-anomaly mt-1 font-semibold">{fieldErrors.password}</p>
            )}
          </div>

          {mode === 'signup' && (
            <div>
              <label htmlFor="auth-confirm-password" className="block text-xs font-bold uppercase tracking-wider text-text-secondary mb-1">
                Confirm Password
              </label>
              <div className="relative">
                <input
                  id="auth-confirm-password"
                  type="password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  disabled={isLoading}
                  placeholder="••••••••"
                  className="w-full px-3 py-2 bg-surface-raised border border-border-default text-text-primary text-sm focus:outline-none focus:border-border-accent transition-colors pl-9 rounded-none"
                />
                <Lock className="w-4 h-4 text-text-secondary absolute left-3 top-2.5" />
              </div>
              {fieldErrors.confirmPassword && (
                <p className="text-xs text-status-anomaly mt-1 font-semibold">
                  {fieldErrors.confirmPassword}
                </p>
              )}
            </div>
          )}

          {/* Submit Action */}
          <button
            type="submit"
            disabled={isLoading}
            className="w-full py-2.5 px-4 bg-border-accent text-white hover:opacity-90 font-bold text-xs uppercase tracking-widest transition-opacity flex items-center justify-center space-x-2 rounded-none disabled:opacity-50 focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none"
          >
            {isLoading && <Loader2 className="w-3.5 h-3.5 animate-spin" />}
            <span>{mode === 'login' ? 'Sign In' : 'Create Account'}</span>
          </button>
        </form>

        {/* Mode Switch Toggle */}
        <div className="mt-6 pt-4 border-t border-border-default text-center">
          {mode === 'login' ? (
            <p className="text-xs text-text-secondary">
              Need higher limits?{' '}
              <button
                type="button"
                onClick={() => setMode('signup')}
                className="font-bold text-border-accent hover:underline ml-1 uppercase tracking-wider focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none"
              >
                Sign Up
              </button>
            </p>
          ) : (
            <p className="text-xs text-text-secondary">
              Already have an account?{' '}
              <button
                type="button"
                onClick={() => setMode('login')}
                className="font-bold text-border-accent hover:underline ml-1 uppercase tracking-wider focus-visible:ring-1 focus-visible:ring-border-accent focus-visible:outline-none"
              >
                Sign In
              </button>
            </p>
          )}
        </div>
      </div>
    </div>
  );
};
