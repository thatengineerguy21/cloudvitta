import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  apiFetch,
  authApi,
  providerApi,
  pricesApi,
  calculateApi,
} from '../client';
import { setAuthTokenProvider } from '../refreshQueue';
import { ApiError } from '../errors';
import type { AuthTokens } from '../../types';

describe('apiFetch & client wrappers', () => {
  let mockTokens: AuthTokens | null = null;

  beforeEach(() => {
    mockTokens = null;
    setAuthTokenProvider({
      getAccessToken: vi.fn(() => mockTokens?.accessToken ?? null),
      getRefreshToken: vi.fn(() => mockTokens?.refreshToken ?? null),
      setTokens: vi.fn((tokens) => {
        mockTokens = tokens;
      }),
    });
    vi.restoreAllMocks();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('performs standard JSON GET request successfully', async () => {
    const mockData = { status: 'healthy', provider: 'aws' };
    const globalFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: vi.fn().mockResolvedValue(mockData),
    });
    vi.stubGlobal('fetch', globalFetch);

    const result = await apiFetch<typeof mockData>('/api/v1/providers/aws/status');

    expect(result).toEqual(mockData);
    expect(globalFetch).toHaveBeenCalledWith(
      '/api/v1/providers/aws/status',
      expect.objectContaining({
        headers: expect.objectContaining({
          'Content-Type': 'application/json',
          Accept: 'application/json, application/problem+json',
        }),
      })
    );
  });

  it('injects Authorization Bearer header when token exists and skipAuth is false', async () => {
    mockTokens = {
      accessToken: 'test_access_jwt',
      refreshToken: 'test_refresh_token',
      tokenType: 'Bearer',
      expiresIn: 900,
    };

    const globalFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: vi.fn().mockResolvedValue({ ok: true }),
    });
    vi.stubGlobal('fetch', globalFetch);

    await apiFetch('/api/v1/calculate', { method: 'POST', body: '{}' });

    expect(globalFetch).toHaveBeenCalledWith(
      '/api/v1/calculate',
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: 'Bearer test_access_jwt',
        }),
      })
    );
  });

  it('does not inject Authorization header when skipAuth is true', async () => {
    mockTokens = {
      accessToken: 'test_access_jwt',
      refreshToken: 'test_refresh_token',
      tokenType: 'Bearer',
      expiresIn: 900,
    };

    const globalFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: vi.fn().mockResolvedValue({ access_token: 'new_jwt' }),
    });
    vi.stubGlobal('fetch', globalFetch);

    await apiFetch('/api/v1/auth/login', {
      method: 'POST',
      body: '{}',
      skipAuth: true,
    });

    const callHeaders = globalFetch.mock.calls[0][1].headers;
    expect(callHeaders.Authorization).toBeUndefined();
  });

  it('parses RFC 7807 problem details into ApiError on HTTP error status', async () => {
    const problemPayload = {
      type: 'https://cloudvitta.dev/errors/validation',
      title: 'Validation Error',
      status: 400,
      detail: 'Invalid query parameter',
      instance: '/api/v1/prices/compute',
      invalid_params: [{ name: 'vcpu', reason: 'must be positive integer' }],
    };

    const globalFetch = vi.fn().mockResolvedValue({
      ok: false,
      status: 400,
      statusText: 'Bad Request',
      headers: new Headers({ 'content-type': 'application/problem+json' }),
      json: vi.fn().mockResolvedValue(problemPayload),
    });
    vi.stubGlobal('fetch', globalFetch);

    await expect(apiFetch('/api/v1/prices/compute')).rejects.toThrow(ApiError);

    try {
      await apiFetch('/api/v1/prices/compute');
    } catch (err) {
      const apiErr = err as ApiError;
      expect(apiErr.status).toBe(400);
      expect(apiErr.title).toBe('Validation Error');
      expect(apiErr.detail).toBe('Invalid query parameter');
      expect(apiErr.invalidParams).toEqual([{ name: 'vcpu', reason: 'must be positive integer' }]);
    }
  });

  it('handles 401 with automatic singleflight refresh and request replay', async () => {
    mockTokens = {
      accessToken: 'expired_jwt',
      refreshToken: 'valid_refresh',
      tokenType: 'Bearer',
      expiresIn: 900,
    };

    let fetchCount = 0;
    const globalFetch = vi.fn().mockImplementation((url: string) => {
      fetchCount++;
      if (url === '/api/v1/prices/compute' && fetchCount === 1) {
        // Initial request returns 401
        return Promise.resolve({
          ok: false,
          status: 401,
          statusText: 'Unauthorized',
          headers: new Headers({ 'content-type': 'application/problem+json' }),
          json: vi.fn().mockResolvedValue({
            type: 'https://cloudvitta.dev/errors/unauthorized',
            title: 'Unauthorized',
            status: 401,
            detail: 'Access token expired',
            instance: '/api/v1/prices/compute',
          }),
        });
      }

      if (url === '/api/v1/auth/refresh') {
        // Refresh request returns 200
        return Promise.resolve({
          ok: true,
          status: 200,
          headers: new Headers({ 'content-type': 'application/json' }),
          json: vi.fn().mockResolvedValue({
            access_token: 'fresh_rotated_jwt',
            refresh_token: 'fresh_rotated_refresh',
            token_type: 'Bearer',
            expires_in: 900,
          }),
        });
      }

      if (url === '/api/v1/prices/compute' && fetchCount === 3) {
        // Replayed request returns 200
        return Promise.resolve({
          ok: true,
          status: 200,
          headers: new Headers({ 'content-type': 'application/json' }),
          json: vi.fn().mockResolvedValue({ results: [{ sku_id: 't3.micro' }] }),
        });
      }

      return Promise.reject(new Error('Unexpected URL'));
    });
    vi.stubGlobal('fetch', globalFetch);

    const result = await apiFetch<{ results: Array<{ sku_id: string }> }>('/api/v1/prices/compute');

    expect(result.results[0].sku_id).toBe('t3.micro');
    expect(globalFetch).toHaveBeenCalledTimes(3);
  });

  it('handles 204 No Content response gracefully', async () => {
    const globalFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 204,
      headers: new Headers(),
    });
    vi.stubGlobal('fetch', globalFetch);

    const result = await apiFetch<unknown>('/api/v1/auth/logout', { method: 'POST' });
    expect(result).toEqual({});
  });

  it('wraps typed endpoint namespaces correctly', async () => {
    const globalFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: vi.fn().mockResolvedValue({ status: 'healthy' }),
    });
    vi.stubGlobal('fetch', globalFetch);

    await authApi.login({ email: 'user@test.com', password: 'password123' });
    expect(globalFetch).toHaveBeenCalledWith(
      '/api/v1/auth/login',
      expect.objectContaining({ method: 'POST' })
    );

    await authApi.signup({ email: 'user@test.com', password: 'password123' });
    expect(globalFetch).toHaveBeenCalledWith(
      '/api/v1/auth/signup',
      expect.objectContaining({ method: 'POST' })
    );

    await authApi.refresh({ refresh_token: 'r_token', idempotency_key: 'key' });
    expect(globalFetch).toHaveBeenCalledWith(
      '/api/v1/auth/refresh',
      expect.objectContaining({ method: 'POST' })
    );

    await authApi.logout({ refresh_token: 'r_token' });
    expect(globalFetch).toHaveBeenCalledWith(
      '/api/v1/auth/logout',
      expect.objectContaining({ method: 'POST' })
    );

    await providerApi.getStatus('aws');
    expect(globalFetch).toHaveBeenCalledWith(
      '/api/v1/providers/aws/status',
      expect.objectContaining({ method: 'GET' })
    );

    await pricesApi.getCompute({ vcpu: 4, ram_gb: 16 });
    expect(globalFetch).toHaveBeenCalledWith(
      '/api/v1/prices/compute?vcpu=4&ram_gb=16',
      expect.anything()
    );

    await pricesApi.getStorage({ size_gb: 100, storage_class: 'standard' });
    expect(globalFetch).toHaveBeenCalledWith(
      '/api/v1/prices/storage?size_gb=100&storage_class=standard',
      expect.anything()
    );

    await pricesApi.getNetwork({ egress_gb: 500, transfer_type: 'internet_egress' });
    expect(globalFetch).toHaveBeenCalledWith(
      '/api/v1/prices/network?egress_gb=500&transfer_type=internet_egress',
      expect.anything()
    );

    await pricesApi.getDatabase({ engine: 'postgresql', vcpu: 4, ram_gb: 16 });
    expect(globalFetch).toHaveBeenCalledWith(
      '/api/v1/prices/database?engine=postgresql&vcpu=4&ram_gb=16',
      expect.anything()
    );

    await pricesApi.getDatabaseNoSQL({ data_model: 'key_value', pricing_mode: 'on_demand' });
    expect(globalFetch).toHaveBeenCalledWith(
      '/api/v1/prices/database-nosql?data_model=key_value&pricing_mode=on_demand',
      expect.anything()
    );

    await pricesApi.getKubernetes({ tier: 'standard' });
    expect(globalFetch).toHaveBeenCalledWith(
      '/api/v1/prices/kubernetes?tier=standard',
      expect.anything()
    );

    await pricesApi.getServerless({ requests_per_month: 1000000 });
    expect(globalFetch).toHaveBeenCalledWith(
      '/api/v1/prices/serverless?requests_per_month=1000000',
      expect.anything()
    );

    await calculateApi.calculate({ compute: { vcpu: 4, ram_gb: 16 } });
    expect(globalFetch).toHaveBeenCalledWith(
      '/api/v1/calculate',
      expect.objectContaining({ method: 'POST' })
    );
  });

  it('respects caller-supplied AbortSignal without discarding user cancellation', async () => {
    const callerController = new AbortController();
    const globalFetch = vi.fn().mockImplementation((_url, init) => {
      return new Promise((_resolve, reject) => {
        const signal: AbortSignal = init.signal;
        if (signal.aborted) {
          const err = new Error('The operation was aborted');
          err.name = 'AbortError';
          reject(err);
          return;
        }
        signal.addEventListener('abort', () => {
          const err = new Error('The operation was aborted');
          err.name = 'AbortError';
          reject(err);
        });
      });
    });
    vi.stubGlobal('fetch', globalFetch);

    const promise = apiFetch('/api/v1/prices/compute', {
      signal: callerController.signal,
    });

    // Abort from caller side
    callerController.abort();

    await expect(promise).rejects.toThrow('The operation was aborted');
  });

  it('removes event listeners from caller AbortSignal on request completion in fallback combiner', async () => {
    const callerController = new AbortController();
    const addSpy = vi.spyOn(callerController.signal, 'addEventListener');
    const removeSpy = vi.spyOn(callerController.signal, 'removeEventListener');

    const globalFetch = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: vi.fn().mockResolvedValue({ status: 'healthy' }),
    });
    vi.stubGlobal('fetch', globalFetch);

    // Force fallback path by temporarily removing AbortSignal.any if present
    const originalAny = (AbortSignal as unknown as { any?: unknown }).any;
    delete (AbortSignal as unknown as { any?: unknown }).any;

    try {
      await apiFetch('/api/v1/providers/aws/status', {
        signal: callerController.signal,
      });

      expect(addSpy).toHaveBeenCalledWith('abort', expect.any(Function), { once: true });
      expect(removeSpy).toHaveBeenCalledWith('abort', expect.any(Function));
      expect(addSpy.mock.calls.length).toBe(removeSpy.mock.calls.length);
    } finally {
      if (originalAny) {
        (AbortSignal as unknown as { any?: unknown }).any = originalAny;
      }
    }
  });
});
