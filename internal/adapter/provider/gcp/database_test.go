package gcp

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/quarantine"
)

func TestGCPNormalize_Database(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)

	f, err := os.Open("../../../../testdata/golden/gcp/database.json")
	if err != nil {
		t.Fatalf("failed to open database golden file: %v", err)
	}
	defer func() { _ = f.Close() }()

	res, _, err := Normalize(f, fixedTime)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}
	obs := res.Observations

	// 16 shapes * 2 (Zonal + Regional HA) + 2 storage (Zonal + Regional HA) + 1 AlloyDB = 35
	expectedTotal := len(knownGCPCloudSQLSpecs)*2 + 3
	if len(obs) != expectedTotal {
		t.Fatalf("expected %d observations, got %d", expectedTotal, len(obs))
	}

	for _, o := range obs {
		if o.ServiceCategory != "database_rdbms" {
			t.Errorf("expected ServiceCategory database_rdbms, got %s", o.ServiceCategory)
		}
		if o.Provider != "gcp" {
			t.Errorf("expected Provider gcp, got %s", o.Provider)
		}
	}

	// Check Cloud SQL PG synthesized 4 vCPU, 15 GB instance observation (Zonal)
	var cloudSQLInst15 *struct {
		engine      string
		vcpu        float64
		ram         float64
		multiAZ     bool
		priceAmount string
	}
	for _, o := range obs {
		if o.SkuID == "SKU-GCP-CLOUDSQL-POSTGRESQL-STANDARD-4VCPU-15GB" {
			cloudSQLInst15 = &struct {
				engine      string
				vcpu        float64
				ram         float64
				multiAZ     bool
				priceAmount string
			}{
				engine:      o.DatabaseRDBMSAttributes.Engine,
				vcpu:        o.DatabaseRDBMSAttributes.VCPU,
				ram:         o.DatabaseRDBMSAttributes.RAMGB,
				multiAZ:     o.DatabaseRDBMSAttributes.MultiAZ,
				priceAmount: o.PriceAmount.String(),
			}
			if o.DatabaseRDBMSAttributes.ComponentType != "instance" {
				t.Errorf("expected component_type instance, got %s", o.DatabaseRDBMSAttributes.ComponentType)
			}
		}
	}
	if cloudSQLInst15 == nil {
		t.Fatalf("missing SKU-GCP-CLOUDSQL-POSTGRESQL-STANDARD-4VCPU-15GB")
	}
	// 4 * 0.05 + 15 * 0.007 = 0.20 + 0.105 = 0.305
	if cloudSQLInst15.engine != "postgresql" || cloudSQLInst15.vcpu != 4 || cloudSQLInst15.ram != 15 || cloudSQLInst15.multiAZ != false || cloudSQLInst15.priceAmount != "0.305" {
		t.Errorf("unexpected instance attrs: %+v", cloudSQLInst15)
	}

	// Check Cloud SQL PG synthesized 4 vCPU, 16 GB instance observation (Regional HA)
	var cloudSQLInst16HA *struct {
		engine      string
		vcpu        float64
		ram         float64
		multiAZ     bool
		priceAmount string
	}
	for _, o := range obs {
		if o.SkuID == "SKU-GCP-CLOUDSQL-POSTGRESQL-STANDARD-4VCPU-16GB-HA" {
			cloudSQLInst16HA = &struct {
				engine      string
				vcpu        float64
				ram         float64
				multiAZ     bool
				priceAmount string
			}{
				engine:      o.DatabaseRDBMSAttributes.Engine,
				vcpu:        o.DatabaseRDBMSAttributes.VCPU,
				ram:         o.DatabaseRDBMSAttributes.RAMGB,
				multiAZ:     o.DatabaseRDBMSAttributes.MultiAZ,
				priceAmount: o.PriceAmount.String(),
			}
			if o.DatabaseRDBMSAttributes.ComponentType != "instance" {
				t.Errorf("expected component_type instance, got %s", o.DatabaseRDBMSAttributes.ComponentType)
			}
		}
	}
	if cloudSQLInst16HA == nil {
		t.Fatalf("missing SKU-GCP-CLOUDSQL-POSTGRESQL-STANDARD-4VCPU-16GB-HA")
	}
	// 4 * 0.10 + 16 * 0.014 = 0.40 + 0.224 = 0.624
	if cloudSQLInst16HA.engine != "postgresql" || cloudSQLInst16HA.vcpu != 4 || cloudSQLInst16HA.ram != 16 || cloudSQLInst16HA.multiAZ != true || cloudSQLInst16HA.priceAmount != "0.624" {
		t.Errorf("unexpected HA instance attrs: %+v", cloudSQLInst16HA)
	}

	// Check Cloud SQL storage observation (Regional HA)
	var cloudSQLStor *struct {
		family  string
		multiAZ bool
	}
	for _, o := range obs {
		if o.SkuID == "SKU-GCP-CLOUDSQL-STORAGE-SSD-HA" {
			cloudSQLStor = &struct {
				family  string
				multiAZ bool
			}{
				family:  o.DatabaseRDBMSAttributes.StorageFamily,
				multiAZ: o.DatabaseRDBMSAttributes.MultiAZ,
			}
			if o.DatabaseRDBMSAttributes.ComponentType != "storage" {
				t.Errorf("expected component_type storage, got %s", o.DatabaseRDBMSAttributes.ComponentType)
			}
		}
	}
	if cloudSQLStor == nil {
		t.Fatalf("missing SKU-GCP-CLOUDSQL-STORAGE-SSD-HA")
	}
	if cloudSQLStor.family != "ssd" || cloudSQLStor.multiAZ != true {
		t.Errorf("unexpected storage attrs: %+v", cloudSQLStor)
	}
}

