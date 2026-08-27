// web/src/types/index.ts

// Re-export all API schema types
export * from './api';

// UI Theme types
export type Theme = 'light' | 'dark';

export interface NavItem {
  label: string;
  href: string;
}

// 4-State Honesty Vocabulary Match Quality (PRD §10.2, 08-CONSISTENCY-RULES.md)
// Note: 'none' represents unmatchable candidate states; UI badges render 'exact' | 'close' | 'approximate'.
export type MatchQuality = 'exact' | 'close' | 'approximate' | 'none';

// Supported Cloud Providers (7 Curated Providers)
export type Provider =
  | 'aws'
  | 'azure'
  | 'gcp'
  | 'oracle'
  | 'ibm'
  | 'alibaba'
  | 'digitalocean';

// Supported Service Categories (7 Curated Categories)
export type ServiceCategory =
  | 'compute'
  | 'storage'
  | 'network'
  | 'database'
  | 'database-nosql'
  | 'kubernetes'
  | 'serverless';

// Canonical Warning Codes (08-CONSISTENCY-RULES.md §2)
export type WarningCode =
  | 'engine_mismatch_excluded'
  | 'architecture_unsupported_excluded'
  | 'cluster_topology_unspecified'
  | 'category_not_supported'
  | 'not_yet_ingested'
  | 'pricing_anomaly_flagged'
  | 'stale_pricing_data'
  | 'non_usd_currency_unsupported'
  | 'no_match'
  | 'fetch_failed'
  | 'no_data_available';

// Honesty Status Signals
export type HonestyStatus = 'stale' | 'partial' | 'anomaly';

// User and Session Types
export interface AuthUser {
  id?: string;
  email: string;
}

export interface AuthTokens {
  accessToken: string;
  refreshToken: string;
  tokenType: string;
  expiresIn: number;
}
