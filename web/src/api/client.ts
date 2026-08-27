import { ApiError } from './errors';
import type { RFC7807ProblemDetails, AuthTokens } from '../types';
import {
  setAuthTokenProvider,
  getAuthTokenProvider,
  executeSingleFlightRefresh,
} from './refreshQueue';
import type {
  LoginRequest,
  LoginResponse,
  SignupRequest,
  SignupResponse,
  RefreshRequest,
  RefreshResponse,
  LogoutRequest,
  LogoutResponse,
  ProviderStatusResponse,
  CalculateRequestBody,
  CalculateResponse,
  ComputeComparisonResponse,
  StorageComparisonResponse,
  NetworkComparisonResponse,
  DatabaseComparisonResponse,
  DatabaseNoSQLComparisonResponse,
  KubernetesComparisonResponse,
  ServerlessComparisonResponse,
} from '../types/api';

export { setAuthTokenProvider };

const BASE_URL = import.meta.env.VITE_API_BASE_URL || '';
const DEFAULT_TIMEOUT_MS = 15000;

export interface RequestOptions extends RequestInit {
  timeoutMs?: number;
  skipAuth?: boolean;
  _retry?: boolean;
}

/**
 * Maps login or refresh API responses into canonical domain AuthTokens.
 */
export function mapAuthTokens(res: LoginResponse | RefreshResponse): AuthTokens {
  return {
    accessToken: res.access_token || '',
    refreshToken: res.refresh_token || '',
    tokenType: res.token_type || 'Bearer',
    expiresIn: res.expires_in || 900,
  };
}

/**
 * Combines timeout signal with any caller-supplied AbortSignal without discarding user cancellation.
 */
function combineSignals(timeoutSignal: AbortSignal, callerSignal?: AbortSignal | null): AbortSignal {
  if (!callerSignal) return timeoutSignal;
  if (typeof AbortSignal !== 'undefined' && 'any' in AbortSignal && typeof AbortSignal.any === 'function') {
    return AbortSignal.any([timeoutSignal, callerSignal]);
  }
  const controller = new AbortController();
  const onAbort = () => controller.abort();
  if (timeoutSignal.aborted || callerSignal.aborted) {
    controller.abort();
    return controller.signal;
  }
  timeoutSignal.addEventListener('abort', onAbort, { once: true });
  callerSignal.addEventListener('abort', onAbort, { once: true });
  return controller.signal;
}

/**
 * Core type-safe fetch wrapper with RFC 7807 parsing and 401 single-flight mutex refresh.
 */
export async function apiFetch<T>(
  endpoint: string,
  options: RequestOptions = {}
): Promise<T> {
  const {
    timeoutMs = DEFAULT_TIMEOUT_MS,
    skipAuth = false,
    _retry = false,
    signal: callerSignal,
    headers: customHeaders = {},
    ...restOptions
  } = options;

  const tokenProvider = getAuthTokenProvider();
  const accessToken = tokenProvider?.getAccessToken();

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    Accept: 'application/json, application/problem+json',
    ...(customHeaders as Record<string, string>),
  };

  if (accessToken && !skipAuth) {
    headers['Authorization'] = `Bearer ${accessToken}`;
  }

  // Configure request timeout signal and combine with caller signal
  const timeoutController = new AbortController();
  const timeoutId = setTimeout(() => timeoutController.abort(), timeoutMs);
  const activeSignal = combineSignals(timeoutController.signal, callerSignal);

  const url = `${BASE_URL}${endpoint}`;

  try {
    const response = await fetch(url, {
      ...restOptions,
      headers,
      signal: activeSignal,
    });

    clearTimeout(timeoutId);

    // Handle 401 Unauthorized with singleflight refresh
    if (response.status === 401 && !_retry && !skipAuth && tokenProvider?.getRefreshToken()) {
      try {
        const newAccessToken = await executeSingleFlightRefresh(async (rToken, idempKey) => {
          const refreshRes = await apiFetch<RefreshResponse>('/api/v1/auth/refresh', {
            method: 'POST',
            body: JSON.stringify({
              refresh_token: rToken,
              idempotency_key: idempKey,
            }),
            skipAuth: true,
            _retry: true,
          });

          return mapAuthTokens(refreshRes);
        });

        // Replay original request with the new access token
        return apiFetch<T>(endpoint, {
          ...options,
          _retry: true,
          headers: {
            ...customHeaders,
            Authorization: `Bearer ${newAccessToken}`,
          },
        });
      } catch {
        // Refresh failed: proceed below to throw original 401 error
      }
    }

    if (!response.ok) {
      let problemDetails: RFC7807ProblemDetails;
      const contentType = response.headers.get('content-type') || '';

      if (contentType.includes('problem+json') || contentType.includes('application/json')) {
        try {
          problemDetails = await response.json();
        } catch {
          problemDetails = {
            type: 'https://cloudvitta.dev/errors/parse-failure',
            title: response.statusText || 'Error',
            status: response.status,
            detail: `Server returned HTTP ${response.status}`,
            instance: endpoint,
          };
        }
      } else {
        const rawText = await response.text();
        problemDetails = {
          type: 'https://cloudvitta.dev/errors/http-error',
          title: response.statusText || 'HTTP Error',
          status: response.status,
          detail: rawText || `Server returned status ${response.status}`,
          instance: endpoint,
        };
      }

      throw new ApiError(problemDetails);
    }

    if (response.status === 204) {
      return {} as T;
    }

    return (await response.json()) as T;
  } catch (err: unknown) {
    clearTimeout(timeoutId);
    if (err instanceof ApiError) {
      throw err;
    }
    if ((err as Error)?.name === 'AbortError') {
      // Check if abort was triggered by timeout or caller
      if (timeoutController.signal.aborted) {
        throw new ApiError({
          type: 'https://cloudvitta.dev/errors/timeout',
          title: 'Request Timeout',
          status: 408,
          detail: `The request timed out after ${timeoutMs}ms`,
          instance: endpoint,
        });
      }
      // Re-throw caller abort as standard AbortError
      throw err;
    }
    throw new ApiError({
      type: 'https://cloudvitta.dev/errors/network-error',
      title: 'Network Error',
      status: 0,
      detail: (err as Error)?.message || 'Failed to communicate with the CloudVitta API server',
      instance: endpoint,
    });
  }
}

