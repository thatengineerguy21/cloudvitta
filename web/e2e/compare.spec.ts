import { test, expect } from '@playwright/test';

test.describe('Compare Views & Honesty Elements E2E', () => {
  test.beforeEach(async ({ page }) => {
    // Mock health status endpoint
    await page.route('**/api/v1/providers/*/status', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          provider: 'aws',
          status: 'fresh',
          last_fetched_at: new Date().toISOString(),
          total_observations: 1000,
          categories: {},
          dlq: { status: 'healthy', consecutive_failures: 0 },
        }),
      });
    });

    // Mock compute catalog instances endpoint (default empty list, overridable in tests)
    await page.route('**/api/v1/catalog/compute/instances*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          count: 0,
          total: 0,
          instances: [],
        }),
      });
    });

    // Mock compute prices endpoint
    await page.route('**/api/v1/prices/compute*', async (route) => {
      const url = new URL(route.request().url());
      const vcpu = url.searchParams.get('vcpu') || '4';
      const ram = url.searchParams.get('ram_gb') || '16';

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          category: 'compute',
          region: url.searchParams.get('region') || 'us-east',
          currency: 'USD',
          results: [
            {
              provider: 'aws',
              sku_id: 'aws-ec2-t3-medium',
              description: 'AWS t3.medium 2 vCPU 4GB RAM',
              match_quality: 'exact',
              match_delta_pct: '0.00',
              missing_attributes: 0,
              stale: false,
              unit_price: '0.0416',
              unit: '1 Hour',
              normalized_hourly_usd: '0.0416',
              instance_type: 't3.medium',
              matched_spec: {
                vcpu: Number(vcpu),
                ram_gb: Number(ram),
              },
              warnings: [],
            },
            {
              provider: 'gcp',
              sku_id: 'gcp-e2-standard-2',
              description: 'GCP e2-standard-2 2 vCPU 8GB RAM',
              match_quality: 'close',
              match_delta_pct: '12.50',
              missing_attributes: 0,
              stale: false,
              unit_price: '0.0670',
              unit: '1 Hour',
              normalized_hourly_usd: '0.0670',
              instance_type: 'e2-standard-2',
              matched_spec: {
                vcpu: 2,
                ram_gb: 8,
              },
              warnings: [],
            },
            {
              provider: 'azure',
              sku_id: 'azure-b2s',
              description: 'Azure B2s 2 vCPU 4GB RAM',
              match_quality: 'approximate',
              match_delta_pct: '25.00',
              missing_attributes: 1,
              stale: true,
              unit_price: '0.0520',
              unit: '1 Hour',
              normalized_hourly_usd: '0.0520',
              instance_type: 'Standard_B2s',
              matched_spec: {
                vcpu: 2,
                ram_gb: 4,
              },
              warnings: [
                {
                  code: 'stale_pricing_data',
                  message: 'Azure pricing data is stale by >168 hours',
                  provider: 'azure',
                },
              ],
            },
          ],
          warnings: [],
        }),
      });
    });

    // Mock storage prices endpoint
    await page.route('**/api/v1/prices/storage*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          category: 'storage',
          region: 'us-east',
          currency: 'USD',
          results: [
            {
              provider: 'aws',
              sku_id: 'aws-s3-standard',
              description: 'AWS S3 Standard Object Storage',
              match_quality: 'exact',
              match_delta_pct: '0.00',
              missing_attributes: 0,
              stale: false,
              unit_price: '0.0230',
              unit: '1 GB-Mo',
              normalized_hourly_usd: '0.0000315',
              storage_class: 'standard',
              warnings: [],
            },
          ],
          warnings: [],
        }),
      });
    });
  });

  test('should render compute comparison with inputs, results table, and honesty badges', async ({ page }) => {
    await page.goto('/compare/compute');

    // Verify page header
    await expect(page.getByRole('heading', { level: 1, name: /Compute Pricing Comparison/i })).toBeVisible();

    // Verify form controls
    const vcpuInput = page.getByTestId('compute-vcpu-input');
    const ramInput = page.getByTestId('compute-ram-input');
    await expect(vcpuInput).toBeVisible();
    await expect(ramInput).toBeVisible();

    // Verify table rows render
    await expect(page.getByText('aws-ec2-t3-medium')).toBeVisible();
    await expect(page.getByText('gcp-e2-standard-2')).toBeVisible();
    await expect(page.getByText('azure-b2s')).toBeVisible();

    // Verify 3 distinct match quality tiers
    await expect(page.getByText(/Exact Match/i).first()).toBeVisible();
    await expect(page.getByText(/Close Match/i).first()).toBeVisible();
    await expect(page.getByText(/Approximate/i).first()).toBeVisible();

    // Verify stale data badge
    await expect(page.getByText(/STALE/i).first()).toBeVisible();
  });

  test('should synchronize input changes to URL search parameters bidirectionally', async ({ page }) => {
    await page.goto('/compare/compute');

    const vcpuInput = page.getByTestId('compute-vcpu-input');
    await vcpuInput.fill('8');

    // Wait for URL parameter debounce sync
    await expect(page).toHaveURL(/vcpu=8/);

    // Direct navigation with query parameters should hydrate form
    await page.goto('/compare/compute?vcpu=16&ram_gb=64&region=eu-west-1');
    await expect(page.getByTestId('compute-vcpu-input')).toHaveValue('16');
    await expect(page.getByTestId('compute-ram-input')).toHaveValue('64');
  });

  test('should navigate across different compare category tabs', async ({ page }) => {
    await page.goto('/compare/compute');

    // Click Storage link in Header nav
    await page.getByRole('link', { name: /^Storage$/i }).click();
    await expect(page).toHaveURL(/\/compare\/storage/);
    await expect(page.getByRole('heading', { level: 1, name: /Storage Pricing Comparison/i })).toBeVisible();
  });

  test('should allow selecting instance preset from catalog autocomplete on compute compare page', async ({ page }) => {
    await page.route('**/api/v1/catalog/compute/instances*', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          count: 1,
          total: 1,
          instances: [
            {
              id: 1,
              provider: 'aws',
              instance_type_id: 'c6i.large',
              display_name: 'Compute Optimized c6i.large',
              instance_family: 'c6i',
              category: 'compute_optimized',
              vcpu: 2,
              memory_gib: 4,
              cpu_architecture: 'x86_64',
              gpu_count: 0,
              is_burstable: false,
              is_current_gen: true,
              first_seen_at: '2026-01-01T00:00:00Z',
              last_seen_at: '2026-09-04T00:00:00Z',
            },
          ],
        }),
      });
    });

    await page.goto('/compare/compute');

    await expect(page.getByTestId('instance-autocomplete')).toBeVisible();
    await page.getByTestId('instance-autocomplete-trigger').click();
    await expect(page.getByTestId('instance-autocomplete-dropdown')).toBeVisible();
    await page.getByTestId('autocomplete-option-c6i.large').click();

    await expect(page.getByTestId('instance-autocomplete-trigger')).toContainText('c6i.large');
    await expect(page.getByTestId('compute-vcpu-input')).toHaveValue('2');
    await expect(page.getByTestId('compute-ram-input')).toHaveValue('4');
  });

  test('should support multi-region global hubs via region parameter', async ({ page }) => {
    // Navigate with Tokyo hub
    await page.goto('/compare/compute?vcpu=4&ram_gb=16&region=ap-northeast');
    await expect(page).toHaveURL(/region=ap-northeast/);

    // Navigate with Frankfurt hub
    await page.goto('/compare/storage?size_gb=200&region=eu-central');
    await expect(page).toHaveURL(/region=eu-central/);
    await expect(page.getByRole('heading', { level: 1, name: /Storage Pricing Comparison/i })).toBeVisible();
  });
});

