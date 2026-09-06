// web/src/components/calculate/WorkloadBuilderForm.tsx
import React, { useState } from 'react';
import { WorkloadCategoryCard } from './WorkloadCategoryCard';
import {
  RegionSelect,
  CurrencySelect,
  InstanceFamilySelect,
  StorageClassSelect,
  TransferTypeSelect,
  DatabaseEngineSelect,
  StorageFamilySelect,
  NoSQLDataModelSelect,
  NoSQLPricingModeSelect,
  KubernetesTierSelect,
  ClusterTopologySelect,
  ServerlessArchSelect,
  ServerlessTierSelect,
} from '../forms/taxonomy/TaxonomySelects';
import { SelectInput } from '../forms/SelectInput';
import { DebouncedInput } from '../forms/DebouncedInput';
import { WorkloadFormState, getActiveCategoryCount } from '../../lib/workloadUrlParams';
import {
  Cpu,
  HardDrive,
  Network,
  Database,
  Server,
  Box,
  Zap,
  RotateCcw,
  Layers,
  Sliders,
} from 'lucide-react';

export interface WorkloadBuilderFormProps {
  state: WorkloadFormState;
  onChange: (newState: WorkloadFormState) => void;
  onReset: () => void;
}

type WorkloadCategoryKey =
  | 'compute'
  | 'storage'
  | 'network'
  | 'database_rdbms'
  | 'database_nosql'
  | 'kubernetes'
  | 'serverless';

