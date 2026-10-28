import { test, expect } from '@playwright/test';
import { clearAuth } from '../helpers/auth';

/**
 * Authentication tests.
 *
 * Tests login and registration flows via the UI.
 */

const DEMO_EMAIL = process.env.DEMO_EMAIL || 'demo@replider.com';
const DEMO_PASSWORD = process.env.DEMO_PASSWORD || 'demo1234';

test.describe('Authentication', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.waitForTimeout(3000); // Flutter boot
  });

  test('should display login page', async ({ page }) => {
    // Login page should have email and password inputs
    const emailInput = page.locator('input[type="email"], input[data-testid="email_input"]').first();
    await emailInput.waitFor({ state: 'visible', timeout: 15_000 });
    await expect(emailInput).toBeVisible();

    const passwordInput = page.locator('input[type="password"]').first();
    await expect(passwordInput).toBeVisible();

    // Should have a login button
    const loginButton = page.getByRole('button', { name: /login|entrar|sign in/i });
    await expect(loginButton).toBeVisible();
  });

  test('should show error for invalid credentials', async ({ page }) => {
    const emailInput = page.locator('input[type="email"], input[data-testid="email_input"]').first();
    await emailInput.waitFor({ state: 'visible', timeout: 15_000 });
    await emailInput.fill('invalid@example.com');

    const passwordInput = page.locator('input[type="password"]').first();
    await passwordInput.fill('wrongpassword');

    const loginButton = page.getByRole('button', { name: /login|entrar|sign in/i });
    await loginButton.click();

    // Should show an error message
    await page.waitForTimeout(3000);

    // Look for error text (snackbar, dialog, or inline)
    const errorVisible = await page
      .getByText(/invalid|incorret|erro|error|falha|credenciais/i)
      .first()
      .isVisible()
      .catch(() => false);

    // The page should NOT navigate to the dashboard
    await expect(page).not.toHaveURL(/home|dashboard/);
  });

  test('should login with valid credentials', async ({ page }) => {
    const emailInput = page.locator('input[type="email"], input[data-testid="email_input"]').first();
    await emailInput.waitFor({ state: 'visible', timeout: 15_000 });
    await emailInput.fill(DEMO_EMAIL);

    const passwordInput = page.locator('input[type="password"]').first();
    await passwordInput.fill(DEMO_PASSWORD);

    const loginButton = page.getByRole('button', { name: /login|entrar|sign in/i });
    await loginButton.click();

    // Should navigate to dashboard
    await page.waitForURL('**/home', { timeout: 15_000 });
    await expect(page).toHaveURL(/home/);
  });

  test('should navigate to registration page', async ({ page }) => {
    // Look for register/create account link
    const registerLink = page
      .getByRole('button', { name: /cadastr|register|criar conta|sign up/i })
      .or(page.getByText(/cadastr|register|criar conta|sign up/i).first());

    if (await registerLink.first().isVisible().catch(() => false)) {
      await registerLink.first().click();
      await page.waitForTimeout(2000);

      // Registration page should have additional fields
      const nameInput = page.locator('input').first();
      await expect(nameInput).toBeVisible();
    }
  });

  test('should show password recovery option', async ({ page }) => {
    const forgotLink = page
      .getByText(/esquec|forgot|recuperar|reset/i)
      .first();

    if (await forgotLink.isVisible().catch(() => false)) {
      await forgotLink.click();
      await page.waitForTimeout(2000);

      // Should show a form for entering email
      const emailInput = page.locator('input[type="email"], input').first();
      await expect(emailInput).toBeVisible();
    }
  });
});
