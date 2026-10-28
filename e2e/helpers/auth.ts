import { Page } from '@playwright/test';

/**
 * Injects JWT token into the browser's secure storage.
 * Flutter web uses a localStorage-based shim for FlutterSecureStorage.
 */
export async function injectAuth(page: Page, token: string, user: { name: string; email: string }) {
  const appUrl = process.env.APP_URL || 'http://localhost:3000';

  // Navigate to app first to set localStorage on the correct origin
  await page.goto(appUrl);

  // Wait for Flutter engine to initialize
  await page.waitForTimeout(3000);

  // FlutterSecureStorage on web uses localStorage with key prefix
  // The key pattern is: flutter_secure_storage_{key}
  await page.evaluate(
    ({ token, user }) => {
      const userData = JSON.stringify({
        token: token,
        name: user.name,
        email: user.email,
        is_new: false,
        is_trialing: false,
        plan_name: 'Starter',
      });
      localStorage.setItem('flutter_secure_storage_user', userData);
    },
    { token, user }
  );

  // Reload to pick up the stored auth
  await page.reload();

  // Wait for Flutter to re-render with auth state
  await page.waitForTimeout(3000);
}

/**
 * Login via the UI (for demo video recording).
 * Uses role-based and text-based selectors compatible with Flutter web HTML renderer.
 */
export async function loginViaUI(page: Page, email: string, password: string) {
  await page.goto('/');

  // Wait for Flutter to fully render
  await page.waitForTimeout(4000);

  // Fill email field - Flutter HTML renderer creates real <input> elements
  const emailInput = page.locator('input[type="email"], input[data-testid="email_input"]').first();
  await emailInput.waitFor({ state: 'visible', timeout: 15_000 });
  await emailInput.click();
  await emailInput.fill(email);

  // Small pause for visual smoothness in recordings
  await page.waitForTimeout(500);

  // Fill password field
  const passwordInput = page.locator('input[type="password"]').first();
  await passwordInput.click();
  await passwordInput.fill(password);

  await page.waitForTimeout(500);

  // Click login button - try multiple selectors for Flutter web
  const loginButton = page.getByRole('button', { name: /login|entrar|sign in/i });
  await loginButton.click();

  // Wait for dashboard to load
  await page.waitForURL('**/home', { timeout: 15_000 });

  // Wait for dashboard content to render
  await page.waitForTimeout(2000);
}

/**
 * Clear stored authentication data.
 */
export async function clearAuth(page: Page) {
  await page.evaluate(() => {
    localStorage.removeItem('flutter_secure_storage_user');
  });
}
