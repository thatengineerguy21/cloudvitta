// web/src/components/honesty/friendlyWarnings.ts
import type { WarningCode } from '../../types';

/**
 * Human-readable friendly titles for all 11 canonical CloudVitta warning codes.
 * Replaces developer-facing error codes with plain-English labels.
 */
export const FRIENDLY_WARNING_TITLES: Record<WarningCode, string> = {
  pricing_anomaly_flagged: 'Pricing Anomaly Detected',
  fetch_failed: 'Provider Data Unavailable',
  stale_pricing_data: 'Outdated Pricing',
  category_not_supported: 'Service Not Offered',
  not_yet_ingested: 'Data Ingestion Pending',
  no_match: 'No Direct Match Found',
  no_data_available: 'No Data Available',
  non_usd_currency_unsupported: 'Currency Conversion Unavailable',
  engine_mismatch_excluded: 'Excluded Engine Type',
  architecture_unsupported_excluded: 'Unsupported Architecture',
  cluster_topology_unspecified: 'Topology Not Specified',
};