func TestGCPNormalize_Database_GenericStorageAndNonInstanceIgnored(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)

	jsonBody := `{
		"skus": [
			{
				"skuId": "SKU-GCP-GENERIC-STORAGE",
				"description": "Storage PD SSD in Virginia",
				"category": {
					"serviceDisplayName": "Cloud SQL",
					"resourceFamily": "ApplicationServices",
					"resourceGroup": "PDSSD",
					"usageType": "OnDemand"
				},
				"serviceRegions": ["us-east4"],
				"pricingInfo": [
					{
						"pricingExpression": {
							"usageUnit": "GiBy.mo",
							"tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "0", "nanos": 170000000}}]
						}
					}
				]
			},
			{
				"skuId": "SKU-GCP-CLOUDSQL-EGRESS",
				"description": "Cloud SQL: Network Egress - Worldwide",
				"category": {
					"serviceDisplayName": "Cloud SQL",
					"resourceFamily": "ApplicationServices",
					"resourceGroup": "Network",
					"usageType": "OnDemand"
				},
				"serviceRegions": ["us-east4"],
				"pricingInfo": [
					{
						"pricingExpression": {
							"usageUnit": "GiBy",
							"tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "0", "nanos": 120000000}}]
						}
					}
				]
			}
		]
	}`

	memSink := quarantine.NewMemorySink()
	res, _, err := Normalize(strings.NewReader(jsonBody), fixedTime, memSink)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	if len(res.Observations) != 1 {
		t.Fatalf("expected 1 observation (generic storage), got %d", len(res.Observations))
	}
	obs := res.Observations[0]
	if obs.SkuID != "SKU-GCP-GENERIC-STORAGE" {
		t.Errorf("expected SKU-GCP-GENERIC-STORAGE, got %s", obs.SkuID)
	}
	if obs.DatabaseRDBMSAttributes.ComponentType != "storage" {
		t.Errorf("expected ComponentType storage, got %s", obs.DatabaseRDBMSAttributes.ComponentType)
	}
	if obs.DatabaseRDBMSAttributes.Engine != "" {
		t.Errorf("expected generic storage to have empty engine, got %s", obs.DatabaseRDBMSAttributes.Engine)
	}

	if res.IgnoredCount != 1 {
		t.Errorf("expected 1 ignored out-of-scope SKU (network egress), got %d", res.IgnoredCount)
	}

	if memSink.Count() != 0 {
		t.Errorf("expected 0 quarantine items, got %d", memSink.Count())
	}
}

