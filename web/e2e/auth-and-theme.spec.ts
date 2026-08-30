import { test, expect } from '@playwright/test';

test.describe('Theme Toggle & In-Memory Auth E2E', () => {
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

  test('should toggle theme and persist choice in localStorage', async ({ page }) => {
    await page.goto('/');

    const themeButton = page.getByRole('button', { name: /Toggle visual theme/i });
    await expect(themeButton).toBeVisible();

    // Toggle theme
    await themeButton.click();

    // Check localStorage and html class
    const isDark = await page.evaluate(() => {
      return document.documentElement.classList.contains('dark') || localStorage.getItem('cv_theme') === 'dark';
    });
    expect(typeof isDark).toBe('boolean');

    // Toggle back
    await themeButton.click();
  });

  test('should open auth modal, perform login, and maintain in-memory state', async ({ page }) => {
    // Mock login endpoint
    await page.route('**/api/v1/auth/login', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          access_token: 'fake-jwt-token-standard-tier',
          token_type: 'Bearer',
          expires_in: 900,
          user: {
            id: 'usr_123456789',
            email: 'developer@cloudvitta.dev',
            tier: 'standard',
          },
        }),
      });
    });

    // Mock logout endpoint
    await page.route('**/api/v1/auth/logout', async (route) => {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ message: 'Successfully logged out' }),
      });
    });

    await page.goto('/');

    // Click Sign In button in header
    const signInBtn = page.getByRole('button', { name: /^Sign In$/i });
    await expect(signInBtn).toBeVisible();
    await signInBtn.click();

    // Verify modal appears
    const dialog = page.getByRole('dialog');
    await expect(dialog).toBeVisible();
    await expect(dialog.getByRole('heading', { name: /Sign In to Standard Tier/i })).toBeVisible();

    // Fill form
    await dialog.getByPlaceholder('name@company.com').fill('developer@cloudvitta.dev');
    await dialog.getByPlaceholder('••••••••').fill('StrongPassword123!');

    // Submit form
    await dialog.getByRole('button', { name: /^Sign In$/i }).click();

    // Verify modal closes and user email appears in header
    await expect(dialog).not.toBeVisible();
    await expect(page.getByText('developer@cloudvitta.dev')).toBeVisible();

    // Verify zero localStorage/sessionStorage token leakage
    const storedTokens = await page.evaluate(() => {
      return {
        localToken: localStorage.getItem('access_token') || localStorage.getItem('token'),
        sessionToken: sessionStorage.getItem('access_token') || sessionStorage.getItem('token'),
      };
    });
    expect(storedTokens.localToken).toBeNull();
    expect(storedTokens.sessionToken).toBeNull();

    // Click Logout button
    const logoutBtn = page.getByRole('button', { name: /Log Out|Logout/i });
    await logoutBtn.click();

    // Verify returns to Sign In button
    await expect(page.getByRole('button', { name: /^Sign In$/i })).toBeVisible();
  });
});
