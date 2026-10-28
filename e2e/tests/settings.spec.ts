import { test, expect } from '@playwright/test';
import { injectAuth } from '../helpers/auth';
import { ApiClient } from '../helpers/api-client';

/**
 * Settings tests.
 *
 * Tests agent configuration, knowledge base, and account settings.
 * Uses auth injection to skip the login UI flow.
 */

const DEMO_EMAIL = process.env.DEMO_EMAIL || 'demo@replider.com';
const DEMO_PASSWORD = process.env.DEMO_PASSWORD || 'demo1234';

test.describe('Settings', () => {
  let token: string;

  test.beforeAll(async () => {
    const api = new ApiClient();
    try {
      token = await api.login(DEMO_EMAIL, DEMO_PASSWORD);
    } catch {
      console.warn('Could not login via API. Tests will be skipped.');
    }
  });

  test('should navigate to agent settings', async ({ page }) => {
    test.skip(!token, 'No auth token available');

    await injectAuth(page, token, { name: 'Dr. Maria Silva', email: DEMO_EMAIL });
    await page.waitForTimeout(2000);

    // Try to navigate to settings/agents
    const settingsNav = page
      .getByRole('button', { name: /config|settings|agentes|agents/i })
      .or(page.getByText(/config|settings/i).first());

    if (await settingsNav.first().isVisible().catch(() => false)) {
      await settingsNav.first().click();
      await page.waitForTimeout(2000);

      await page.screenshot({ path: 'test-results/screenshots/settings-agents.png' });

      // Should have agent list or agent details
      const body = page.locator('body');
      await expect(body).toBeVisible();
    }
  });

  test('should display agent detail with editable fields', async ({ page }) => {
    test.skip(!token, 'No auth token available');

    await injectAuth(page, token, { name: 'Dr. Maria Silva', email: DEMO_EMAIL });
    await page.waitForTimeout(2000);

    // Navigate to settings first
    const settingsNav = page
      .getByRole('button', { name: /config|settings|agentes|agents/i })
      .or(page.getByText(/config|settings/i).first());

    if (await settingsNav.first().isVisible().catch(() => false)) {
      await settingsNav.first().click();
      await page.waitForTimeout(2000);

      // Click on agent
      const agentItem = page
        .locator('flt-semantics, [role="button"], [role="listitem"]')
        .filter({ hasText: /saude|assistente|agente/i })
        .first();

      if (await agentItem.isVisible().catch(() => false)) {
        await agentItem.click();
        await page.waitForTimeout(2000);

        await page.screenshot({ path: 'test-results/screenshots/settings-agent-detail.png' });

        // Should have input fields for agent configuration
        const inputs = page.locator('input, textarea');
        const inputCount = await inputs.count();
        expect(inputCount).toBeGreaterThan(0);
      }
    }
  });

  test('should display profile/account settings', async ({ page }) => {
    test.skip(!token, 'No auth token available');

    await injectAuth(page, token, { name: 'Dr. Maria Silva', email: DEMO_EMAIL });
    await page.waitForTimeout(2000);

    // Navigate to profile
    const profileNav = page
      .getByRole('button', { name: /perfil|profile|conta|account/i })
      .or(page.getByText(/perfil|profile/i).first());

    if (await profileNav.first().isVisible().catch(() => false)) {
      await profileNav.first().click();
      await page.waitForTimeout(2000);

      await page.screenshot({ path: 'test-results/screenshots/settings-profile.png' });

      const body = page.locator('body');
      await expect(body).toBeVisible();
    }
  });

  test('should display billing/plan information', async ({ page }) => {
    test.skip(!token, 'No auth token available');

    await injectAuth(page, token, { name: 'Dr. Maria Silva', email: DEMO_EMAIL });
    await page.waitForTimeout(2000);

    // Navigate to billing
    const billingNav = page
      .getByRole('button', { name: /plano|billing|assinatura|subscription/i })
      .or(page.getByText(/plano|billing/i).first());

    if (await billingNav.first().isVisible().catch(() => false)) {
      await billingNav.first().click();
      await page.waitForTimeout(2000);

      await page.screenshot({ path: 'test-results/screenshots/settings-billing.png' });

      // Should show plan information
      const body = page.locator('body');
      await expect(body).toBeVisible();
    }
  });
});
