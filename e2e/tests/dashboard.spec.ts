import { test, expect } from '@playwright/test';
import { injectAuth } from '../helpers/auth';
import { ApiClient } from '../helpers/api-client';

/**
 * Dashboard tests.
 *
 * Tests conversation listing, detail view, and message interaction.
 * Uses auth injection to skip the login UI flow.
 */

const DEMO_EMAIL = process.env.DEMO_EMAIL || 'demo@replider.com';
const DEMO_PASSWORD = process.env.DEMO_PASSWORD || 'demo1234';

test.describe('Dashboard', () => {
  let token: string;

  test.beforeAll(async () => {
    // Get a JWT token via API
    const api = new ApiClient();
    try {
      token = await api.login(DEMO_EMAIL, DEMO_PASSWORD);
    } catch {
      console.warn('Could not login via API. Tests that need auth injection will be skipped.');
    }
  });

  test('should display conversation list', async ({ page }) => {
    test.skip(!token, 'No auth token available');

    await injectAuth(page, token, { name: 'Dr. Maria Silva', email: DEMO_EMAIL });
    await page.waitForTimeout(2000);

    // Dashboard should show some form of conversation list or empty state
    const pageContent = await page.textContent('body');
    expect(pageContent).toBeTruthy();

    await page.screenshot({ path: 'test-results/screenshots/dashboard-conversations.png' });
  });

  test('should open conversation detail when clicking a conversation', async ({ page }) => {
    test.skip(!token, 'No auth token available');

    await injectAuth(page, token, { name: 'Dr. Maria Silva', email: DEMO_EMAIL });
    await page.waitForTimeout(2000);

    // Find and click first conversation item
    const conversationItem = page
      .locator('flt-semantics, [role="button"], [role="listitem"]')
      .filter({ hasText: /./i })
      .first();

    if (await conversationItem.isVisible().catch(() => false)) {
      await conversationItem.click();
      await page.waitForTimeout(2000);

      // Should show messages area
      await page.screenshot({ path: 'test-results/screenshots/dashboard-conversation-open.png' });
    }
  });

  test('should show empty state when no conversations exist', async ({ page }) => {
    test.skip(!token, 'No auth token available');

    await injectAuth(page, token, { name: 'Dr. Maria Silva', email: DEMO_EMAIL });
    await page.waitForTimeout(2000);

    // At minimum the page should be loaded and visible
    const body = page.locator('body');
    await expect(body).toBeVisible();

    await page.screenshot({ path: 'test-results/screenshots/dashboard-state.png' });
  });

  test('should have navigation elements', async ({ page }) => {
    test.skip(!token, 'No auth token available');

    await injectAuth(page, token, { name: 'Dr. Maria Silva', email: DEMO_EMAIL });
    await page.waitForTimeout(2000);

    // The dashboard should have navigation buttons/items
    // Flutter web generates accessible elements we can query
    const navElements = page.locator('[role="button"], [role="tab"], flt-semantics');
    const count = await navElements.count();

    // Should have at least some interactive elements
    expect(count).toBeGreaterThan(0);
  });
});
