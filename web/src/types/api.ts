import type { paths, definitions, operations, external } from './api-generated';

// Base OpenAPI paths and definitions export
export type { paths, definitions, operations, external };

// Schema definitions shorthand
export type Schemas = definitions;

// Auth Endpoints
export type LoginRequest = definitions['rest.LoginRequest'];
export type LoginResponse = definitions['rest.LoginResponse'];

export type SignupRequest = definitions['rest.SignupRequest'];
export type SignupResponse = definitions['rest.SignupResponse'];

export type RefreshRequest = definitions['rest.RefreshRequest'];
export type RefreshResponse = definitions['rest.RefreshResponse'];

export type LogoutRequest = definitions['rest.LogoutRequest'];
export type LogoutResponse = definitions['rest.LogoutResponse'];

// Provider Status Endpoints
export type ProviderStatusResponse = definitions['rest.ProviderStatusResponse'];
export type CategoryStatusResponse = definitions['rest.CategoryStatusResponse'];
export type DLQStatusResponse = definitions['rest.DLQStatusResponse'];
export type ProviderStatusWarningResponse = definitions['rest.ProviderStatusWarningResponse'];

// Calculation & Comparison Endpoints
export type CalculateRequestBody = definitions['rest.CalculateRequestBody'];
export type CalculateResponse = definitions['rest.CalculateResponse'];
export type CalculateProviderResult = definitions['rest.CalculateProviderResult'];
export type CalculateCategoryResult = definitions['rest.CalculateCategoryResult'];

export type ComputeComparisonResponse = definitions['rest.ComputeComparisonResponse'];
export type StorageComparisonResponse = definitions['rest.StorageComparisonResponse'];
export type NetworkComparisonResponse = definitions['rest.NetworkComparisonResponse'];
export type DatabaseComparisonResponse = definitions['rest.DatabaseComparisonResponse'];
export type DatabaseNoSQLComparisonResponse = definitions['rest.DatabaseNoSQLComparisonResponse'];
export type KubernetesComparisonResponse = definitions['rest.KubernetesComparisonResponse'];
export type ServerlessComparisonResponse = definitions['rest.ServerlessComparisonResponse'];

// Common Result and Price Details
export type PriceDetail = definitions['rest.PriceDetail'];
export type ProviderWarning = definitions['rest.ProviderWarning'];

// RFC 7807 Error Definition
export interface RFC7807ProblemDetails {
  type: string;
  title: string;
  status: number;
  detail: string;
  instance: string;
  invalid_params?: Array<{ name: string; reason: string }>;
  trace_id?: string;
}
