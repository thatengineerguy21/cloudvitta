import { test, expect } from '@playwright/test';

test.describe('Landing Page E2E', () => {
  test.beforeEach(async ({ page }) => {
    // Mock health status endpoint for header telemetry
    await page.route('**/api/v1/providers/*/status', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          provider: 'aws',
          status: 'fresh',
          last_fetched_at: new Date().toISOString(),
          total_observations: 1250,
          categories: {
            compute: { status: 'fresh', observation_count: 500, last_fetched_at: new Date().toISOString() },
          },
          dlq: { status: 'healthy', consecutive_failures: 0 },
        }),
      });
    });
  });

  test('should render the landing page hero, brand, and layout', async ({ page }) => {
    await page.goto('/');

    // Check header branding
    await expect(page.getByRole('link', { name: /CloudVitta/i })).toBeVisible();

    // Check hero headline and subhead
    await expect(page.getByRole('heading', { level: 1, name: /Normalized Cloud Infrastructure Pricing/i })).toBeVisible();
    await expect(page.getByText(/Compare and calculate cloud infrastructure costs across 7 providers/i)).toBeVisible();

    // Check dual workload CTAs
    const compareCta = page.getByRole('link', { name: /Select Category/i });
    const calculateCta = page.getByRole('link', { name: /Build Workload/i });

    await expect(compareCta).toBeVisible();
    await expect(calculateCta).toBeVisible();

    // Check Swagger API documentation and GitHub links
    await expect(page.getByRole('link', { name: /Swagger API Docs/i })).toBeVisible();
    await expect(page.getByRole('link', { name: /GitHub/i }).first()).toBeVisible();
  });

  test('should navigate to Category Compare from Header or Category Card', async ({ page }) => {
    await page.goto('/');
    // Click compute in header or category grid
    await page.getByRole('link', { name: /^Compute$/i }).click();
    await expect(page).toHaveURL(/\/compare\/compute/);
  });

  test('should navigate to Workload Calculator from Hero CTA', async ({ page }) => {
    await page.goto('/');
    await page.getByRole('link', { name: /Build Workload/i }).click();
    await expect(page).toHaveURL(/\/calculate/);
  });

  test('should render all 7 category Bento cards and navigate correctly', async ({ page }) => {
    await page.goto('/');

    // Check all 7 categories exist in Bento grid
    const categoryTitles = [
      'Compute Instances',
      'Storage Classes',
      'Network Egress',
      'Relational DBs (RDBMS)',
      'NoSQL Databases',
      'Kubernetes Control-Plane',
      'Serverless Compute (FaaS)',
    ];

    for (const title of categoryTitles) {
      await expect(page.getByText(title).first()).toBeVisible();
    }

    // Click storage card Open Comparison link and verify navigation
    const storageCard = page.locator('#category-grid').getByRole('link', { name: /Open Comparison/i }).nth(1);
    await storageCard.click();
    await expect(page).toHaveURL(/\/compare\/storage/);
  });
});