export const WorkloadBuilderForm: React.FC<WorkloadBuilderFormProps> = ({
  state,
  onChange,
  onReset,
}) => {
  const [openSections, setOpenSections] = useState<Record<string, boolean>>({
    compute: true,
    storage: false,
    network: false,
    database_rdbms: false,
    database_nosql: false,
    kubernetes: false,
    serverless: false,
  });

  const toggleSection = (key: string) => {
    setOpenSections((prev) => ({ ...prev, [key]: !prev[key] }));
  };

  const activeCount = getActiveCategoryCount(state);

  const updateField = <K extends WorkloadCategoryKey>(
    category: K,
    subField: keyof WorkloadFormState[K],
    value: unknown
  ) => {
    const current = state[category] as unknown as Record<string, unknown>;
    const updated = {
      ...state,
      [category]: {
        ...current,
        [subField]: value,
      },
    };
    onChange(updated);
  };

  const toggleCategoryEnabled = (category: WorkloadCategoryKey, enabled: boolean) => {
    const current = state[category] as unknown as Record<string, unknown>;
    const updated = {
      ...state,
      [category]: {
        ...current,
        enabled,
      },
    };
    if (enabled) {
      setOpenSections((prev) => ({ ...prev, [category]: true }));
    }
    onChange(updated);
  };

  const setAllCategories = (enabled: boolean) => {
    const updated = {
      ...state,
      compute: { ...state.compute, enabled },
      storage: { ...state.storage, enabled },
      network: { ...state.network, enabled },
      database_rdbms: { ...state.database_rdbms, enabled },
      database_nosql: { ...state.database_nosql, enabled },
      kubernetes: { ...state.kubernetes, enabled },
      serverless: { ...state.serverless, enabled },
    };
    onChange(updated);
  };

  return (
    <div className="space-y-4">
      {/* Global Parameters Card */}
      <div className="border border-border-default/80 bg-surface-card rounded-2xl shadow-sm p-4 space-y-4">
        <div className="flex items-center justify-between pb-3 border-b border-border-default/50">
          <div className="flex items-center space-x-2">
            <Sliders className="w-4 h-4 text-border-accent" />
            <h3 className="text-xs uppercase tracking-widest font-bold text-text-primary">
              Global Architecture Parameters
            </h3>
          </div>
          <button
            type="button"
            onClick={onReset}
            className="flex items-center space-x-1 text-xs text-text-secondary hover:text-text-primary uppercase tracking-wider font-semibold transition-colors rounded-lg px-2 py-1 hover:bg-surface-raised"
            title="Reset form to defaults"
          >
            <RotateCcw className="w-3.5 h-3.5" />
            <span>Reset</span>
          </button>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <RegionSelect
            value={state.region}
            onChange={(e) => onChange({ ...state, region: e.target.value })}
          />
          <CurrencySelect
            value={state.currency}
            onChange={(e) => onChange({ ...state, currency: e.target.value })}
          />
        </div>

        <div className="pt-1 flex items-center justify-between">
          <label className="flex items-center space-x-2 cursor-pointer text-xs text-text-secondary">
            <input
              type="checkbox"
              checked={state.strict_family}
              onChange={(e) => onChange({ ...state, strict_family: e.target.checked })}
              className="rounded accent-brand-500"
            />
            <span>Strict instance family matching</span>
          </label>

          <div className="flex items-center space-x-2 text-xs">
            <button
              type="button"
              onClick={() => setAllCategories(true)}
              className="text-border-accent hover:underline font-medium"
            >
              Enable All
            </button>
            <span className="text-border-default">•</span>
            <button
              type="button"
              onClick={() => setAllCategories(false)}
              className="text-text-secondary hover:text-text-primary font-medium"
            >
              Disable All
            </button>
          </div>
        </div>
      </div>

      {/* Category Accordion Header */}
      <div className="flex items-center justify-between px-1">
        <div className="flex items-center space-x-2">
          <Layers className="w-4 h-4 text-text-secondary" />
          <span className="text-xs uppercase tracking-widest font-bold text-text-secondary">
            Workload Components
          </span>
        </div>
        <span className="text-xs font-mono px-2.5 py-0.5 border border-border-default bg-surface-raised rounded-full text-text-primary">
          {activeCount} of 7 Active
        </span>
      </div>

      {/* 1. Compute */}
      <WorkloadCategoryCard
        title="Compute Instances"
        icon={Cpu}
        enabled={state.compute.enabled}
        onToggleEnabled={(en) => toggleCategoryEnabled('compute', en)}
        isOpen={openSections.compute}
        onToggleOpen={() => toggleSection('compute')}
        summary={`${state.compute.vcpu || 1} vCPU • ${state.compute.ram_gb || 1} GB`}
      >
        <DebouncedInput
          label="vCPU Cores"
          type="number"
          min="1"
          max="512"
          value={state.compute.vcpu}
          onChange={(val) => updateField('compute', 'vcpu', Math.max(1, Number(val) || 1))}
        />
        <DebouncedInput
          label="RAM (GiB)"
          type="number"
          min="1"
          max="4096"
          value={state.compute.ram_gb}
          onChange={(val) => updateField('compute', 'ram_gb', Math.max(1, Number(val) || 1))}
        />
        <div className="sm:col-span-2">
          <InstanceFamilySelect
            value={state.compute.family || ''}
            onChange={(e) => updateField('compute', 'family', e.target.value)}
          />
        </div>
      </WorkloadCategoryCard>

      {/* 2. Storage */}
      <WorkloadCategoryCard
        title="Storage Classes"
        icon={HardDrive}
        enabled={state.storage.enabled}
        onToggleEnabled={(en) => toggleCategoryEnabled('storage', en)}
        isOpen={openSections.storage}
        onToggleOpen={() => toggleSection('storage')}
        summary={`${state.storage.size_gb || 100} GB • ${state.storage.storage_class || 'standard'}`}
      >
        <DebouncedInput
          label="Capacity (GB)"
          type="number"
          min="1"
          max="1000000"
          value={state.storage.size_gb}
          onChange={(val) => updateField('storage', 'size_gb', Math.max(1, Number(val) || 1))}
        />
        <StorageClassSelect
          value={state.storage.storage_class || 'standard'}
          onChange={(e) => updateField('storage', 'storage_class', e.target.value)}
        />
        <div className="sm:col-span-2">
          <DebouncedInput
            label="Provisioned IOPS (Optional)"
            type="number"
            min="100"
            max="256000"
            value={state.storage.iops || ''}
            onChange={(val) => updateField('storage', 'iops', val ? Number(val) : undefined)}
          />
        </div>
      </WorkloadCategoryCard>

      {/* 3. Network */}
      <WorkloadCategoryCard
        title="Network Egress"
        icon={Network}
        enabled={state.network.enabled}
        onToggleEnabled={(en) => toggleCategoryEnabled('network', en)}
        isOpen={openSections.network}
        onToggleOpen={() => toggleSection('network')}
        summary={`${state.network.egress_gb || 0} GB • ${state.network.transfer_type || 'egress'}`}
      >
        <DebouncedInput
          label="Outbound Data (GB/mo)"
          type="number"
          min="0"
          max="10000000"
          value={state.network.egress_gb}
          onChange={(val) => updateField('network', 'egress_gb', Math.max(0, Number(val) || 0))}
        />
        <TransferTypeSelect
          value={state.network.transfer_type || 'internet_egress'}
          onChange={(e) => updateField('network', 'transfer_type', e.target.value)}
        />
      </WorkloadCategoryCard>

      {/* 4. Relational Databases */}
      <WorkloadCategoryCard
        title="Relational Databases (RDBMS)"
        icon={Database}
        enabled={state.database_rdbms.enabled}
        onToggleEnabled={(en) => toggleCategoryEnabled('database_rdbms', en)}
        isOpen={openSections.database_rdbms}
        onToggleOpen={() => toggleSection('database_rdbms')}
        summary={`${state.database_rdbms.engine || 'postgresql'} • ${state.database_rdbms.vcpu || 2} vCPU`}
      >
        <DatabaseEngineSelect
          value={state.database_rdbms.engine || 'postgresql'}
          onChange={(e) => updateField('database_rdbms', 'engine', e.target.value)}
        />
        <StorageFamilySelect
          value={state.database_rdbms.storage_family || ''}
          onChange={(e) => updateField('database_rdbms', 'storage_family', e.target.value)}
        />
        <DebouncedInput
          label="vCPU"
          type="number"
          min="1"
          max="256"
          value={state.database_rdbms.vcpu}
          onChange={(val) => updateField('database_rdbms', 'vcpu', Math.max(1, Number(val) || 1))}
        />
        <DebouncedInput
          label="RAM (GiB)"
          type="number"
          min="1"
          max="2048"
          value={state.database_rdbms.ram_gb}
          onChange={(val) => updateField('database_rdbms', 'ram_gb', Math.max(1, Number(val) || 1))}
        />
        <DebouncedInput
          label="Storage (GB)"
          type="number"
          min="1"
          max="1000000"
          value={state.database_rdbms.storage_gb}
          onChange={(val) => updateField('database_rdbms', 'storage_gb', Math.max(1, Number(val) || 1))}
        />
        <DebouncedInput
          label="IOPS (Optional)"
          type="number"
          min="100"
          max="256000"
          value={state.database_rdbms.iops || ''}
          onChange={(val) => updateField('database_rdbms', 'iops', val ? Number(val) : undefined)}
        />
        <div className="sm:col-span-2">
          <SelectInput
            label="High Availability"
            options={[
              { value: 'false', label: 'Single-AZ Instance' },
              { value: 'true', label: 'Multi-AZ High Availability' },
            ]}
            value={state.database_rdbms.multi_az ? 'true' : 'false'}
            onChange={(e) => updateField('database_rdbms', 'multi_az', e.target.value === 'true')}
          />
        </div>
      </WorkloadCategoryCard>

      {/* 5. NoSQL Databases */}
      <WorkloadCategoryCard
        title="NoSQL Databases"
        icon={Server}
        enabled={state.database_nosql.enabled}
        onToggleEnabled={(en) => toggleCategoryEnabled('database_nosql', en)}
        isOpen={openSections.database_nosql}
        onToggleOpen={() => toggleSection('database_nosql')}
        summary={`${state.database_nosql.data_model || 'document'} • ${state.database_nosql.pricing_mode || 'provisioned'}`}
      >
        <NoSQLDataModelSelect
          value={state.database_nosql.data_model || 'document'}
          onChange={(e) => updateField('database_nosql', 'data_model', e.target.value)}
        />
        <NoSQLPricingModeSelect
          value={state.database_nosql.pricing_mode || 'provisioned'}
          onChange={(e) => updateField('database_nosql', 'pricing_mode', e.target.value)}
        />
        <DebouncedInput
          label="Read Units (RCU / ops/sec)"
          type="number"
          min="0"
          max="10000000"
          value={state.database_nosql.read_units}
          onChange={(val) => updateField('database_nosql', 'read_units', Math.max(0, Number(val) || 0))}
        />
        <DebouncedInput
          label="Write Units (WCU / ops/sec)"
          type="number"
          min="0"
          max="10000000"
          value={state.database_nosql.write_units}
          onChange={(val) => updateField('database_nosql', 'write_units', Math.max(0, Number(val) || 0))}
        />
        <DebouncedInput
          label="Storage (GB)"
          type="number"
          min="0"
          max="1000000"
          value={state.database_nosql.storage_gb}
          onChange={(val) => updateField('database_nosql', 'storage_gb', Math.max(0, Number(val) || 0))}
        />
        <SelectInput
          label="Replication"
          options={[
            { value: 'false', label: 'Single Region' },
            { value: 'true', label: 'Multi-Region Global' },
          ]}
          value={state.database_nosql.multi_region ? 'true' : 'false'}
          onChange={(e) => updateField('database_nosql', 'multi_region', e.target.value === 'true')}
        />
      </WorkloadCategoryCard>

      {/* 6. Kubernetes Control-Plane */}
      <WorkloadCategoryCard
        title="Kubernetes Control-Plane"
        icon={Box}
        enabled={state.kubernetes.enabled}
        onToggleEnabled={(en) => toggleCategoryEnabled('kubernetes', en)}
        isOpen={openSections.kubernetes}
        onToggleOpen={() => toggleSection('kubernetes')}
        summary={`${state.kubernetes.tier || 'standard'} • ${state.kubernetes.cluster_topology || 'regional'}`}
      >
        <KubernetesTierSelect
          value={state.kubernetes.tier || 'standard'}
          onChange={(e) => updateField('kubernetes', 'tier', e.target.value)}
        />
        <ClusterTopologySelect
          value={state.kubernetes.cluster_topology || 'regional'}
          onChange={(e) => updateField('kubernetes', 'cluster_topology', e.target.value)}
        />
      </WorkloadCategoryCard>

      {/* 7. Serverless Compute */}
      <WorkloadCategoryCard
        title="Serverless Compute (FaaS)"
        icon={Zap}
        enabled={state.serverless.enabled}
        onToggleEnabled={(en) => toggleCategoryEnabled('serverless', en)}
        isOpen={openSections.serverless}
        onToggleOpen={() => toggleSection('serverless')}
        summary={`${state.serverless.architecture || 'x86_64'} • ${state.serverless.memory_mb || 512} MB`}
      >
        <ServerlessArchSelect
          value={state.serverless.architecture || 'x86_64'}
          onChange={(e) => updateField('serverless', 'architecture', e.target.value)}
        />
        <ServerlessTierSelect
          value={state.serverless.tier || 'consumption'}
          onChange={(e) => updateField('serverless', 'tier', e.target.value)}
        />
        <DebouncedInput
          label="Requests / Month"
          type="number"
          min="0"
          max="10000000000"
          value={state.serverless.requests_per_month}
          onChange={(val) => updateField('serverless', 'requests_per_month', Math.max(0, Number(val) || 0))}
        />
        <DebouncedInput
          label="Memory (MB)"
          type="number"
          min="128"
          max="10240"
          step={64}
          value={state.serverless.memory_mb}
          onChange={(val) => updateField('serverless', 'memory_mb', Math.max(128, Number(val) || 128))}
        />
        <div className="sm:col-span-2">
          <DebouncedInput
            label="Avg Duration (ms)"
            type="number"
            min="1"
            max="900000"
            value={state.serverless.execution_duration_ms}
            onChange={(val) => updateField('serverless', 'execution_duration_ms', Math.max(1, Number(val) || 1))}
          />
        </div>
      </WorkloadCategoryCard>
    </div>
  );
};
