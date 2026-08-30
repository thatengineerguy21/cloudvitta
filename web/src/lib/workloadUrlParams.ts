// web/src/lib/workloadUrlParams.ts
import type {
  CalculateRequestBody,
  ComputeAttributes,
  StorageAttributes,
  NetworkAttributes,
  DatabaseRDBMSAttributes,
  DatabaseNoSQLAttributes,
  KubernetesAttributes,
  ServerlessWorkloadPayload,
} from '../types/api';

export interface ComputeFormState extends ComputeAttributes {
  enabled: boolean;
}

export interface StorageFormState extends StorageAttributes {
  enabled: boolean;
}

export interface NetworkFormState extends NetworkAttributes {
  enabled: boolean;
}

export interface DatabaseRDBMSFormState extends DatabaseRDBMSAttributes {
  enabled: boolean;
}

export interface DatabaseNoSQLFormState extends DatabaseNoSQLAttributes {
  enabled: boolean;
}

export interface KubernetesFormState extends KubernetesAttributes {
  enabled: boolean;
}

export interface ServerlessFormState extends ServerlessWorkloadPayload {
  enabled: boolean;
}

export interface WorkloadFormState {
  region: string;
  currency: string;
  strict_family: boolean;
  compute: ComputeFormState;
  storage: StorageFormState;
  network: NetworkFormState;
  database_rdbms: DatabaseRDBMSFormState;
  database_nosql: DatabaseNoSQLFormState;
  kubernetes: KubernetesFormState;
  serverless: ServerlessFormState;
}

export const DEFAULT_WORKLOAD_STATE: WorkloadFormState = {
  region: 'us-east',
  currency: 'USD',
  strict_family: true,
  compute: {
    enabled: true,
    vcpu: 2,
    ram_gb: 4,
    family: '',
  },
  storage: {
    enabled: false,
    size_gb: 100,
    storage_class: 'standard',
  },
  network: {
    enabled: false,
    egress_gb: 100,
    transfer_type: 'internet_egress',
  },
  database_rdbms: {
    enabled: false,
    engine: 'postgresql',
    vcpu: 2,
    ram_gb: 8,
    storage_gb: 100,
    iops: 3000,
    multi_az: false,
    storage_family: 'gp3',
  },
  database_nosql: {
    enabled: false,
    data_model: 'document',
    pricing_mode: 'provisioned',
    read_units: 100,
    write_units: 50,
    storage_gb: 100,
    storage_class: 'standard',
    multi_region: false,
  },
  kubernetes: {
    enabled: false,
    tier: 'standard',
    cluster_topology: 'regional',
  },
  serverless: {
    enabled: false,
    architecture: 'x86_64',
    tier: 'consumption',
    requests_per_month: 1000000,
    memory_mb: 512,
    execution_duration_ms: 200,
  },
};

/**
 * Parses URL search string into a structured WorkloadFormState.
 */
