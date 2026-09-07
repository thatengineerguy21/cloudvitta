import { ApiError } from './errors';
import type {
  RFC7807ProblemDetails,
  AuthTokens,
  Provider,
  McpJsonRpcRequest,
  McpJsonRpcResponse,
  McpToolsListResult,
  McpCallToolResult,
} from '../types';
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
  VerifyEmailRequest,
  VerifyEmailResponse,
  ResendVerificationRequest,
  ResendVerificationResponse,
  ProviderStatusResponse,
  CatalogSummaryResponse,
  CatalogInstancesResponse,
  CalculateRequestBody,
  CalculateResponse,

  ComputeComparisonResponse,
  StorageComparisonResponse,
  NetworkComparisonResponse,
  DatabaseComparisonResponse,
  DatabaseNoSQLComparisonResponse,
  KubernetesComparisonResponse,
  ServerlessComparisonResponse,
  ComputeQueryParams,
  ComputeCatalogQueryParams,
  StorageQueryParams,

  NetworkQueryParams,
  DatabaseQueryParams,
  DatabaseNoSQLQueryParams,
  KubernetesQueryParams,
  ServerlessQueryParams,
  QueryParamValue,
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
 * Safely parses either direct JSON or Server-Sent Events (SSE) data stream payload.
 */
export function parseSseOrJson<T>(rawText: string): T {
  const lines = rawText.split(/\r?\n/);
  const dataLines: string[] = [];
  for (const line of lines) {
    if (line.startsWith('data:')) {
      dataLines.push(line.slice(5).trim());
    }
  }
  if (dataLines.length > 0) {
    return JSON.parse(dataLines.join('\n')) as T;
  }
  return JSON.parse(rawText) as T;
}

/**
 * Combines timeout signal with any caller-supplied AbortSignal without discarding user cancellation or leaking listeners.
 */
export function combineSignals(
  timeoutSignal: AbortSignal,
  callerSignal?: AbortSignal | null
): { signal: AbortSignal; cleanup: () => void } {
  if (!callerSignal) {
    return { signal: timeoutSignal, cleanup: () => {} };
  }

  // If AbortSignal.any is natively available, use it (no listener cleanup required)
  if (typeof AbortSignal !== 'undefined' && 'any' in AbortSignal && typeof AbortSignal.any === 'function') {
    return {
      signal: AbortSignal.any([timeoutSignal, callerSignal]),
      cleanup: () => {},
    };
  }

  // Fallback for environments without AbortSignal.any
  const controller = new AbortController();
  if (timeoutSignal.aborted) {
    controller.abort(timeoutSignal.reason);
    return { signal: controller.signal, cleanup: () => {} };
  }
  if (callerSignal.aborted) {
    controller.abort(callerSignal.reason);
    return { signal: controller.signal, cleanup: () => {} };
  }

  const onTimeoutAbort = () => controller.abort(timeoutSignal.reason);
  const onCallerAbort = () => controller.abort(callerSignal.reason);

  timeoutSignal.addEventListener('abort', onTimeoutAbort, { once: true });
  callerSignal.addEventListener('abort', onCallerAbort, { once: true });

  const cleanup = () => {
    timeoutSignal.removeEventListener('abort', onTimeoutAbort);
    callerSignal.removeEventListener('abort', onCallerAbort);
  };

  return { signal: controller.signal, cleanup };
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

  // Configure request timeout signal and combine with caller signal without leaking listeners
  const timeoutController = new AbortController();
  const timeoutId = setTimeout(() => timeoutController.abort(), timeoutMs);
  const { signal: activeSignal, cleanup: cleanupSignals } = combineSignals(
    timeoutController.signal,
    callerSignal
  );

  const url = `${BASE_URL}${endpoint}`;

  try {
    const response = await fetch(url, {
      ...restOptions,
      headers,
      signal: activeSignal,
    });

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
        // Strip raw HTML from non-JSON error responses (e.g. reverse proxy 404 pages)
        const isHtml = contentType.includes('text/html') || rawText.trimStart().startsWith('<');
        const cleanDetail = isHtml
          ? `Server returned HTTP ${response.status} (${response.statusText || 'Error'})`
          : rawText || `Server returned status ${response.status}`;
        problemDetails = {
          type: 'https://cloudvitta.dev/errors/http-error',
          title: response.statusText || 'HTTP Error',
          status: response.status,
          detail: cleanDetail,
          instance: endpoint,
        };
      }

      throw new ApiError(problemDetails);
    }

    if (response.status === 204) {
      return {} as T;
    }

    const contentType = response.headers.get('content-type') || '';
    if (contentType.includes('text/event-stream')) {
      const rawText = await response.text();
      return parseSseOrJson<T>(rawText);
    }

    return (await response.json()) as T;
  } catch (err: unknown) {
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
  } finally {
    clearTimeout(timeoutId);
    cleanupSignals();
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

  verifyEmail: (data: VerifyEmailRequest, options?: RequestOptions) =>
    apiFetch<VerifyEmailResponse>('/api/v1/auth/verify-email', {
      method: 'POST',
      body: JSON.stringify(data),
      skipAuth: true,
      ...options,
    }),

  resendVerification: (data: ResendVerificationRequest, options?: RequestOptions) =>
    apiFetch<ResendVerificationResponse>('/api/v1/auth/resend-verification', {
      method: 'POST',
      body: JSON.stringify(data),
      skipAuth: true,
      ...options,
    }),
};

