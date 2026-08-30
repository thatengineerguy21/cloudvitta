import type { AuthTokens } from '../types';

export interface TokenProvider {
  getAccessToken: () => string | null;
  getRefreshToken: () => string | null;
  setTokens: (tokens: AuthTokens | null) => void;
}

let activeTokenProvider: TokenProvider | null = null;
let isRefreshing = false;
let refreshQueue: Array<{
  resolve: (accessToken: string) => void;
  reject: (error: unknown) => void;
}> = [];

export function setAuthTokenProvider(provider: TokenProvider): void {
  activeTokenProvider = provider;
}

export function getAuthTokenProvider(): TokenProvider | null {
  return activeTokenProvider;
}

/**
 * Dispatches queued requests with the freshly minted access token.
 */
function processQueue(error: unknown, newAccessToken: string | null = null): void {
  refreshQueue.forEach((prom) => {
    if (error) {
      prom.reject(error);
    } else if (newAccessToken) {
      prom.resolve(newAccessToken);
    }
  });
  refreshQueue = [];
}

/**
 * Acquires the refresh mutex and executes a singleflight token rotation.
 */
export async function executeSingleFlightRefresh(
  refreshCall: (refreshToken: string, idempotencyKey: string) => Promise<AuthTokens>
): Promise<string> {
  const currentRefreshToken = activeTokenProvider?.getRefreshToken();

  if (!currentRefreshToken) {
    activeTokenProvider?.setTokens(null);
    throw new Error('No refresh token available');
  }

  // If a refresh is already in flight, enqueue and await resolution
  if (isRefreshing) {
    return new Promise<string>((resolve, reject) => {
      refreshQueue.push({ resolve, reject });
    });
  }

  isRefreshing = true;
  const idempotencyKey =
    typeof crypto !== 'undefined' && crypto.randomUUID
      ? crypto.randomUUID()
      : `idemp_${Date.now()}_${Math.random().toString(36).substring(2, 9)}`;

  try {
    const newTokens = await refreshCall(currentRefreshToken, idempotencyKey);
    activeTokenProvider?.setTokens(newTokens);
    processQueue(null, newTokens.accessToken);
    return newTokens.accessToken;
  } catch (refreshErr) {
    activeTokenProvider?.setTokens(null);
    processQueue(refreshErr, null);
    throw refreshErr;
  } finally {
    isRefreshing = false;
  }
}
