import { describe, it, expect } from 'vitest';
import { ApiError, getErrorMessage } from '../errors';
import type { RFC7807ProblemDetails } from '../../types';

describe('ApiError', () => {
  it('instantiates correctly with RFC 7807 problem details', () => {
    const problem: RFC7807ProblemDetails = {
      type: 'https://cloudvitta.dev/errors/invalid-credentials',
      title: 'Invalid Credentials',
      status: 401,
      detail: 'Invalid email or password',
      instance: '/api/v1/auth/login',
      invalid_params: [{ name: 'password', reason: 'too short' }],
      trace_id: 'trace_12345',
    };

    const error = new ApiError(problem);

    expect(error).toBeInstanceOf(Error);
    expect(error).toBeInstanceOf(ApiError);
    expect(error.name).toBe('ApiError');
    expect(error.message).toBe('Invalid email or password');
    expect(error.status).toBe(401);
    expect(error.type).toBe('https://cloudvitta.dev/errors/invalid-credentials');
    expect(error.title).toBe('Invalid Credentials');
    expect(error.detail).toBe('Invalid email or password');
    expect(error.instance).toBe('/api/v1/auth/login');
    expect(error.invalidParams).toEqual([{ name: 'password', reason: 'too short' }]);
    expect(error.traceId).toBe('trace_12345');
    expect(error.rawProblem).toEqual(problem);
  });

  it('falls back to title if detail is missing', () => {
    const problem: RFC7807ProblemDetails = {
      type: 'https://cloudvitta.dev/errors/internal',
      title: 'Internal Server Error',
      status: 500,
      detail: '',
      instance: '/api/v1/prices/compute',
    };

    const error = new ApiError(problem);
    expect(error.message).toBe('Internal Server Error');
  });

  it('falls back to default message if both detail and title are empty', () => {
    const problem: RFC7807ProblemDetails = {
      type: '',
      title: '',
      status: 500,
      detail: '',
      instance: '',
    };

    const error = new ApiError(problem);
    expect(error.message).toBe('An unexpected API error occurred');
  });
});

describe('getErrorMessage', () => {
  it('returns specific detail for ApiError with status 401', () => {
    const err = new ApiError({
      type: 'https://cloudvitta.dev/errors/unauthorized',
      title: 'Unauthorized',
      status: 401,
      detail: 'Token expired',
      instance: '/api/v1/prices/compute',
    });
    expect(getErrorMessage(err)).toBe('Token expired');
  });

  it('returns default message for ApiError 401 when detail is empty', () => {
    const err = new ApiError({
      type: 'https://cloudvitta.dev/errors/unauthorized',
      title: 'Unauthorized',
      status: 401,
      detail: '',
      instance: '/api/v1/prices/compute',
    });
    expect(getErrorMessage(err)).toBe('Invalid email or password. Please verify your credentials.');
  });

  it('returns specific detail for 409 Conflict', () => {
    const err = new ApiError({
      type: 'https://cloudvitta.dev/errors/conflict',
      title: 'Conflict',
      status: 409,
      detail: 'Email already registered',
      instance: '/api/v1/auth/signup',
    });
    expect(getErrorMessage(err)).toBe('Email already registered');
  });

  it('returns rate limit message for 429 Too Many Requests', () => {
    const err = new ApiError({
      type: 'https://cloudvitta.dev/errors/rate-limit',
      title: 'Too Many Requests',
      status: 429,
      detail: 'Rate limit exceeded',
      instance: '/api/v1/calculate',
    });
    expect(getErrorMessage(err)).toBe('Rate limit exceeded. Please wait a moment before trying again.');
  });

  it('returns API unavailable message for 502/503', () => {
    const err502 = new ApiError({
      type: 'https://cloudvitta.dev/errors/bad-gateway',
      title: 'Bad Gateway',
      status: 502,
      detail: '',
      instance: '/api/v1/prices/storage',
    });
    const err503 = new ApiError({
      type: 'https://cloudvitta.dev/errors/unavailable',
      title: 'Service Unavailable',
      status: 503,
      detail: '',
      instance: '/api/v1/prices/storage',
    });
    expect(getErrorMessage(err502)).toBe('The CloudVitta API is temporarily unreachable. Please retry shortly.');
    expect(getErrorMessage(err503)).toBe('The CloudVitta API is temporarily unreachable. Please retry shortly.');
  });

  it('handles standard Error and AbortError', () => {
    const abortErr = new Error('The operation was aborted');
    abortErr.name = 'AbortError';
    expect(getErrorMessage(abortErr)).toBe('Request timed out. The server took too long to respond.');

    const regularErr = new Error('Custom error');
    expect(getErrorMessage(regularErr)).toBe('Custom error');
  });

  it('handles unknown error objects', () => {
    expect(getErrorMessage('some raw string')).toBe('An unexpected system error occurred. Please try again.');
    expect(getErrorMessage(null)).toBe('An unexpected system error occurred. Please try again.');
  });
});