export function parseWorkloadFromUrl(search: string): WorkloadFormState {
  const params = new URLSearchParams(search.startsWith('?') ? search.slice(1) : search);
  const state: WorkloadFormState = JSON.parse(JSON.stringify(DEFAULT_WORKLOAD_STATE));

  // Global parameters
  if (params.has('region')) state.region = params.get('region') || DEFAULT_WORKLOAD_STATE.region;
  if (params.has('currency')) state.currency = params.get('currency') || DEFAULT_WORKLOAD_STATE.currency;
  if (params.has('strict_family')) state.strict_family = params.get('strict_family') !== 'false';

  // Compute (c_)
  if (params.has('c_on')) state.compute.enabled = params.get('c_on') === '1' || params.get('c_on') === 'true';
  if (params.has('c_vcpu')) {
    const v = Number(params.get('c_vcpu'));
    if (!isNaN(v) && v > 0) state.compute.vcpu = v;
  }
  if (params.has('c_ram')) {
    const v = Number(params.get('c_ram'));
    if (!isNaN(v) && v > 0) state.compute.ram_gb = v;
  }
  if (params.has('c_family')) state.compute.family = params.get('c_family') || '';

  // Storage (s_)
  if (params.has('s_on')) state.storage.enabled = params.get('s_on') === '1' || params.get('s_on') === 'true';
  if (params.has('s_size')) {
    const v = Number(params.get('s_size'));
    if (!isNaN(v) && v > 0) state.storage.size_gb = v;
  }
  if (params.has('s_tier')) state.storage.storage_class = params.get('s_tier') || 'standard';
  if (params.has('s_iops')) {
    const v = Number(params.get('s_iops'));
    if (!isNaN(v) && v > 0) state.storage.iops = v;
  }

  // Network (n_)
  if (params.has('n_on')) state.network.enabled = params.get('n_on') === '1' || params.get('n_on') === 'true';
  if (params.has('n_egress')) {
    const v = Number(params.get('n_egress'));
    if (!isNaN(v) && v >= 0) state.network.egress_gb = v;
  }
  if (params.has('n_type')) state.network.transfer_type = params.get('n_type') || 'internet_egress';

  // Database RDBMS (db_)
  if (params.has('db_on')) state.database_rdbms.enabled = params.get('db_on') === '1' || params.get('db_on') === 'true';
  if (params.has('db_engine')) state.database_rdbms.engine = params.get('db_engine') || 'postgresql';
  if (params.has('db_vcpu')) {
    const v = Number(params.get('db_vcpu'));
    if (!isNaN(v) && v > 0) state.database_rdbms.vcpu = v;
  }
  if (params.has('db_ram')) {
    const v = Number(params.get('db_ram'));
    if (!isNaN(v) && v > 0) state.database_rdbms.ram_gb = v;
  }
  if (params.has('db_storage')) {
    const v = Number(params.get('db_storage'));
    if (!isNaN(v) && v > 0) state.database_rdbms.storage_gb = v;
  }
  if (params.has('db_iops')) {
    const v = Number(params.get('db_iops'));
    if (!isNaN(v) && v > 0) state.database_rdbms.iops = v;
  }
  if (params.has('db_multi_az')) state.database_rdbms.multi_az = params.get('db_multi_az') === '1' || params.get('db_multi_az') === 'true';
  if (params.has('db_family')) state.database_rdbms.storage_family = params.get('db_family') || '';

  // Database NoSQL (nosql_)
  if (params.has('nosql_on')) state.database_nosql.enabled = params.get('nosql_on') === '1' || params.get('nosql_on') === 'true';
  if (params.has('nosql_model')) state.database_nosql.data_model = params.get('nosql_model') || 'document';
  if (params.has('nosql_mode')) state.database_nosql.pricing_mode = params.get('nosql_mode') || 'provisioned';
  if (params.has('nosql_ru')) {
    const v = Number(params.get('nosql_ru'));
    if (!isNaN(v) && v >= 0) state.database_nosql.read_units = v;
  }
  if (params.has('nosql_wu')) {
    const v = Number(params.get('nosql_wu'));
    if (!isNaN(v) && v >= 0) state.database_nosql.write_units = v;
  }
  if (params.has('nosql_storage')) {
    const v = Number(params.get('nosql_storage'));
    if (!isNaN(v) && v >= 0) state.database_nosql.storage_gb = v;
  }
  if (params.has('nosql_class')) state.database_nosql.storage_class = params.get('nosql_class') || 'standard';
  if (params.has('nosql_multi_region')) state.database_nosql.multi_region = params.get('nosql_multi_region') === '1' || params.get('nosql_multi_region') === 'true';

  // Kubernetes (k8s_)
  if (params.has('k8s_on')) state.kubernetes.enabled = params.get('k8s_on') === '1' || params.get('k8s_on') === 'true';
  if (params.has('k8s_tier')) state.kubernetes.tier = params.get('k8s_tier') || 'standard';
  if (params.has('k8s_topo')) state.kubernetes.cluster_topology = params.get('k8s_topo') || 'regional';

  // Serverless (fn_)
  if (params.has('fn_on')) state.serverless.enabled = params.get('fn_on') === '1' || params.get('fn_on') === 'true';
  if (params.has('fn_arch')) state.serverless.architecture = params.get('fn_arch') || 'x86_64';
  if (params.has('fn_tier')) state.serverless.tier = params.get('fn_tier') || 'consumption';
  if (params.has('fn_req')) {
    const v = Number(params.get('fn_req'));
    if (!isNaN(v) && v >= 0) state.serverless.requests_per_month = v;
  }
  if (params.has('fn_mem')) {
    const v = Number(params.get('fn_mem'));
    if (!isNaN(v) && v > 0) state.serverless.memory_mb = v;
  }
  if (params.has('fn_dur')) {
    const v = Number(params.get('fn_dur'));
    if (!isNaN(v) && v > 0) state.serverless.execution_duration_ms = v;
  }

  return state;
}

