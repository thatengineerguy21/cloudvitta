import { describe, it, expect, vi, beforeEach } from 'vitest';
import {
  setAuthTokenProvider,
  getAuthTokenProvider,
  executeSingleFlightRefresh,
  type TokenProvider,
} from '../refreshQueue';
import type { AuthTokens } from '../../types';

describe('refreshQueue & executeSingleFlightRefresh', () => {
  let mockProvider: TokenProvider;
  let currentTokens: AuthTokens | null;

  beforeEach(() => {
    currentTokens = {
      accessToken: 'old_access_token',
      refreshToken: 'valid_refresh_token',
      tokenType: 'Bearer',
      expiresIn: 900,
    };

    mockProvider = {
      getAccessToken: vi.fn(() => currentTokens?.accessToken ?? null),
      getRefreshToken: vi.fn(() => currentTokens?.refreshToken ?? null),
      setTokens: vi.fn((newTokens: AuthTokens | null) => {
        currentTokens = newTokens;
      }),
    };

    setAuthTokenProvider(mockProvider);
  });

  it('manages active token provider', () => {
    expect(getAuthTokenProvider()).toBe(mockProvider);
  });

  it('throws and resets tokens if no refresh token is available', async () => {
    currentTokens = null;

    await expect(
      executeSingleFlightRefresh(async () => {
        return {
          accessToken: 'new_token',
          refreshToken: 'new_refresh',
          tokenType: 'Bearer',
          expiresIn: 900,
        };
      })
    ).rejects.toThrow('No refresh token available');

    expect(mockProvider.setTokens).toHaveBeenCalledWith(null);
  });

  it('deduplicates concurrent refresh requests into a single flight', async () => {
    let callCount = 0;
    const refreshFn = vi.fn(async (_rToken: string, _idempKey: string): Promise<AuthTokens> => {
      callCount++;
      // Simulate network latency
      await new Promise((resolve) => setTimeout(resolve, 50));
      return {
        accessToken: `new_access_token_${callCount}`,
        refreshToken: `new_refresh_token_${callCount}`,
        tokenType: 'Bearer',
        expiresIn: 900,
      };
    });

    // Fire 5 concurrent refresh requests
    const promises = [
      executeSingleFlightRefresh(refreshFn),
      executeSingleFlightRefresh(refreshFn),
      executeSingleFlightRefresh(refreshFn),
      executeSingleFlightRefresh(refreshFn),
      executeSingleFlightRefresh(refreshFn),
    ];

    const results = await Promise.all(promises);

    // Assert that refreshFn was called exactly ONCE
    expect(refreshFn).toHaveBeenCalledTimes(1);
    expect(refreshFn).toHaveBeenCalledWith('valid_refresh_token', expect.any(String));

    // All 5 callers receive the same new access token
    results.forEach((token) => {
      expect(token).toBe('new_access_token_1');
    });

    // Provider state is updated
    expect(mockProvider.setTokens).toHaveBeenCalledWith({
      accessToken: 'new_access_token_1',
      refreshToken: 'new_refresh_token_1',
      tokenType: 'Bearer',
      expiresIn: 900,
    });
  });

  it('purges tokens and rejects all queued promises when refresh fails', async () => {
    const refreshError = new Error('Token compromised / family revoked');
    const refreshFn = vi.fn(async () => {
      await new Promise((resolve) => setTimeout(resolve, 30));
      throw refreshError;
    });

    const promises = [
      executeSingleFlightRefresh(refreshFn),
      executeSingleFlightRefresh(refreshFn),
      executeSingleFlightRefresh(refreshFn),
    ];

    await expect(Promise.all(promises)).rejects.toThrow('Token compromised / family revoked');

    expect(refreshFn).toHaveBeenCalledTimes(1);
    expect(mockProvider.setTokens).toHaveBeenCalledWith(null);
  });
});
