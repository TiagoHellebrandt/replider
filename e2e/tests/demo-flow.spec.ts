import { test, expect } from '@playwright/test';

/**
 * Demo Flow - Main video recording test.
 *
 * Flutter Web (CanvasKit) renders everything onto a <canvas> element.
 * Text fields only appear as real <input> elements AFTER the user clicks
 * on them — before that, they exist only in Flutter's semantics layer as
 * [role="textbox"] elements.
 *
 * Interaction pattern:
 *   1. Wait for flt-glass-pane  (Flutter engine loaded)
 *   2. Click [role="textbox"]   (Flutter receives click, creates <input>)
 *   3. keyboard.type()          (types into the native <input>)
 *
 * Run:
 *   npm run test:demo   # headed, desktop only
 *   npm run test:video  # headless with video capture
 */

const DEMO_EMAIL = process.env.DEMO_EMAIL || 'demo@replider.com';
const DEMO_PASSWORD = process.env.DEMO_PASSWORD || 'demo1234';

/** Wait for Flutter engine to finish booting and rendering its first frame. */
async function waitForFlutter(page: import('@playwright/test').Page) {
  await page.waitForSelector('flt-glass-pane', { timeout: 30_000 });
  // Extra settle time for CanvasKit to compile shaders and layout
  await page.waitForTimeout(3_000);
}

test.describe('Demo Flow', () => {
  test('full dashboard walkthrough', async ({ page }) => {
    // ---------------------------------------------------------------
    // 1. Login
    // ---------------------------------------------------------------
    test.info().annotations.push({ type: 'step', description: 'Login to dashboard' });

    await page.goto('/');
    await waitForFlutter(page);

    await page.screenshot({ path: 'test-results/screenshots/01-login-page.png' });

    // Flutter CanvasKit exposes text fields via [role="textbox"] in its semantics layer.
    // Clicking triggers Flutter to create a real <input> element for keyboard input.
    const textboxes = page.locator('[role="textbox"]');
    await textboxes.first().waitFor({ state: 'attached', timeout: 15_000 });

    // Fill email
    await textboxes.first().click();
    await page.waitForTimeout(400);
    await page.keyboard.type(DEMO_EMAIL);
    await page.waitForTimeout(300);

    // Fill password
    await textboxes.nth(1).click();
    await page.waitForTimeout(400);
    await page.keyboard.type(DEMO_PASSWORD);
    await page.waitForTimeout(300);

    await page.screenshot({ path: 'test-results/screenshots/02-login-filled.png' });

    // Click login button via Flutter semantics
    const loginButton = page.getByRole('button', { name: /login|entrar|sign in/i });
    await loginButton.waitFor({ state: 'attached', timeout: 10_000 });
    await loginButton.click();

    // Wait for navigation to dashboard
    await page.waitForURL('**/home', { timeout: 25_000 });
    await page.waitForTimeout(3_000);

    // ---------------------------------------------------------------
    // 2. Dashboard overview
    // ---------------------------------------------------------------
    test.info().annotations.push({ type: 'step', description: 'View dashboard' });

    await page.screenshot({ path: 'test-results/screenshots/03-dashboard.png' });
    await page.waitForTimeout(2_000);

    // ---------------------------------------------------------------
    // 3. Conversations
    // ---------------------------------------------------------------
    test.info().annotations.push({ type: 'step', description: 'Browse conversations' });

    const conversationItem = page
      .locator('[role="button"], [role="listitem"]')
      .filter({ hasText: /conversa|mensagem|cliente/i })
      .first();

    if (await conversationItem.isVisible().catch(() => false)) {
      await conversationItem.click();
      await page.waitForTimeout(2_000);
      await page.screenshot({ path: 'test-results/screenshots/04-conversation-detail.png' });
      await page.waitForTimeout(1_500);
    } else {
      await page.screenshot({ path: 'test-results/screenshots/04-no-conversations.png' });
      await page.waitForTimeout(1_000);
    }

    // ---------------------------------------------------------------
    // 4. Navigate to Settings
    // ---------------------------------------------------------------
    test.info().annotations.push({ type: 'step', description: 'Navigate to settings' });

    const settingsNav = page
      .getByRole('button', { name: /config|settings|agentes|agents/i })
      .or(page.getByText(/settings/i).first());

    if (await settingsNav.first().isVisible().catch(() => false)) {
      await settingsNav.first().click();
      await page.waitForTimeout(2_000);
      await page.screenshot({ path: 'test-results/screenshots/05-settings.png' });
      await page.waitForTimeout(1_500);
    }

    // ---------------------------------------------------------------
    // 5. Agent detail
    // ---------------------------------------------------------------
    test.info().annotations.push({ type: 'step', description: 'View agent detail' });

    const agentItem = page
      .locator('[role="button"], [role="listitem"]')
      .filter({ hasText: /saude|assistente|agente/i })
      .first();

    if (await agentItem.isVisible().catch(() => false)) {
      await agentItem.click();
      await page.waitForTimeout(2_000);
      await page.screenshot({ path: 'test-results/screenshots/06-agent-detail.png' });
      await page.waitForTimeout(1_500);

      const backButton = page.getByRole('button', { name: /voltar|back/i }).first();
      if (await backButton.isVisible().catch(() => false)) {
        await backButton.click();
        await page.waitForTimeout(1_000);
      }
    }

    // ---------------------------------------------------------------
    // 6. Return to dashboard
    // ---------------------------------------------------------------
    test.info().annotations.push({ type: 'step', description: 'Return to dashboard' });

    const homeNav = page
      .getByRole('button', { name: /home|inicio|dashboard|conversas/i })
      .or(page.getByText(/home|inicio/i).first());

    if (await homeNav.first().isVisible().catch(() => false)) {
      await homeNav.first().click();
      await page.waitForTimeout(2_000);
    } else {
      await page.goto('/');
      await waitForFlutter(page);
    }

    await page.screenshot({ path: 'test-results/screenshots/07-final-dashboard.png' });
    await page.waitForTimeout(2_000);

    await expect(page).toHaveURL(/home|dashboard|\//);
  });
});
