package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"time"

	gcsstorage "cloud.google.com/go/storage"
	"github.com/redis/go-redis/v9"
	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider"
	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/alibaba"
	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/aws"
	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/azure"
	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/gcp"
	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/ibm"
	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/oracle"
	"github.com/thatengineerguy21/CloudVitta/internal/cache"
	"github.com/thatengineerguy21/CloudVitta/internal/config"
	"github.com/thatengineerguy21/CloudVitta/internal/dlq"
	"github.com/thatengineerguy21/CloudVitta/internal/fx"
	"github.com/thatengineerguy21/CloudVitta/internal/fx/frankfurter"
	"github.com/thatengineerguy21/CloudVitta/internal/observability"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/storage"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("ingestion job failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	serviceName := "cloudvitta-ingest"

	// --- Observability & OpenTelemetry Setup ---
	otelProviders, err := observability.InitOTel(ctx, observability.Config{
		ServiceName: serviceName,
		Endpoint:    cfg.Observability.OTLPEndpoint,
		Headers:     cfg.Observability.OTLPHeaders,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "OpenTelemetry initialization error: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = otelProviders.Shutdown(shutdownCtx)
	}()

	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.Primary.LogLevel)); err != nil {
		level = slog.LevelInfo
	}
	logger := observability.SetupLogger(serviceName, level, otelProviders.LoggerProvider, os.Stdout)
	slog.SetDefault(logger)

	slog.InfoContext(ctx, "starting ingestion job runner...", "environment", cfg.Primary.Environment)

	// --- Database ---
	dbPool, err := store.NewPool(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	defer dbPool.Close()

	// --- Storage ---
	gcsClient, err := gcsstorage.NewClient(ctx)
	if err != nil {
		return fmt.Errorf("failed to create GCS client: %w", err)
	}
	defer func() { _ = gcsClient.Close() }()
	rawStorage := storage.NewGCSStorage(gcsClient, cfg.Storage.GCSBucketName)

	// --- Redis ---
	var redisClient redis.Cmdable
	if cfg.Redis.URL != "" {
		rc, err := cache.NewClient(cfg.Redis.URL)
		if err != nil {
			safeURL := "<invalid-url>"
			if parsed, pErr := url.Parse(cfg.Redis.URL); pErr == nil {
				parsed.User = nil
				safeURL = parsed.String()
			}
			slog.WarnContext(ctx, "failed to connect to redis, proceeding without DLQ/Cache", "url", safeURL, "error", err)
		} else {
			defer func() { _ = rc.Close() }()
			redisClient = rc
		}
	}

	// --- Provider Factory ---
	factory := provider.NewFactory()

	// Register AWS compute adapter with rate limiting and retry config
	awsClient := aws.NewClient()
	awsAdapter := aws.NewAdapter(awsClient, rawStorage, aws.WithCategory("compute"))
	factory.Register(provider.ProviderConfig{
		Provider:       "aws",
		Category:       "compute",
		RateLimitRPS:   10, // AWS bulk API is generous; 10 req/s is safe
		RateLimitBurst: 5,
		Retry:          provider.DefaultRetryConfig(),
	}, awsAdapter)

	// Register AWS storage adapter with rate limiting and retry config
	awsStorageClient := aws.NewClient(aws.WithURL(aws.DefaultS3PriceListURL))
	awsStorageAdapter := aws.NewAdapter(awsStorageClient, rawStorage, aws.WithCategory("storage"))
	factory.Register(provider.ProviderConfig{
		Provider:       "aws",
		Category:       "storage",
		RateLimitRPS:   10,
		RateLimitBurst: 5,
		Retry:          provider.DefaultRetryConfig(),
	}, awsStorageAdapter)

	// Register AWS network adapter with rate limiting and retry config
	awsNetworkClient := aws.NewClient(aws.WithURL(aws.DefaultDataTransferPriceListURL))
	awsNetworkAdapter := aws.NewAdapter(awsNetworkClient, rawStorage, aws.WithCategory("network"))
	factory.Register(provider.ProviderConfig{
		Provider:       "aws",
		Category:       "network",
		RateLimitRPS:   10,
		RateLimitBurst: 5,
		Retry:          provider.DefaultRetryConfig(),
	}, awsNetworkAdapter)

	// Register AWS database adapter with rate limiting and retry config
	awsDBClient := aws.NewClient(aws.WithURL(aws.DefaultRDSPriceListURL))
	awsDBAdapter := aws.NewAdapter(awsDBClient, rawStorage, aws.WithCategory("database_rdbms"))
	factory.Register(provider.ProviderConfig{
		Provider:       "aws",
		Category:       "database_rdbms",
		RateLimitRPS:   10,
		RateLimitBurst: 5,
		Retry:          provider.DefaultRetryConfig(),
	}, awsDBAdapter)

	// Register AWS NoSQL database adapter with rate limiting and retry config
	awsNoSQLDBClient := aws.NewClient(aws.WithURL(aws.DefaultDynamoDBPriceListURL))
	awsNoSQLDBAdapter := aws.NewAdapter(awsNoSQLDBClient, rawStorage, aws.WithCategory("database_nosql"))
	factory.Register(provider.ProviderConfig{
		Provider:       "aws",
		Category:       "database_nosql",
		RateLimitRPS:   10,
		RateLimitBurst: 5,
		Retry:          provider.DefaultRetryConfig(),
	}, awsNoSQLDBAdapter)

	// Register AWS Kubernetes adapter with rate limiting and retry config
	awsKubernetesClient := aws.NewClient(aws.WithURL(aws.DefaultEKSPriceListURL))
	awsKubernetesAdapter := aws.NewAdapter(awsKubernetesClient, rawStorage, aws.WithCategory("kubernetes"))
	factory.Register(provider.ProviderConfig{
		Provider:       "aws",
		Category:       "kubernetes",
		RateLimitRPS:   10,
		RateLimitBurst: 5,
		Retry:          provider.DefaultRetryConfig(),
	}, awsKubernetesAdapter)

	// Register Azure compute adapter with rate limiting and retry config
	azureClient := azure.NewClient()
	azureAdapter := azure.NewAdapter(azureClient, rawStorage, azure.WithCategory("compute"))
	factory.Register(provider.ProviderConfig{
		Provider:       "azure",
		Category:       "compute",
		RateLimitRPS:   10, // Azure Retail Prices API is public; 10 req/s is safe
		RateLimitBurst: 5,
		Retry:          provider.DefaultRetryConfig(),
	}, azureAdapter)

	// Register Azure storage adapter with rate limiting and retry config
	azureStorageClient := azure.NewClient(azure.WithURL(azure.DefaultStorageRetailPricesURL))
	azureStorageAdapter := azure.NewAdapter(azureStorageClient, rawStorage, azure.WithCategory("storage"))
	factory.Register(provider.ProviderConfig{
		Provider:       "azure",
		Category:       "storage",
		RateLimitRPS:   10,
		RateLimitBurst: 5,
		Retry:          provider.DefaultRetryConfig(),
	}, azureStorageAdapter)

	// Register Azure network adapter with rate limiting and retry config
	azureNetworkClient := azure.NewClient(azure.WithURL(azure.DefaultNetworkRetailPricesURL))
	azureNetworkAdapter := azure.NewAdapter(azureNetworkClient, rawStorage, azure.WithCategory("network"))
	factory.Register(provider.ProviderConfig{
		Provider:       "azure",
		Category:       "network",
		RateLimitRPS:   10,
		RateLimitBurst: 5,
		Retry:          provider.DefaultRetryConfig(),
	}, azureNetworkAdapter)

	// Register Azure database adapter with rate limiting and retry config
	azureDBClient := azure.NewClient(azure.WithURL(azure.DefaultDatabaseRetailPricesURL))
	azureDBAdapter := azure.NewAdapter(azureDBClient, rawStorage, azure.WithCategory("database_rdbms"))
	factory.Register(provider.ProviderConfig{
		Provider:       "azure",
		Category:       "database_rdbms",
		RateLimitRPS:   10,
		RateLimitBurst: 5,
		Retry:          provider.DefaultRetryConfig(),
	}, azureDBAdapter)

	// Register Azure NoSQL database adapter with rate limiting and retry config
	azureNoSQLDBClient := azure.NewClient(azure.WithURL(azure.DefaultCosmosDBRetailPricesURL))
	azureNoSQLDBAdapter := azure.NewAdapter(azureNoSQLDBClient, rawStorage, azure.WithCategory("database_nosql"))
	factory.Register(provider.ProviderConfig{
		Provider:       "azure",
		Category:       "database_nosql",
		RateLimitRPS:   10,
		RateLimitBurst: 5,
		Retry:          provider.DefaultRetryConfig(),
	}, azureNoSQLDBAdapter)

	// Register Azure Kubernetes adapter with rate limiting and retry config
	azureKubernetesClient := azure.NewClient(azure.WithURL(azure.DefaultKubernetesRetailPricesURL))
	azureKubernetesAdapter := azure.NewAdapter(azureKubernetesClient, rawStorage, azure.WithCategory("kubernetes"))
	factory.Register(provider.ProviderConfig{
		Provider:       "azure",
		Category:       "kubernetes",
		RateLimitRPS:   10,
		RateLimitBurst: 5,
		Retry:          provider.DefaultRetryConfig(),
	}, azureKubernetesAdapter)

	// Register GCP compute adapter with rate limiting and retry config
	var gcpOpts []gcp.Option
	if cfg.GCP.APIKey != "" {
		gcpOpts = append(gcpOpts, gcp.WithAPIKey(cfg.GCP.APIKey))
	}
	gcpClient := gcp.NewClient(gcpOpts...)
	gcpAdapter := gcp.NewAdapter(gcpClient, rawStorage, gcp.WithCategory("compute"))
	factory.Register(provider.ProviderConfig{
		Provider:       "gcp",
		Category:       "compute",
		RateLimitRPS:   10, // GCP Cloud Billing API standard rate limit
		RateLimitBurst: 5,
		Retry:          provider.DefaultRetryConfig(),
	}, gcpAdapter)

	// Register GCP storage adapter with rate limiting and retry config
	var gcpStorageOpts []gcp.Option
	if cfg.GCP.APIKey != "" {
		gcpStorageOpts = append(gcpStorageOpts, gcp.WithAPIKey(cfg.GCP.APIKey))
	}
	gcpStorageOpts = append(gcpStorageOpts, gcp.WithURL(gcp.DefaultStorageBillingCatalogURL))
	gcpStorageClient := gcp.NewClient(gcpStorageOpts...)
	gcpStorageAdapter := gcp.NewAdapter(gcpStorageClient, rawStorage, gcp.WithCategory("storage"))
	factory.Register(provider.ProviderConfig{
		Provider:       "gcp",
		Category:       "storage",
		RateLimitRPS:   10,
		RateLimitBurst: 5,
		Retry:          provider.DefaultRetryConfig(),
	}, gcpStorageAdapter)

	// Register GCP network adapter with rate limiting and retry config
	var gcpNetworkOpts []gcp.Option
	if cfg.GCP.APIKey != "" {
		gcpNetworkOpts = append(gcpNetworkOpts, gcp.WithAPIKey(cfg.GCP.APIKey))
	}
	gcpNetworkOpts = append(gcpNetworkOpts, gcp.WithURL(gcp.DefaultNetworkBillingCatalogURL))
	gcpNetworkClient := gcp.NewClient(gcpNetworkOpts...)
	gcpNetworkAdapter := gcp.NewAdapter(gcpNetworkClient, rawStorage, gcp.WithCategory("network"))
	factory.Register(provider.ProviderConfig{
		Provider:       "gcp",
		Category:       "network",
		RateLimitRPS:   10,
		RateLimitBurst: 5,
		Retry:          provider.DefaultRetryConfig(),
	}, gcpNetworkAdapter)

	// Register GCP database adapter with rate limiting and retry config
	var gcpDBOpts []gcp.Option
	if cfg.GCP.APIKey != "" {
		gcpDBOpts = append(gcpDBOpts, gcp.WithAPIKey(cfg.GCP.APIKey))
	}
	gcpDBOpts = append(gcpDBOpts, gcp.WithURL(gcp.DefaultDatabaseBillingCatalogURL))
	gcpDBClient := gcp.NewClient(gcpDBOpts...)
	gcpDBAdapter := gcp.NewAdapter(gcpDBClient, rawStorage, gcp.WithCategory("database_rdbms"))
	factory.Register(provider.ProviderConfig{
		Provider:       "gcp",
		Category:       "database_rdbms",
		RateLimitRPS:   10,
		RateLimitBurst: 5,
		Retry:          provider.DefaultRetryConfig(),
	}, gcpDBAdapter)

	// Register GCP NoSQL database adapter with rate limiting and retry config
	var gcpNoSQLDBOpts []gcp.Option
	if cfg.GCP.APIKey != "" {
		gcpNoSQLDBOpts = append(gcpNoSQLDBOpts, gcp.WithAPIKey(cfg.GCP.APIKey))
	}
	gcpNoSQLDBOpts = append(gcpNoSQLDBOpts, gcp.WithURL(gcp.DefaultNoSQLDatabaseBillingCatalogURL))
	gcpNoSQLDBClient := gcp.NewClient(gcpNoSQLDBOpts...)
	gcpNoSQLDBAdapter := gcp.NewAdapter(gcpNoSQLDBClient, rawStorage, gcp.WithCategory("database_nosql"))
	factory.Register(provider.ProviderConfig{
		Provider:       "gcp",
		Category:       "database_nosql",
		RateLimitRPS:   10,
		RateLimitBurst: 5,
		Retry:          provider.DefaultRetryConfig(),
	}, gcpNoSQLDBAdapter)

	// Register GCP Kubernetes adapter with rate limiting and retry config
	var gcpKubernetesOpts []gcp.Option
	if cfg.GCP.APIKey != "" {
		gcpKubernetesOpts = append(gcpKubernetesOpts, gcp.WithAPIKey(cfg.GCP.APIKey))
	}
	gcpKubernetesOpts = append(gcpKubernetesOpts, gcp.WithURL(gcp.DefaultKubernetesBillingCatalogURL))
	gcpKubernetesClient := gcp.NewClient(gcpKubernetesOpts...)
	gcpKubernetesAdapter := gcp.NewAdapter(gcpKubernetesClient, rawStorage, gcp.WithCategory("kubernetes"))
	factory.Register(provider.ProviderConfig{
		Provider:       "gcp",
		Category:       "kubernetes",
		RateLimitRPS:   10,
		RateLimitBurst: 5,
		Retry:          provider.DefaultRetryConfig(),
	}, gcpKubernetesAdapter)

	// Register Oracle compute adapter with rate limiting and retry config
	oracleClient := oracle.NewClient()
	oracleAdapter := oracle.NewAdapter(oracleClient, rawStorage, oracle.WithCategory("compute"))
	factory.Register(provider.ProviderConfig{
		Provider:       "oracle",
		Category:       "compute",
		RateLimitRPS:   10,
		RateLimitBurst: 5,
		Retry:          provider.DefaultRetryConfig(),
	}, oracleAdapter)

	// Register IBM compute adapter with rate limiting and retry config
	var ibmOpts []ibm.Option
	if cfg.IBM.APIKey != "" {
		ibmOpts = append(ibmOpts, ibm.WithAPIKey(cfg.IBM.APIKey))
	}
	ibmClient := ibm.NewClient(ibmOpts...)
	ibmAdapter := ibm.NewAdapter(ibmClient, rawStorage, ibm.WithCategory("compute"))
	factory.Register(provider.ProviderConfig{
		Provider:       "ibm",
		Category:       "compute",
		RateLimitRPS:   10,
		RateLimitBurst: 5,
		Retry:          provider.DefaultRetryConfig(),
	}, ibmAdapter)

	// Register Alibaba compute adapter with rate limiting and retry config
	var aliOpts []alibaba.Option
	if cfg.Alibaba.AccessKeyID != "" && cfg.Alibaba.AccessKeySecret != "" {
		aliOpts = append(aliOpts, alibaba.WithCredentials(cfg.Alibaba.AccessKeyID, cfg.Alibaba.AccessKeySecret))
	}
	aliClient := alibaba.NewClient(aliOpts...)
	aliAdapter := alibaba.NewAdapter(aliClient, rawStorage, alibaba.WithCategory("compute"))
	factory.Register(provider.ProviderConfig{
		Provider:       "alibaba",
		Category:       "compute",
		RateLimitRPS:   10,
		RateLimitBurst: 5,
		Retry:          provider.DefaultRetryConfig(),
	}, aliAdapter)

	// --- DLQ ---
	var dlqSvc *dlq.DLQ
	if redisClient != nil {
		dlqSvc = dlq.New(redisClient)
	}

	queries := store.New(dbPool)

	// --- FX Rates Synchronization ---
	frankfurterClient := frankfurter.NewClient()
	fxSvc := fx.NewService(queries, frankfurterClient)
	slog.InfoContext(ctx, "syncing daily fx rates...")
	if err := fxSvc.RefreshRates(ctx); err != nil {
		slog.WarnContext(ctx, "fx rate synchronization failed during ingestion run, operating with fallback rates", "error", err)
	} else {
		slog.InfoContext(ctx, "daily fx rates synchronized successfully")
	}

	// --- Orchestrator ---
	orchConfig := service.DefaultOrchestratorConfig()
	orchConfig.Tracer = otelProviders.Tracer
	orchConfig.Meter = otelProviders.Meter

	orchestrator := service.NewOrchestrator(
		queries,
		redisClient,
		dlqSvc,
		factory,
		orchConfig,
	)

	// --- Execution ---
	slog.InfoContext(ctx, "starting orchestrated ingestion run...")
	results := orchestrator.RunAll(ctx)

	// Report results
	var hasErrors bool
	for _, r := range results {
		if r.Skipped {
			slog.InfoContext(ctx, "ingestion job skipped (lock held)",
				"provider", r.Provider,
				"category", r.Category,
			)
			continue
		}
		if r.Err != nil {
			slog.ErrorContext(ctx, "ingestion job failed",
				"provider", r.Provider,
				"category", r.Category,
				"error", r.Err,
			)
			hasErrors = true
			continue
		}
		slog.InfoContext(ctx, "ingestion job completed",
			"provider", r.Provider,
			"category", r.Category,
			"inserted", r.InsertedCount,
			"anomalies", r.AnomalyCount,
		)
	}

	if hasErrors {
		return fmt.Errorf("one or more ingestion jobs failed")
	}

	slog.InfoContext(ctx, "all ingestion jobs completed successfully")
	return nil
}