export async function verifyEmail(token: string, options?: RequestOptions): Promise<VerifyEmailResponse> {
  return authApi.verifyEmail({ token }, options);
}

export async function resendVerification(email: string, options?: RequestOptions): Promise<ResendVerificationResponse> {
  return authApi.resendVerification({ email }, options);
}

// Typed Provider Status API
export const providerApi = {
  getStatus: (provider: Provider, options?: RequestOptions) =>
    apiFetch<ProviderStatusResponse>(`/api/v1/providers/${encodeURIComponent(provider)}/status`, {
      method: 'GET',
      ...options,
    }),
};

/**
 * Builds comparison endpoint URLs with filtered query parameters.
 */
export function buildComparisonUrl(
  path: string,
  query: Record<string, QueryParamValue>
): string {
  return `${path}?${new URLSearchParams(cleanQueryParams(query))}`;
}

// Typed Comparison and Calculator APIs
export const pricesApi = {
  getCompute: (query: ComputeQueryParams, options?: RequestOptions) =>
    apiFetch<ComputeComparisonResponse>(buildComparisonUrl('/api/v1/prices/compute', query), options),

  getStorage: (query: StorageQueryParams, options?: RequestOptions) =>
    apiFetch<StorageComparisonResponse>(buildComparisonUrl('/api/v1/prices/storage', query), options),

  getNetwork: (query: NetworkQueryParams, options?: RequestOptions) =>
    apiFetch<NetworkComparisonResponse>(buildComparisonUrl('/api/v1/prices/network', query), options),

  getDatabase: (query: DatabaseQueryParams, options?: RequestOptions) =>
    apiFetch<DatabaseComparisonResponse>(buildComparisonUrl('/api/v1/prices/database', query), options),

  getDatabaseNoSQL: (query: DatabaseNoSQLQueryParams, options?: RequestOptions) =>
    apiFetch<DatabaseNoSQLComparisonResponse>(buildComparisonUrl('/api/v1/prices/database-nosql', query), options),

  getKubernetes: (query: KubernetesQueryParams, options?: RequestOptions) =>
    apiFetch<KubernetesComparisonResponse>(buildComparisonUrl('/api/v1/prices/kubernetes', query), options),

  getServerless: (query: ServerlessQueryParams, options?: RequestOptions) =>
    apiFetch<ServerlessComparisonResponse>(buildComparisonUrl('/api/v1/prices/serverless', query), options),
};

