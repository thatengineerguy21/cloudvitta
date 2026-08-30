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

// Domain Attribute Types for Workloads
export type ComputeAttributes = definitions['domain.ComputeAttributes'];
export type StorageAttributes = definitions['domain.StorageAttributes'];
export type NetworkAttributes = definitions['domain.NetworkAttributes'];
export type DatabaseRDBMSAttributes = definitions['domain.DatabaseRDBMSAttributes'];
export type DatabaseNoSQLAttributes = definitions['domain.DatabaseNoSQLAttributes'];
export type KubernetesAttributes = definitions['domain.KubernetesAttributes'];
export type ServerlessWorkloadPayload = definitions['rest.ServerlessWorkloadPayload'];
export type ServerlessRateAttributes = definitions['domain.ServerlessRateAttributes'];

// Query Parameter Interfaces for Comparison APIs
export type QueryParamValue = string | number | boolean | undefined | null;

export interface BaseQueryParams {
  region?: string;
  region_group?: string;
  currency?: string;
  provider?: string;
  [key: string]: QueryParamValue;
}

export interface ComputeQueryParams extends BaseQueryParams {
  vcpu?: number | string;
  ram_gb?: number | string;
  family?: string;
  strict_family?: boolean | string;
}

export interface StorageQueryParams extends BaseQueryParams {
  size_gb?: number | string;
  storage_class?: 'standard' | 'infrequent_access' | 'archive' | string;
}

export interface NetworkQueryParams extends BaseQueryParams {
  egress_gb?: number | string;
  transfer_type?: 'internet_egress' | 'intra_region' | 'inter_region' | string;
}

export interface DatabaseQueryParams extends BaseQueryParams {
  engine?: 'postgresql' | 'mysql' | 'sqlserver' | string;
  vcpu?: number | string;
  ram_gb?: number | string;
  storage_gb?: number | string;
  iops?: number | string;
  multi_az?: boolean | string;
  storage_family?: string;
}

export interface DatabaseNoSQLQueryParams extends BaseQueryParams {
  data_model?: 'document' | 'key_value' | 'wide_column' | 'graph' | 'multi_model' | string;
  pricing_mode?: 'provisioned' | 'on_demand' | 'serverless' | string;
  read_units?: number | string;
  write_units?: number | string;
  storage_gb?: number | string;
  storage_class?: string;
  multi_region?: boolean | string;
}

export interface KubernetesQueryParams extends BaseQueryParams {
  tier?: 'free' | 'standard' | 'extended_support' | string;
  cluster_topology?: 'zonal' | 'regional' | 'autopilot' | string;
}

export interface ServerlessQueryParams extends BaseQueryParams {
  architecture?: 'x86_64' | 'arm64' | string;
  cpu_architecture?: string;
  tier?: 'consumption' | 'flex_consumption' | '1st_gen' | '2nd_gen' | string;
  requests_per_month?: number | string;
  memory_mb?: number | string;
  execution_duration_ms?: number | string;
  duration_ms?: number | string;
}

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

