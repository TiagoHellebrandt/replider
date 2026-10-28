import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  fullyParallel: false, // Sequential for demo recording
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: 'html',
  timeout: 60_000, // Flutter web can be slow

  // Auto-serve the built Flutter app — run `flutter build web` in app/ first
  webServer: {
    command: 'npx serve ../app/build/web -l 3000 --no-clipboard',
    url: 'http://localhost:3000',
    reuseExistingServer: true,
    timeout: 15_000,
  },

  use: {
    baseURL: process.env.APP_URL || 'http://localhost:3000',
    trace: 'retain-on-failure',
    screenshot: 'on',
    video: 'on',
    actionTimeout: 15_000,
  },

  projects: [
    {
      name: 'desktop',
      use: {
        ...devices['Desktop Chrome'],
        viewport: { width: 1920, height: 1080 },
        video: { mode: 'on', size: { width: 1920, height: 1080 } },
      },
    },
    {
      name: 'mobile',
      use: {
        ...devices['iPhone 14'],
        video: { mode: 'on', size: { width: 390, height: 844 } },
      },
    },
  ],
});
