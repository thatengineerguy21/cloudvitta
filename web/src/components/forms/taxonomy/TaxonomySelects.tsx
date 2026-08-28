import React from 'react';
import { SelectInput, SelectInputProps } from '../SelectInput';
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
} from './options';

export const RegionSelect: React.FC<Omit<SelectInputProps, 'options'>> = (props) => (
  <SelectInput label="Region Group" options={REGION_OPTIONS} {...props} />
);

export const CurrencySelect: React.FC<Omit<SelectInputProps, 'options'>> = (props) => (
  <SelectInput label="Currency" options={CURRENCY_OPTIONS} {...props} />
);

export const StorageClassSelect: React.FC<Omit<SelectInputProps, 'options'>> = (props) => (
  <SelectInput label="Storage Tier" options={STORAGE_CLASS_OPTIONS} {...props} />
);

export const TransferTypeSelect: React.FC<Omit<SelectInputProps, 'options'>> = (props) => (
  <SelectInput label="Transfer Type" options={TRANSFER_TYPE_OPTIONS} {...props} />
);

export const DatabaseEngineSelect: React.FC<Omit<SelectInputProps, 'options'>> = (props) => (
  <SelectInput label="Database Engine" options={DATABASE_ENGINE_OPTIONS} {...props} />
);

export const NoSQLDataModelSelect: React.FC<Omit<SelectInputProps, 'options'>> = (props) => (
  <SelectInput label="Data Model" options={NOSQL_DATA_MODEL_OPTIONS} {...props} />
);

export const NoSQLPricingModeSelect: React.FC<Omit<SelectInputProps, 'options'>> = (props) => (
  <SelectInput label="Pricing Mode" options={NOSQL_PRICING_MODE_OPTIONS} {...props} />
);

export const KubernetesTierSelect: React.FC<Omit<SelectInputProps, 'options'>> = (props) => (
  <SelectInput label="Management Tier" options={KUBERNETES_TIER_OPTIONS} {...props} />
);

export const ClusterTopologySelect: React.FC<Omit<SelectInputProps, 'options'>> = (props) => (
  <SelectInput label="Cluster Topology" options={CLUSTER_TOPOLOGY_OPTIONS} {...props} />
);

export const ServerlessArchSelect: React.FC<Omit<SelectInputProps, 'options'>> = (props) => (
  <SelectInput label="CPU Architecture" options={SERVERLESS_ARCH_OPTIONS} {...props} />
);

export const ServerlessTierSelect: React.FC<Omit<SelectInputProps, 'options'>> = (props) => (
  <SelectInput label="Serverless Tier" options={SERVERLESS_TIER_OPTIONS} {...props} />
);

export const InstanceFamilySelect: React.FC<Omit<SelectInputProps, 'options'>> = (props) => (
  <SelectInput label="Instance Family" options={INSTANCE_FAMILY_OPTIONS} {...props} />
);

export const StorageFamilySelect: React.FC<Omit<SelectInputProps, 'options'>> = (props) => (
  <SelectInput label="Storage Family" options={STORAGE_FAMILY_OPTIONS} {...props} />
);

