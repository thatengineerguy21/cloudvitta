export type Theme = 'light' | 'dark';

export interface NavItem {
  label: string;
  href: string;
}

export type MatchQuality = 'exact' | 'close' | 'approximate';

export type HonestyStatus = 'stale' | 'partial' | 'anomaly';
