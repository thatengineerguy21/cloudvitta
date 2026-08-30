// web/src/lib/format.ts
export function formatRelativeTime(isoDate?: string): string {
  if (!isoDate) return 'Age unknown';
  const diffMs = Date.now() - new Date(isoDate).getTime();
  if (Number.isNaN(diffMs) || diffMs < 0) return 'Age unknown';
  const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
  if (diffHours < 24) return `${diffHours}h ago`;
  const diffDays = Math.floor(diffHours / 24);
  return `${diffDays}d ago`;
}

export function formatProviderName(provider?: string): string {
  if (!provider) return 'UNKNOWN';
  switch (provider.toLowerCase()) {
    case 'aws':
      return 'AWS';
    case 'azure':
      return 'Azure';
    case 'gcp':
      return 'GCP';
    case 'oracle':
      return 'Oracle';
    case 'ibm':
      return 'IBM Cloud';
    case 'alibaba':
      return 'Alibaba';
    case 'digitalocean':
      return 'DigitalOcean';
    default:
      return provider.toUpperCase();
  }
}
