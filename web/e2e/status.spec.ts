import { test, expect } from '@playwright/test';

test.describe('Provider Status & Telemetry E2E', () => {
  test('should render 7 provider status cards with freshness and observation telemetry', async ({ page }) => {
    // Mock 7 provider status endpoints
    await page.route('**/api/v1/providers/*/status', async (route) => {
      const match = route.request().url().match(/\/providers\/([^/]+)\/status/);
      const provider = match ? match[1] : 'aws';

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          provider,
          status: 'healthy',
          last_successful_fetch: new Date().toISOString(),
          categories: {
            compute: { supported: true, stale: false, observation_count: 1200, last_fetched_at: new Date().toISOString() },
            storage: { supported: true, stale: false, observation_count: 850, last_fetched_at: new Date().toISOString() },
            network: { supported: true, stale: false, observation_count: 400, last_fetched_at: new Date().toISOString() },
          },
          dlq: { status: 'healthy', consecutive_failures: 0 },
        }),
      });
    });

    await page.goto('/status');

    await expect(page.getByRole('heading', { level: 1, name: /Provider Operational Status/i })).toBeVisible();

    // Verify all 7 providers are represented
    const providers = ['aws', 'azure', 'gcp', 'oracle', 'ibm', 'alibaba', 'digitalocean'];
    for (const providerId of providers) {
      await expect(page.getByTestId(`provider-card-${providerId}`)).toBeVisible();
    }

    // Verify observation counts and freshness labels
    await expect(page.getByText(/1200 obs/i).first()).toBeVisible();
    await expect(page.getByText(/Fresh/i).first()).toBeVisible();
  });

  test('should isolate single provider failure within CardErrorBoundary without crashing page', async ({ page }) => {
    // Mock 6 providers as healthy, but fail alibaba
    await page.route('**/api/v1/providers/*/status', async (route) => {
      const url = route.request().url();
      if (url.includes('/providers/alibaba/')) {
        await route.fulfill({
          status: 503,
          contentType: 'application/problem+json',
          body: JSON.stringify({
            type: 'https://cloudvitta.dev/errors/provider-unavailable',
            title: 'Provider Unavailable',
            status: 503,
            detail: 'Alibaba Cloud pricing API is temporarily unreachable',
          }),
        });
        return;
      }

      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          provider: 'aws',
          status: 'healthy',
          last_successful_fetch: new Date().toISOString(),
          categories: {
            compute: { supported: true, stale: false, observation_count: 1500, last_fetched_at: new Date().toISOString() },
          },
          dlq: { status: 'healthy', consecutive_failures: 0 },
        }),
      });
    });

    await page.goto('/status');

    // Verify page didn't crash and other providers are visible
    await expect(page.getByRole('heading', { level: 1, name: /Provider Operational Status/i })).toBeVisible();
    await expect(page.getByTestId('provider-card-aws')).toBeVisible();

    // Verify isolated error boundary display on failed provider card
    await expect(page.getByTestId('provider-error-alibaba')).toBeVisible();
    await expect(page.getByRole('button', { name: /Retry/i })).toBeVisible();
  });

  test('should render compute catalog summary inventory card', async ({ page }) => {
    await page.route('**/api/v1/providers/*/status', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          provider: 'aws',
          status: 'healthy',
          categories: {},
          dlq: { status: 'healthy', consecutive_failures: 0 },
        }),
      });
    });

    await page.route('**/api/v1/catalog/compute/summary', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          total_instances: 3200,
          provider_totals: { aws: 1400, azure: 1100, gcp: 700 },
          category_breakdown: {
            aws: { general_purpose: 500, compute_optimized: 400 },
          },
        }),
      });
    });

    await page.goto('/status');

    await expect(page.getByTestId('compute-catalog-summary-card')).toBeVisible();
    await expect(page.getByTestId('catalog-total-instances')).toContainText('3,200');
    await expect(page.getByTestId('catalog-provider-total-aws')).toContainText('1,400');
  });
});