/**
 * Serializes WorkloadFormState to URL query string format.
 */
export function serializeWorkloadToUrl(state: WorkloadFormState): string {
  const params = new URLSearchParams();

  if (state.region !== DEFAULT_WORKLOAD_STATE.region) params.set('region', state.region);
  if (state.currency !== DEFAULT_WORKLOAD_STATE.currency) params.set('currency', state.currency);
  if (state.strict_family !== DEFAULT_WORKLOAD_STATE.strict_family) params.set('strict_family', String(state.strict_family));

  // Compute
  if (state.compute.enabled) {
    params.set('c_on', '1');
    if (state.compute.vcpu !== undefined) params.set('c_vcpu', String(state.compute.vcpu));
    if (state.compute.ram_gb !== undefined) params.set('c_ram', String(state.compute.ram_gb));
    if (state.compute.family) params.set('c_family', state.compute.family);
  }

  // Storage
  if (state.storage.enabled) {
    params.set('s_on', '1');
    if (state.storage.size_gb !== undefined) params.set('s_size', String(state.storage.size_gb));
    if (state.storage.storage_class) params.set('s_tier', state.storage.storage_class);
    if (state.storage.iops !== undefined) params.set('s_iops', String(state.storage.iops));
  }

  // Network
  if (state.network.enabled) {
    params.set('n_on', '1');
    if (state.network.egress_gb !== undefined) params.set('n_egress', String(state.network.egress_gb));
    if (state.network.transfer_type) params.set('n_type', state.network.transfer_type);
  }

  // Database RDBMS
  if (state.database_rdbms.enabled) {
    params.set('db_on', '1');
    if (state.database_rdbms.engine) params.set('db_engine', state.database_rdbms.engine);
    if (state.database_rdbms.vcpu !== undefined) params.set('db_vcpu', String(state.database_rdbms.vcpu));
    if (state.database_rdbms.ram_gb !== undefined) params.set('db_ram', String(state.database_rdbms.ram_gb));
    if (state.database_rdbms.storage_gb !== undefined) params.set('db_storage', String(state.database_rdbms.storage_gb));
    if (state.database_rdbms.iops !== undefined) params.set('db_iops', String(state.database_rdbms.iops));
    if (state.database_rdbms.multi_az) params.set('db_multi_az', '1');
    if (state.database_rdbms.storage_family) params.set('db_family', state.database_rdbms.storage_family);
  }

  // Database NoSQL
  if (state.database_nosql.enabled) {
    params.set('nosql_on', '1');
    if (state.database_nosql.data_model) params.set('nosql_model', state.database_nosql.data_model);
    if (state.database_nosql.pricing_mode) params.set('nosql_mode', state.database_nosql.pricing_mode);
    if (state.database_nosql.read_units !== undefined) params.set('nosql_ru', String(state.database_nosql.read_units));
    if (state.database_nosql.write_units !== undefined) params.set('nosql_wu', String(state.database_nosql.write_units));
    if (state.database_nosql.storage_gb !== undefined) params.set('nosql_storage', String(state.database_nosql.storage_gb));
    if (state.database_nosql.storage_class) params.set('nosql_class', state.database_nosql.storage_class);
    if (state.database_nosql.multi_region) params.set('nosql_multi_region', '1');
  }

  // Kubernetes
  if (state.kubernetes.enabled) {
    params.set('k8s_on', '1');
    if (state.kubernetes.tier) params.set('k8s_tier', state.kubernetes.tier);
    if (state.kubernetes.cluster_topology) params.set('k8s_topo', state.kubernetes.cluster_topology);
  }

  // Serverless
  if (state.serverless.enabled) {
    params.set('fn_on', '1');
    if (state.serverless.architecture) params.set('fn_arch', state.serverless.architecture);
    if (state.serverless.tier) params.set('fn_tier', state.serverless.tier);
    if (state.serverless.requests_per_month !== undefined) params.set('fn_req', String(state.serverless.requests_per_month));
    if (state.serverless.memory_mb !== undefined) params.set('fn_mem', String(state.serverless.memory_mb));
    if (state.serverless.execution_duration_ms !== undefined) params.set('fn_dur', String(state.serverless.execution_duration_ms));
  }

  const str = params.toString();
  return str ? `?${str}` : '';
}