func TestNormalize_GCPAlloyDB(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)

	f, err := os.Open("../../../../testdata/golden/gcp/alloydb.json")
	if err != nil {
		t.Fatalf("failed to open alloydb golden file: %v", err)
	}
	defer func() { _ = f.Close() }()

	res, _, err := Normalize(f, fixedTime)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	if len(res.Observations) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(res.Observations))
	}
	obs := res.Observations[0]
	if obs.Provider != "gcp" {
		t.Errorf("Provider = %q, want gcp", obs.Provider)
	}
	if obs.ServiceCategory != "database_rdbms" {
		t.Errorf("ServiceCategory = %q, want database_rdbms", obs.ServiceCategory)
	}
	if obs.DatabaseRDBMSAttributes.Engine != "postgresql" {
		t.Errorf("Engine = %q, want postgresql", obs.DatabaseRDBMSAttributes.Engine)
	}
	if obs.DatabaseRDBMSAttributes.VCPU != 8 {
		t.Errorf("VCPU = %v, want 8", obs.DatabaseRDBMSAttributes.VCPU)
	}
	if obs.DatabaseRDBMSAttributes.RAMGB != 64 {
		t.Errorf("RAMGB = %v, want 64", obs.DatabaseRDBMSAttributes.RAMGB)
	}
	if obs.DatabaseRDBMSAttributes.DeploymentTier != "alloydb" {
		t.Errorf("DeploymentTier = %q, want alloydb", obs.DatabaseRDBMSAttributes.DeploymentTier)
	}
	if obs.DatabaseRDBMSAttributes.ComponentType != "instance" {
		t.Errorf("ComponentType = %q, want instance", obs.DatabaseRDBMSAttributes.ComponentType)
	}
}

func TestGCPNormalize_Database_OutOfScopeIgnoredWithoutQuarantine(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)

	outOfScopeSamples := []struct {
		name        string
		description string
		group       string
	}{
		{
			name:        "extended support vCPU",
			description: "Cloud SQL for PostgreSQL: Zonal - Enterprise Plus Extended support vCPU v13 in Northern Virginia",
			group:       "CPU",
		},
		{
			name:        "extended support combined",
			description: "Cloud SQL for MySQL: Zonal - Extended support 16 vCPU + 104GB RAM v56 in Delhi",
			group:       "MySQL",
		},
		{
			name:        "storage hyperdisk balanced IOPS",
			description: "Cloud SQL for SQL Server: Zonal - Enterprise Storage Hyperdisk Balanced IOPS in Netherlands",
			group:       "PDSSD",
		},
		{
			name:        "backup storage",
			description: "Backup storage in asia-southeast4 region",
			group:       "PDStandard",
		},
		{
			name:        "point in time recovery",
			description: "Cloud SQL for PostgreSQL: Point-in-time recovery in us-east4",
			group:       "PITR",
		},
		{
			name:        "IP address reservation",
			description: "Cloud SQL for PostgreSQL: Regional - IP address reservation in Salt Lake City",
			group:       "Network",
		},
		{
			name:        "network data transfer egress",
			description: "Network Internet Data Transfer Out from EMEA to Seoul",
			group:       "PremiumInternetEgress",
		},
		{
			name:        "read replica promotional",
			description: "Cloud SQL for MySQL: Read Replica (free with promotional discount until September 2021) - Standard storage in Stockholm",
			group:       "PDSSD",
		},
		{
			name:        "FDC trial",
			description: "FDC Trial in Cloud SQL for PostgreSQL: Regional - vCPU in Phoenix",
			group:       "CPU",
		},
		{
			name:        "legacy generation g1-small",
			description: "Cloud SQL for PostgreSQL: Regional - Extended support g1-small v11 in Belgium",
			group:       "PostgreSQL",
		},
		{
			name:        "legacy generation f1-micro",
			description: "Cloud SQL for MySQL: db-f1-micro shared-core in Iowa",
			group:       "MySQL",
		},
		{
			name:        "serverless export",
			description: "Cloud SQL for PostgreSQL: Zonal - Serverless Exports in Mexico",
			group:       "ServerlessExport",
		},
		{
			name:        "micro instance",
			description: "Cloud SQL for MySQL: Regional - Micro instance in Milan",
			group:       "SQLGen2InstancesF1Micro",
		},
		{
			name:        "small instance",
			description: "Cloud SQL for MySQL: Regional - Small instance in Dallas",
			group:       "SQLGen2InstancesG1Small",
		},
		{
			name:        "legacy Gen1 tier D32",
			description: "D32",
			group:       "SQLGen1Instances",
		},
		{
			name:        "legacy Gen1 tier D2 usage - hour",
			description: "D2 usage - hour",
			group:       "SQLGen1Instances",
		},
	}

	for _, tc := range outOfScopeSamples {
		t.Run(tc.name, func(t *testing.T) {
			jsonBody := `{
				"skus": [
					{
						"skuId": "SKU-OUT-OF-SCOPE",
						"description": "` + tc.description + `",
						"category": {
							"serviceDisplayName": "Cloud SQL",
							"resourceFamily": "ApplicationServices",
							"resourceGroup": "` + tc.group + `",
							"usageType": "OnDemand"
						},
						"serviceRegions": ["us-east4"],
						"pricingInfo": [
							{
								"pricingExpression": {
									"usageUnit": "h",
									"tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "0", "nanos": 100000000}}]
								}
							}
						]
					}
				]
			}`

			memSink := quarantine.NewMemorySink()
			res, _, err := Normalize(strings.NewReader(jsonBody), fixedTime, memSink)
			if err != nil {
				t.Fatalf("Normalize() error = %v", err)
			}

			if len(res.Observations) != 0 {
				t.Errorf("expected 0 observations for out-of-scope line item, got %d", len(res.Observations))
			}
			if res.IgnoredCount != 1 {
				t.Errorf("expected 1 ignored count, got %d", res.IgnoredCount)
			}
			if memSink.Count() != 0 {
				t.Errorf("expected 0 quarantine items, got %d: %+v", memSink.Count(), memSink.Items())
			}
		})
	}
}

