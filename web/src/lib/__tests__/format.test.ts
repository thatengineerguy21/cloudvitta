// web/src/lib/__tests__/format.test.ts
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { formatRelativeTime, formatProviderName } from '../format';

describe('formatRelativeTime', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-08-28T12:00:00Z'));
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('formats timestamps less than 1 hour ago', () => {
    // 30 minutes ago
    expect(formatRelativeTime('2026-08-28T11:30:00Z')).toBe('0h ago');
    // 0 minutes ago
    expect(formatRelativeTime('2026-08-28T12:00:00Z')).toBe('0h ago');
  });

  it('formats timestamps between 1 and 24 hours ago', () => {
    // 1 hour ago
    expect(formatRelativeTime('2026-08-28T11:00:00Z')).toBe('1h ago');
    // 4 hours ago
    expect(formatRelativeTime('2026-08-28T08:00:00Z')).toBe('4h ago');
    // 23 hours ago
    expect(formatRelativeTime('2026-08-27T13:00:00Z')).toBe('23h ago');
  });

  it('formats timestamps greater than 24 hours ago as days', () => {
    // 24 hours ago
    expect(formatRelativeTime('2026-08-27T12:00:00Z')).toBe('1d ago');
    // 48 hours ago
    expect(formatRelativeTime('2026-08-26T12:00:00Z')).toBe('2d ago');
    // 8 days ago
    expect(formatRelativeTime('2026-08-20T12:00:00Z')).toBe('8d ago');
  });

  it('handles undefined, empty string, invalid, or future dates gracefully', () => {
    expect(formatRelativeTime(undefined)).toBe('Age unknown');
    expect(formatRelativeTime('')).toBe('Age unknown');
    expect(formatRelativeTime('invalid-date')).toBe('Age unknown');
    // Future date (negative diff)
    expect(formatRelativeTime('2026-08-28T13:00:00Z')).toBe('Age unknown');
  });
});

describe('formatProviderName', () => {
  it('formats known provider identifiers correctly', () => {
    expect(formatProviderName('aws')).toBe('AWS');
    expect(formatProviderName('azure')).toBe('Azure');
    expect(formatProviderName('gcp')).toBe('GCP');
    expect(formatProviderName('oracle')).toBe('Oracle');
    expect(formatProviderName('ibm')).toBe('IBM Cloud');
    expect(formatProviderName('alibaba')).toBe('Alibaba');
    expect(formatProviderName('digitalocean')).toBe('DigitalOcean');
  });

  it('falls back to uppercase for unknown provider strings or UNKNOWN for empty input', () => {
    expect(formatProviderName('unknown-provider')).toBe('UNKNOWN-PROVIDER');
    expect(formatProviderName(undefined)).toBe('UNKNOWN');
    expect(formatProviderName('')).toBe('UNKNOWN');
  });
});
