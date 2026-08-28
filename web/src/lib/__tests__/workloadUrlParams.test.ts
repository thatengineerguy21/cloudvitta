// web/src/lib/__tests__/workloadUrlParams.test.ts
import { describe, it, expect } from 'vitest';
import {
  DEFAULT_WORKLOAD_STATE,
  parseWorkloadFromUrl,
  serializeWorkloadToUrl,
  buildCalculateRequestBody,
  WorkloadFormState,
} from '../workloadUrlParams';

describe('workloadUrlParams serialization and deserialization', () => {
  it('parses empty query string to default state', () => {
    const state = parseWorkloadFromUrl('');
    expect(state).toEqual(DEFAULT_WORKLOAD_STATE);
  });

  it('serializes and deserializes full 7-category state correctly', () => {
    const fullState: WorkloadFormState = {
      region: 'eu-west',
      currency: 'EUR',
      strict_family: false,
      compute: {
        enabled: true,
        vcpu: 8,
        ram_gb: 32,
        family: 'c6g',
      },
      storage: {
        enabled: true,
        size_gb: 500,
        storage_class: 'infrequent_access',
        iops: 1000,
      },
      network: {
        enabled: true,
        egress_gb: 2000,
        transfer_type: 'inter_region',
      },
      database_rdbms: {
        enabled: true,
        engine: 'mysql',
        vcpu: 4,
        ram_gb: 16,
        storage_gb: 250,
        iops: 5000,
        multi_az: true,
        storage_family: 'gp3',
      },
      database_nosql: {
        enabled: true,
        data_model: 'key_value',
        pricing_mode: 'on_demand',
        read_units: 500,
        write_units: 200,
        storage_gb: 50,
        storage_class: 'standard',
        multi_region: true,
      },
      kubernetes: {
        enabled: true,
        tier: 'extended_support',
        cluster_topology: 'autopilot',
      },
      serverless: {
        enabled: true,
        architecture: 'arm64',
        tier: 'flex_consumption',
        requests_per_month: 5000000,
        memory_mb: 1024,
        execution_duration_ms: 150,
      },
    };

    const url = serializeWorkloadToUrl(fullState);
    expect(url).toContain('region=eu-west');
    expect(url).toContain('currency=EUR');
    expect(url).toContain('strict_family=false');
    expect(url).toContain('c_on=1');
    expect(url).toContain('c_vcpu=8');
    expect(url).toContain('s_on=1');
    expect(url).toContain('s_size=500');
    expect(url).toContain('n_on=1');
    expect(url).toContain('db_on=1');
    expect(url).toContain('db_engine=mysql');
    expect(url).toContain('nosql_on=1');
    expect(url).toContain('nosql_model=key_value');
    expect(url).toContain('k8s_on=1');
    expect(url).toContain('k8s_topo=autopilot');
    expect(url).toContain('fn_on=1');
    expect(url).toContain('fn_arch=arm64');

    const parsed = parseWorkloadFromUrl(url);
    expect(parsed.region).toBe('eu-west');
    expect(parsed.currency).toBe('EUR');
    expect(parsed.strict_family).toBe(false);
    expect(parsed.compute).toEqual(fullState.compute);
    expect(parsed.storage).toEqual(fullState.storage);
    expect(parsed.network).toEqual(fullState.network);
    expect(parsed.database_rdbms).toEqual(fullState.database_rdbms);
    expect(parsed.database_nosql).toEqual(fullState.database_nosql);
    expect(parsed.kubernetes).toEqual(fullState.kubernetes);
    expect(parsed.serverless).toEqual(fullState.serverless);
  });

  it('buildCalculateRequestBody includes only enabled categories with canonical keys', () => {
    const customState: WorkloadFormState = {
      ...DEFAULT_WORKLOAD_STATE,
      compute: { enabled: false, vcpu: 2, ram_gb: 4, family: '' },
      storage: { enabled: true, size_gb: 200, storage_class: 'archive' },
      database_rdbms: {
        enabled: true,
        engine: 'postgresql',
        vcpu: 4,
        ram_gb: 16,
        storage_gb: 100,
        iops: 3000,
        multi_az: false,
        storage_family: 'gp3',
      },
    };

    const body = buildCalculateRequestBody(customState);

    expect(body.region).toBe('us-east');
    expect(body.currency).toBe('USD');
    expect(body.compute).toBeUndefined();
    expect(body.network).toBeUndefined();
    expect(body.database_nosql).toBeUndefined();
    expect(body.kubernetes).toBeUndefined();
    expect(body.serverless).toBeUndefined();

    // Canonical RDBMS key (ADR 0032 compliance)
    expect(body.database_rdbms).toBeDefined();
    expect(body.database_rdbms?.engine).toBe('postgresql');
    expect(body.database_rdbms?.vcpu).toBe(4);
    expect(body.database).toBeUndefined(); // Must NOT populate alias

    // Storage
    expect(body.storage).toBeDefined();
    expect(body.storage?.size_gb).toBe(200);
    expect(body.storage?.storage_class).toBe('archive');
  });
});