func TestGCPNormalize_Database_GenuinelyNovelSKUQuarantined(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)

	// A genuinely novel/unrecognized database SKU that is NOT out of scope must still quarantine loudly
	jsonBody := `{
		"skus": [
			{
				"skuId": "SKU-GCP-NOVEL-SHAPE",
				"description": "Cloud SQL for PostgreSQL: Quantum Compute Acceleration Unit in Virginia",
				"category": {
					"serviceDisplayName": "Cloud SQL",
					"resourceFamily": "ApplicationServices",
					"resourceGroup": "Quantum",
					"usageType": "OnDemand"
				},
				"serviceRegions": ["us-east4"],
				"pricingInfo": [
					{
						"pricingExpression": {
							"usageUnit": "h",
							"tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "5", "nanos": 0}}]
						}
					}
				]
			}
		]
	}`

	memSink := quarantine.NewMemorySink()
	res, _, err := Normalize(strings.NewReader(jsonBody), fixedTime, memSink)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	if len(res.Observations) != 0 {
		t.Errorf("expected 0 observations for novel unmapped SKU, got %d", len(res.Observations))
	}
	if res.IgnoredCount != 0 {
		t.Errorf("expected 0 ignored items for novel SKU, got %d", res.IgnoredCount)
	}
	if memSink.Count() != 1 {
		t.Fatalf("expected 1 quarantine sink item for novel SKU, got %d", memSink.Count())
	}
	item := memSink.Items()[0]
	if item.Kind != "database_attributes" || item.SkuID != "SKU-GCP-NOVEL-SHAPE" {
		t.Errorf("unexpected quarantine item: %+v", item)
	}
}

func TestGCPNormalize_Database_RealisticMixUnderThreshold(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)

	f, err := os.Open("../../../../testdata/golden/gcp/database.json")
	if err != nil {
		t.Fatalf("failed to open database golden file: %v", err)
	}
	defer func() { _ = f.Close() }()

	memSink := quarantine.NewMemorySink()
	res, _, err := Normalize(f, fixedTime, memSink)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	inScopeTotal := memSink.Count() + len(res.Observations)
	if inScopeTotal == 0 {
		t.Fatalf("expected in-scope total > 0")
	}

	unmappedRatio := float64(memSink.Count()) / float64(inScopeTotal)
	if unmappedRatio > 0.05 {
		t.Errorf("unmapped item ratio %.2f%% exceeds 5.00%% threshold (%d unmapped / %d in-scope, %d ignored)",
			unmappedRatio*100, memSink.Count(), inScopeTotal, res.IgnoredCount)
	}
}
