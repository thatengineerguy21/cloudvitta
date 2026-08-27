// web/src/auth/AuthContext.tsx
import React, { createContext, useContext, useState, useCallback, useMemo, useEffect, useRef } from 'react';
import type { AuthUser, AuthTokens } from '../types';
import { authApi, setAuthTokenProvider, mapAuthTokens } from '../api/client';

export interface AuthContextType {
  user: AuthUser | null;
  tokens: AuthTokens | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (email: string, password: string) => Promise<void>;
  signup: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [tokens, setTokens] = useState<AuthTokens | null>(null);
  const [isLoading, setIsLoading] = useState<boolean>(false);

  // Keep a ref to the latest tokens to avoid stale closures in setAuthTokenProvider
  const tokensRef = useRef<AuthTokens | null>(tokens);
  tokensRef.current = tokens;

  // Initialize and keep token provider connected
  useEffect(() => {
    setAuthTokenProvider({
      getAccessToken: () => tokensRef.current?.accessToken ?? null,
      getRefreshToken: () => tokensRef.current?.refreshToken ?? null,
      setTokens: (newTokens: AuthTokens | null) => {
        tokensRef.current = newTokens;
        setTokens(newTokens);
        if (!newTokens) {
          setUser(null);
        }
      },
    });
  }, []);

  const login = useCallback(async (email: string, password: string) => {
    setIsLoading(true);
    try {
      const response = await authApi.login({ email, password });
      const newTokens = mapAuthTokens(response);
      tokensRef.current = newTokens;
      setTokens(newTokens);
      setUser({ email });
    } finally {
      setIsLoading(false);
    }
  }, []);

  const signup = useCallback(async (email: string, password: string) => {
    setIsLoading(true);
    try {
      await authApi.signup({ email, password });
      // Automatically log the user in following successful registration
      const loginResponse = await authApi.login({ email, password });
      const newTokens = mapAuthTokens(loginResponse);
      tokensRef.current = newTokens;
      setTokens(newTokens);
      setUser({ email });
    } finally {
      setIsLoading(false);
    }
  }, []);

  const logout = useCallback(async () => {
    const currentRefreshToken = tokensRef.current?.refreshToken;
    tokensRef.current = null;
    setTokens(null);
    setUser(null);
    if (currentRefreshToken) {
      try {
        await authApi.logout({ refresh_token: currentRefreshToken });
      } catch {
        // Suppress errors during logout to guarantee clean client-side state reset
      }
    }
  }, []);

  const value = useMemo<AuthContextType>(
    () => ({
      user,
      tokens,
      isAuthenticated: !!tokens?.accessToken,
      isLoading,
      login,
      signup,
      logout,
    }),
    [user, tokens, isLoading, login, signup, logout]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
};

export const useAuth = (): AuthContextType => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};
