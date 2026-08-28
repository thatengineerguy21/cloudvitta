import { SelectOption } from '../SelectInput';

// 1. Region Group Options (18 Canonical Groups from internal/matching/regionmap)
export const REGION_OPTIONS: SelectOption[] = [
  { value: 'us-east', label: 'US East (N. Virginia, Ohio)' },
  { value: 'us-central', label: 'US Central (Iowa, Dallas)' },
  { value: 'us-west', label: 'US West (Oregon, California)' },
  { value: 'ca-central', label: 'Canada (Montreal, Toronto)' },
  { value: 'eu-west', label: 'Europe West (Ireland, London, Paris)' },
  { value: 'eu-central', label: 'Europe Central (Frankfurt, Zurich)' },
  { value: 'eu-north', label: 'Europe North (Stockholm, Finland)' },
  { value: 'eu-south', label: 'Europe South (Milan, Spain)' },
  { value: 'ap-east', label: 'Asia Pacific East (Hong Kong, Taipei)' },
  { value: 'ap-south', label: 'Asia Pacific South (Mumbai, Hyderabad)' },
  { value: 'ap-southeast', label: 'Asia Pacific SE (Singapore, Sydney, Jakarta)' },
  { value: 'ap-northeast', label: 'Asia Pacific NE (Tokyo, Seoul, Osaka)' },
  { value: 'sa-east', label: 'South America East (Sao Paulo)' },
  { value: 'me-central', label: 'Middle East Central (UAE, Bahrain)' },
  { value: 'me-south', label: 'Middle East South (Saudi Arabia)' },
  { value: 'il-central', label: 'Israel Central (Tel Aviv)' },
  { value: 'af-south', label: 'Africa South (Cape Town, Johannesburg)' },
  { value: 'global', label: 'Global (Multi-Region / Any)' },
];

// 2. Currency Options (ECB / Frankfurter Reference Rates from internal/fx)
export const CURRENCY_OPTIONS: SelectOption[] = [
  { value: 'USD', label: 'USD ($) - US Dollar' },
  { value: 'EUR', label: 'EUR (€) - Euro' },
  { value: 'GBP', label: 'GBP (£) - British Pound' },
  { value: 'JPY', label: 'JPY (¥) - Japanese Yen' },
  { value: 'CAD', label: 'CAD ($) - Canadian Dollar' },
  { value: 'AUD', label: 'AUD ($) - Australian Dollar' },
  { value: 'CHF', label: 'CHF (Fr) - Swiss Franc' },
  { value: 'CNY', label: 'CNY (¥) - Chinese Yuan' },
  { value: 'INR', label: 'INR (₹) - Indian Rupee' },
  { value: 'SGD', label: 'SGD ($) - Singapore Dollar' },
  { value: 'BRL', label: 'BRL (R$) - Brazilian Real' },
  { value: 'HKD', label: 'HKD ($) - Hong Kong Dollar' },
  { value: 'NZD', label: 'NZD ($) - New Zealand Dollar' },
  { value: 'SEK', label: 'SEK (kr) - Swedish Krona' },
  { value: 'KRW', label: 'KRW (₩) - South Korean Won' },
  { value: 'ZAR', label: 'ZAR (R) - South African Rand' },
  { value: 'MXN', label: 'MXN ($) - Mexican Peso' },
  { value: 'NOK', label: 'NOK (kr) - Norwegian Krone' },
  { value: 'PLN', label: 'PLN (zł) - Polish Zloty' },
  { value: 'TRY', label: 'TRY (₺) - Turkish Lira' },
];

// 3. Storage Class Options (internal/matching/storageclassmap)
export const STORAGE_CLASS_OPTIONS: SelectOption[] = [
  { value: 'standard', label: 'Standard Tier' },
  { value: 'infrequent_access', label: 'Infrequent Access Tier' },
  { value: 'archive', label: 'Archive / Cold Tier' },
];

