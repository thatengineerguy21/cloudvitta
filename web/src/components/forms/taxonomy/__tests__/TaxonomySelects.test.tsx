import { render, screen } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import {
  REGION_OPTIONS,
  CURRENCY_OPTIONS,
  STORAGE_CLASS_OPTIONS,
  TRANSFER_TYPE_OPTIONS,
  DATABASE_ENGINE_OPTIONS,
  NOSQL_DATA_MODEL_OPTIONS,
  NOSQL_PRICING_MODE_OPTIONS,
  KUBERNETES_TIER_OPTIONS,
  CLUSTER_TOPOLOGY_OPTIONS,
  SERVERLESS_ARCH_OPTIONS,
  INSTANCE_FAMILY_OPTIONS,
  STORAGE_FAMILY_OPTIONS,
  SERVERLESS_TIER_OPTIONS,
} from '../options';
import {
  RegionSelect,
  CurrencySelect,
  StorageClassSelect,
  TransferTypeSelect,
  DatabaseEngineSelect,
  NoSQLDataModelSelect,
  NoSQLPricingModeSelect,
  KubernetesTierSelect,
  ClusterTopologySelect,
  ServerlessArchSelect,
  ServerlessTierSelect,
  InstanceFamilySelect,
  StorageFamilySelect,
} from '../TaxonomySelects';

describe('TaxonomySelects Components & Options Arrays', () => {
  it('exports complete canonical options with traceable backend values (Task 1 acceptance criterion)', () => {
    // 1. Region Group (18 canonical groups)
    expect(REGION_OPTIONS.map((o) => o.value)).toEqual([
      'us-east', 'us-central', 'us-west', 'ca-central', 'eu-west', 'eu-central',
      'eu-north', 'eu-south', 'ap-east', 'ap-south', 'ap-southeast', 'ap-northeast',
      'sa-east', 'me-central', 'me-south', 'il-central', 'af-south', 'global',
    ]);

    // 2. Currencies (20 reference rates)
    expect(CURRENCY_OPTIONS.map((o) => o.value)).toContain('USD');
    expect(CURRENCY_OPTIONS.map((o) => o.value)).toContain('EUR');
    expect(CURRENCY_OPTIONS.map((o) => o.value)).toContain('GBP');
    expect(CURRENCY_OPTIONS.map((o) => o.value)).toContain('INR');

    // 3. Storage Classes (3 canonical tiers)
    expect(STORAGE_CLASS_OPTIONS.map((o) => o.value)).toEqual([
      'standard', 'infrequent_access', 'archive',
    ]);

    // 4. Transfer Types (3 canonical types)
    expect(TRANSFER_TYPE_OPTIONS.map((o) => o.value)).toEqual([
      'internet_egress', 'intra_region', 'inter_region',
    ]);

    // 5. Database Engines (3 canonical relational engines)
    expect(DATABASE_ENGINE_OPTIONS.map((o) => o.value)).toEqual([
      'postgresql', 'mysql', 'sqlserver',
    ]);

    // 6. NoSQL Data Models (5 canonical models from internal/matching/nosqldatamodelmap)
    expect(NOSQL_DATA_MODEL_OPTIONS.map((o) => o.value)).toEqual([
      'document', 'key_value', 'wide_column', 'graph', 'multi_model',
    ]);

    // 7. NoSQL Pricing Modes (3 canonical modes from internal/domain/database_nosql.go)
    expect(NOSQL_PRICING_MODE_OPTIONS.map((o) => o.value)).toEqual([
      'provisioned', 'on_demand', 'serverless',
    ]);

    // 8. Kubernetes Tiers
    expect(KUBERNETES_TIER_OPTIONS.map((o) => o.value)).toEqual([
      'free', 'standard', 'extended_support',
    ]);

    // 9. Cluster Topologies
    expect(CLUSTER_TOPOLOGY_OPTIONS.map((o) => o.value)).toEqual([
      'regional', 'zonal', 'autopilot',
    ]);

    // 10. Serverless Architectures
    expect(SERVERLESS_ARCH_OPTIONS.map((o) => o.value)).toEqual([
      'x86_64', 'arm64',
    ]);

    // 11. Instance Families
    expect(INSTANCE_FAMILY_OPTIONS.map((o) => o.value)).toEqual([
      'general_purpose', 'compute_optimized', 'memory_optimized', 'storage_optimized', 'gpu',
    ]);

    // 12. Storage Families (relational database)
    expect(STORAGE_FAMILY_OPTIONS.map((o) => o.value)).toEqual([
      '', 'gp3', 'gp2', 'io1', 'ssd', 'hdd',
    ]);

    // 13. Serverless Tiers
    expect(SERVERLESS_TIER_OPTIONS.map((o) => o.value)).toEqual([
      'consumption', 'flex_consumption', '1st_gen', '2nd_gen',
    ]);
  });

  it('renders all concrete select components with their respective labels', () => {
    const { unmount } = render(
      <div>
        <RegionSelect value="us-east" onChange={() => {}} />
        <CurrencySelect value="USD" onChange={() => {}} />
        <StorageClassSelect value="standard" onChange={() => {}} />
        <TransferTypeSelect value="internet_egress" onChange={() => {}} />
        <DatabaseEngineSelect value="postgresql" onChange={() => {}} />
        <NoSQLDataModelSelect value="document" onChange={() => {}} />
        <NoSQLPricingModeSelect value="provisioned" onChange={() => {}} />
        <KubernetesTierSelect value="standard" onChange={() => {}} />
        <ClusterTopologySelect value="regional" onChange={() => {}} />
        <ServerlessArchSelect value="x86_64" onChange={() => {}} />
        <ServerlessTierSelect value="consumption" onChange={() => {}} />
        <InstanceFamilySelect value="general_purpose" onChange={() => {}} />
        <StorageFamilySelect value="" onChange={() => {}} />
      </div>
    );

    expect(screen.getByLabelText('Region Group')).toBeInTheDocument();
    expect(screen.getByLabelText('Currency')).toBeInTheDocument();
    expect(screen.getByLabelText('Storage Tier')).toBeInTheDocument();
    expect(screen.getByLabelText('Transfer Type')).toBeInTheDocument();
    expect(screen.getByLabelText('Database Engine')).toBeInTheDocument();
    expect(screen.getByLabelText('Data Model')).toBeInTheDocument();
    expect(screen.getByLabelText('Pricing Mode')).toBeInTheDocument();
    expect(screen.getByLabelText('Management Tier')).toBeInTheDocument();
    expect(screen.getByLabelText('Cluster Topology')).toBeInTheDocument();
    expect(screen.getByLabelText('CPU Architecture')).toBeInTheDocument();
    expect(screen.getByLabelText('Serverless Tier')).toBeInTheDocument();
    expect(screen.getByLabelText('Instance Family')).toBeInTheDocument();
    expect(screen.getByLabelText('Storage Family')).toBeInTheDocument();

    unmount();
  });
});