export const calculateApi = {
  calculate: (body: CalculateRequestBody, options?: RequestOptions) =>
    apiFetch<CalculateResponse>('/api/v1/calculate', {
      ...options,
      method: 'POST',
      body: JSON.stringify(body),
    }),
};

export const catalogApi = {
  getComputeSummary: (options?: RequestOptions) =>
    apiFetch<CatalogSummaryResponse>('/api/v1/catalog/compute/summary', options),

  getComputeInstances: (query?: ComputeCatalogQueryParams, options?: RequestOptions) =>
    apiFetch<CatalogInstancesResponse>(
      buildComparisonUrl('/api/v1/catalog/compute/instances', (query || {}) as Record<string, QueryParamValue>),
      options
    ),
};

export const DEFAULT_MCP_TOOL_ARGUMENTS: Record<string, Record<string, unknown>> = {
  compare_compute: {
    region: 'us-east-1',
    vcpu: 4,
    ram_gb: 16,
    family: 'general_purpose',
    currency: 'USD',
  },
  compare_storage: {
    region: 'us-east-1',
    size_gb: 1000,
    storage_class: 'standard',
    currency: 'USD',
  },
  compare_network: {
    region: 'us-east-1',
    egress_gb: 5000,
    transfer_type: 'internet_egress',
    currency: 'USD',
  },
  compare_database: {
    region: 'us-east-1',
    engine: 'postgresql',
    vcpu: 4,
    ram_gb: 16,
    storage_gb: 100,
    currency: 'USD',
  },
  compare_database_nosql: {
    region: 'us-east-1',
    data_model: 'key_value',
    storage_gb: 50,
    read_units: 100,
    write_units: 50,
    currency: 'USD',
  },
  compare_kubernetes: {
    region: 'us-east-1',
    tier: 'standard',
    cluster_topology: 'zonal',
    currency: 'USD',
  },
  compare_serverless: {
    region: 'us-east-1',
    architecture: 'x86_64',
    memory_mb: 512,
    invocations_monthly: 1000000,
    execution_duration_ms: 200,
    currency: 'USD',
  },
  calculate_workload: {
    region: 'us-east-1',
    currency: 'USD',
    compute: {
      vcpu: 4,
      ram_gb: 16,
      family: 'general_purpose',
    },
    storage: {
      size_gb: 500,
      storage_class: 'standard',
    },
  },
  get_provider_status: {
    provider: 'aws',
  },
  get_compute_catalog: {
    provider: 'aws',
    region: 'us-east-1',
    category: 'general_purpose',
    limit: 10,
  },
};

export const mcpApi = {
  execute: <T = unknown>(body: McpJsonRpcRequest, options?: RequestOptions) =>
    apiFetch<McpJsonRpcResponse<T>>('/mcp', {
      ...options,
      method: 'POST',
      body: JSON.stringify(body),
      headers: {
        'Content-Type': 'application/json',
        Accept: 'application/json, text/event-stream',
        ...((options?.headers as Record<string, string>) || {}),
      },
    }),

  listTools: (options?: RequestOptions) =>
    mcpApi.execute<McpToolsListResult>(
      {
        jsonrpc: '2.0',
        id: 'tools-list-' + Date.now(),
        method: 'tools/list',
      },
      options
    ),

  callTool: (toolName: string, args: Record<string, unknown>, options?: RequestOptions) =>
    mcpApi.execute<McpCallToolResult>(
      {
        jsonrpc: '2.0',
        id: 'tool-call-' + Date.now(),
        method: 'tools/call',
        params: {
          name: toolName,
          arguments: args,
        },
      },
      options
    ),
};


function cleanQueryParams(params: Record<string, QueryParamValue>): Record<string, string> {
  const result: Record<string, string> = {};
  Object.entries(params).forEach(([key, val]) => {
    if (val !== undefined && val !== null && val !== '') {
      result[key] = String(val);
    }
  });
  return result;
}

