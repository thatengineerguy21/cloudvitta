import type { RFC7807ProblemDetails } from '../types/api';

export class ApiError extends Error {
  readonly status: number;
  readonly type: string;
  readonly title: string;
  readonly detail: string;
  readonly instance: string;
  readonly invalidParams?: Array<{ name: string; reason: string }>;
  readonly traceId?: string;
  readonly rawProblem: RFC7807ProblemDetails;

  constructor(problem: RFC7807ProblemDetails) {
    super(problem.detail || problem.title || 'An unexpected API error occurred');
    this.name = 'ApiError';
    this.status = problem.status;
    this.type = problem.type;
    this.title = problem.title;
    this.detail = problem.detail;
    this.instance = problem.instance;
    this.invalidParams = problem.invalid_params;
    this.traceId = problem.trace_id;
    this.rawProblem = problem;

    // Restore prototype chain for instanceof checks in ES6/V8
    Object.setPrototypeOf(this, ApiError.prototype);
  }
}

/**
 * Maps any thrown error into a clean, human-readable string suitable for UI alerts.
 */
export function getErrorMessage(error: unknown): string {
  if (error instanceof ApiError) {
    if (error.status === 401) {
      return error.detail || 'Invalid email or password. Please verify your credentials.';
    }
    if (error.status === 409) {
      return error.detail || 'An account with this email address already exists.';
    }
    if (error.status === 429) {
      return 'Rate limit exceeded. Please wait a moment before trying again.';
    }
    // Upstream provider fetch failures return HTTP 200 with warnings[].code: "fetch_failed" (12-API-CONTRACT.md).
    // A 502/503 from this API indicates that the CloudVitta API gateway or service itself is unreachable.
    if (error.status === 502 || error.status === 503) {
      return 'The CloudVitta API is temporarily unreachable. Please retry shortly.';
    }
    return error.detail || error.title || `API error (${error.status})`;
  }

  if (error instanceof Error) {
    if (error.name === 'AbortError' || error.name === 'TimeoutError') {
      return 'Request timed out. The server took too long to respond.';
    }
    return error.message;
  }

  return 'An unexpected system error occurred. Please try again.';
}