// Typed Auth API Endpoints
export const authApi = {
  login: (data: LoginRequest) =>
    apiFetch<LoginResponse>('/api/v1/auth/login', {
      method: 'POST',
      body: JSON.stringify(data),
      skipAuth: true,
    }),

  signup: (data: SignupRequest) =>
    apiFetch<SignupResponse>('/api/v1/auth/signup', {
      method: 'POST',
      body: JSON.stringify(data),
      skipAuth: true,
    }),

  refresh: (data: RefreshRequest) =>
    apiFetch<RefreshResponse>('/api/v1/auth/refresh', {
      method: 'POST',
      body: JSON.stringify(data),
      skipAuth: true,
    }),

  logout: (data: LogoutRequest) =>
    apiFetch<LogoutResponse>('/api/v1/auth/logout', {
      method: 'POST',
      body: JSON.stringify(data),
      skipAuth: true,
    }),
};

// Typed Provider Status API
export const providerApi = {
  getStatus: (provider: string) =>
    apiFetch<ProviderStatusResponse>(`/api/v1/providers/${encodeURIComponent(provider)}/status`, {
      method: 'GET',
    }),
};

// Typed Comparison and Calculator APIs
export const pricesApi = {
  getCompute: (query: Record<string, string | number | boolean | undefined>) =>
    apiFetch<ComputeComparisonResponse>(`/api/v1/prices/compute?${new URLSearchParams(cleanQueryParams(query))}`),

  getStorage: (query: Record<string, string | number | boolean | undefined>) =>
    apiFetch<StorageComparisonResponse>(`/api/v1/prices/storage?${new URLSearchParams(cleanQueryParams(query))}`),

  getNetwork: (query: Record<string, string | number | boolean | undefined>) =>
    apiFetch<NetworkComparisonResponse>(`/api/v1/prices/network?${new URLSearchParams(cleanQueryParams(query))}`),

  getDatabase: (query: Record<string, string | number | boolean | undefined>) =>
    apiFetch<DatabaseComparisonResponse>(`/api/v1/prices/database?${new URLSearchParams(cleanQueryParams(query))}`),

  getDatabaseNoSQL: (query: Record<string, string | number | boolean | undefined>) =>
    apiFetch<DatabaseNoSQLComparisonResponse>(`/api/v1/prices/database-nosql?${new URLSearchParams(cleanQueryParams(query))}`),

  getKubernetes: (query: Record<string, string | number | boolean | undefined>) =>
    apiFetch<KubernetesComparisonResponse>(`/api/v1/prices/kubernetes?${new URLSearchParams(cleanQueryParams(query))}`),

  getServerless: (query: Record<string, string | number | boolean | undefined>) =>
    apiFetch<ServerlessComparisonResponse>(`/api/v1/prices/serverless?${new URLSearchParams(cleanQueryParams(query))}`),
};

export const calculateApi = {
  calculate: (body: CalculateRequestBody) =>
    apiFetch<CalculateResponse>('/api/v1/calculate', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
};

function cleanQueryParams(params: Record<string, string | number | boolean | undefined>): Record<string, string> {
  const result: Record<string, string> = {};
  Object.entries(params).forEach(([key, val]) => {
    if (val !== undefined && val !== null && val !== '') {
      result[key] = String(val);
    }
  });
  return result;
}