// 4. Transfer Type Options (internal/matching/transfertypemap)
export const TRANSFER_TYPE_OPTIONS: SelectOption[] = [
  { value: 'internet_egress', label: 'Internet Outbound Egress' },
  { value: 'intra_region', label: 'Intra-Region Data Transfer' },
  { value: 'inter_region', label: 'Inter-Region / Inter-Zone Transfer' },
];

// 5. Database Engine Options (internal/matching/databaseenginemap)
export const DATABASE_ENGINE_OPTIONS: SelectOption[] = [
  { value: 'postgresql', label: 'PostgreSQL' },
  { value: 'mysql', label: 'MySQL' },
  { value: 'sqlserver', label: 'Microsoft SQL Server' },
];

// 6. NoSQL Data Model Options (internal/matching/nosqldatamodelmap: document, key_value, wide_column, graph, multi_model)
export const NOSQL_DATA_MODEL_OPTIONS: SelectOption[] = [
  { value: 'document', label: 'Document (e.g. Firestore, Cosmos Document, DynamoDB)' },
  { value: 'key_value', label: 'Key-Value (e.g. DynamoDB, Cosmos Table)' },
  { value: 'wide_column', label: 'Wide-Column (e.g. Cosmos Cassandra)' },
  { value: 'graph', label: 'Graph (e.g. Cosmos Gremlin)' },
  { value: 'multi_model', label: 'Multi-Model' },
];

// 7. NoSQL Pricing Mode Options (defined in internal/domain/database_nosql.go and REST/MCP validation)
export const NOSQL_PRICING_MODE_OPTIONS: SelectOption[] = [
  { value: 'provisioned', label: 'Provisioned Throughput (RU/s / Capacity Units; default)' },
  { value: 'on_demand', label: 'On-Demand (Pay-per-Request / RCU-WCU)' },
  { value: 'serverless', label: 'Serverless Consumption Mode' },
];

// 8. Kubernetes Tier Options (internal/matching/kubernetestieremap & internal/domain/kubernetes.go)
export const KUBERNETES_TIER_OPTIONS: SelectOption[] = [
  { value: 'free', label: 'Free / Basic Tier' },
  { value: 'standard', label: 'Standard Cluster Tier' },
  { value: 'extended_support', label: 'Extended Support / Enterprise Tier' },
];

// 9. Kubernetes Topology Options (internal/domain/kubernetes.go)
export const CLUSTER_TOPOLOGY_OPTIONS: SelectOption[] = [
  { value: 'regional', label: 'Regional (Multi-Master High Availability)' },
  { value: 'zonal', label: 'Zonal (Single Master Zone)' },
  { value: 'autopilot', label: 'Autopilot / Fully Managed Control Plane' },
];

// 10. Serverless Architecture Options (internal/matching/serverlessarchmap)
export const SERVERLESS_ARCH_OPTIONS: SelectOption[] = [
  { value: 'x86_64', label: 'x86_64 (Intel / AMD)' },
  { value: 'arm64', label: 'arm64 (AWS Graviton / Ampere Altra)' },
];

// 11. Compute Instance Family Options (canonical families normalized across provider adapters in internal/adapter/provider/*/compute.go and internal/service/match.go)
export const INSTANCE_FAMILY_OPTIONS: SelectOption[] = [
  { value: 'general_purpose', label: 'General Purpose (e.g. t3, m5, Standard_D, e2)' },
  { value: 'compute_optimized', label: 'Compute Optimized (e.g. c5, Standard_F, c2)' },
  { value: 'memory_optimized', label: 'Memory Optimized (e.g. r5, Standard_E, m2)' },
  { value: 'storage_optimized', label: 'Storage Optimized (e.g. i3, Standard_L, z3)' },
  { value: 'gpu', label: 'Accelerated / GPU (e.g. g4dn, Standard_NC, a2)' },
];