/**
 * Builds backend CalculateRequestBody from WorkloadFormState.
 * Enforces ADR 0032 by using single canonical database_rdbms key.
 */
export function buildCalculateRequestBody(state: WorkloadFormState): CalculateRequestBody {
  const body: CalculateRequestBody = {
    region: state.region || 'us-east',
    currency: state.currency || 'USD',
    strict_family: state.strict_family,
  };

  if (state.compute.enabled) {
    body.compute = {
      vcpu: Number(state.compute.vcpu) || 1,
      ram_gb: Number(state.compute.ram_gb) || 1,
      ...(state.compute.family ? { family: state.compute.family } : {}),
    };
  }

  if (state.storage.enabled) {
    body.storage = {
      size_gb: Number(state.storage.size_gb) || 1,
      storage_class: state.storage.storage_class || 'standard',
      ...(state.storage.iops ? { iops: Number(state.storage.iops) } : {}),
    };
  }

  if (state.network.enabled) {
    body.network = {
      egress_gb: Number(state.network.egress_gb) >= 0 ? Number(state.network.egress_gb) : 0,
      transfer_type: state.network.transfer_type || 'internet_egress',
    };
  }

  if (state.database_rdbms.enabled) {
    body.database_rdbms = {
      engine: state.database_rdbms.engine || 'postgresql',
      vcpu: Number(state.database_rdbms.vcpu) || 1,
      ram_gb: Number(state.database_rdbms.ram_gb) || 1,
      storage_gb: Number(state.database_rdbms.storage_gb) || 1,
      ...(state.database_rdbms.iops ? { iops: Number(state.database_rdbms.iops) } : {}),
      ...(state.database_rdbms.multi_az !== undefined ? { multi_az: state.database_rdbms.multi_az } : {}),
      ...(state.database_rdbms.storage_family ? { storage_family: state.database_rdbms.storage_family } : {}),
    };
  }

  if (state.database_nosql.enabled) {
    body.database_nosql = {
      data_model: state.database_nosql.data_model || 'document',
      pricing_mode: state.database_nosql.pricing_mode || 'provisioned',
      read_units: Number(state.database_nosql.read_units) >= 0 ? Number(state.database_nosql.read_units) : 0,
      write_units: Number(state.database_nosql.write_units) >= 0 ? Number(state.database_nosql.write_units) : 0,
      storage_gb: Number(state.database_nosql.storage_gb) >= 0 ? Number(state.database_nosql.storage_gb) : 0,
      ...(state.database_nosql.storage_class ? { storage_class: state.database_nosql.storage_class } : {}),
      ...(state.database_nosql.multi_region !== undefined ? { multi_region: state.database_nosql.multi_region } : {}),
    };
  }

  if (state.kubernetes.enabled) {
    body.kubernetes = {
      tier: state.kubernetes.tier || 'standard',
      ...(state.kubernetes.cluster_topology ? { cluster_topology: state.kubernetes.cluster_topology } : {}),
    };
  }

  if (state.serverless.enabled) {
    body.serverless = {
      architecture: state.serverless.architecture || 'x86_64',
      tier: state.serverless.tier || 'consumption',
      requests_per_month: Number(state.serverless.requests_per_month) >= 0 ? Number(state.serverless.requests_per_month) : 0,
      memory_mb: Number(state.serverless.memory_mb) || 128,
      execution_duration_ms: Number(state.serverless.execution_duration_ms) || 100,
    };
  }

  return body;
}

/**
 * Returns an array of canonical keys for all enabled categories in the workload state.
 */
export function getEnabledCategoryKeys(state: WorkloadFormState): string[] {
  const keys: string[] = [];
  if (state.compute.enabled) keys.push('compute');
  if (state.storage.enabled) keys.push('storage');
  if (state.network.enabled) keys.push('network');
  if (state.database_rdbms.enabled) keys.push('database_rdbms');
  if (state.database_nosql.enabled) keys.push('database_nosql');
  if (state.kubernetes.enabled) keys.push('kubernetes');
  if (state.serverless.enabled) keys.push('serverless');
  return keys;
}

/**
 * Returns the count of active categories in the workload state.
 */
export function getActiveCategoryCount(state: WorkloadFormState): number {
  return getEnabledCategoryKeys(state).length;
}

