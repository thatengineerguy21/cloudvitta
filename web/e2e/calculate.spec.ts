import { test, expect } from '@playwright/test';

test.describe('Workload Calculator E2E', () => {
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
  });

  test('should render workload builder and calculate complete totals with solid border', async ({ page }) => {
    // Mock successful complete calculate response
    await page.route('**/api/v1/calculate', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          currency: 'USD',
          results: [
            {
              provider: 'aws',
              partial: false,
              total_normalized_hourly_usd: '0.1250',
              breakdown: {
                compute: {
                  category: 'compute',
                  normalized_hourly_usd: '0.0800',
                  provider: 'aws',
                  sku_id: 'aws-ec2-t3-large',
                  match_quality: 'exact',
                  stale: false,
                  warnings: [],
                },
                storage: {
                  category: 'storage',
                  normalized_hourly_usd: '0.0450',
                  provider: 'aws',
                  sku_id: 'aws-s3-standard',
                  match_quality: 'exact',
                  stale: false,
                  warnings: [],
                },
              },
              warnings: [],
            },
          ],
          warnings: [],
        }),
      });
    });

    // Navigate with compute & storage enabled
    await page.goto('/calculate?compute_enabled=true&compute_vcpu=4&compute_ram_gb=16&storage_enabled=true&storage_size_gb=500');

    await expect(page.getByRole('heading', { level: 1, name: /Composite Workload Calculator/i })).toBeVisible();

    // Verify complete total is displayed
    await expect(page.getByText(/0\.1250/)).toBeVisible();
    await expect(page.getByText(/Complete Workload Total/i)).toBeVisible();
  });

  test('should render partial total with dashed border and warning under ADR 0022 honesty rules', async ({ page }) => {
    // Mock partial calculate response
    await page.route('**/api/v1/calculate', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          currency: 'USD',
          results: [
            {
              provider: 'aws',
              partial: true,
              partial_total_normalized_hourly_usd: '0.0800',
              breakdown: {
                compute: {
                  category: 'compute',
                  normalized_hourly_usd: '0.0800',
                  provider: 'aws',
                  sku_id: 'aws-ec2-t3-large',
                  match_quality: 'exact',
                  stale: false,
                  warnings: [],
                },
              },
              warnings: [
                {
                  code: 'partial_provider_failure',
                  message: 'Storage pricing unavailable for requested provider; total is incomplete',
                },
              ],
            },
          ],
          warnings: [
            {
              code: 'partial_provider_failure',
              message: 'Storage pricing unavailable for requested provider; total is incomplete',
            },
          ],
        }),
      });
    });

    await page.goto('/calculate?compute_enabled=true&compute_vcpu=4&compute_ram_gb=16&storage_enabled=true&storage_size_gb=500');

    // Verify partial honesty display
    await expect(page.getByText(/Partial Estimate:/i)).toBeVisible();
    await expect(page.getByText(/0\.0800/)).toBeVisible();
    await expect(page.getByText(/Storage pricing unavailable/i).first()).toBeVisible();
  });
});
