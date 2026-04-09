import { defineConfig, devices } from '@playwright/test';

/**
 * Playwright configuration for Domain Check E2E tests.
 *
 * Tests require the Go server to be running on localhost:8080.
 * Start it with: ./domain-check serve --port 8080
 */
export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: false,  // Run sequentially to avoid rate limiting
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 2 : 0,
  workers: 1,  // Single worker to avoid overwhelming the server
  reporter: 'html',

  use: {
    baseURL: 'http://localhost:8080',
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
    {
      name: 'firefox',
      use: { ...devices['Desktop Firefox'] },
    },
    {
      name: 'webkit',
      use: { ...devices['Desktop Safari'] },
    },
    {
      name: 'Mobile Chrome',
      use: { ...devices['Pixel 5'] },
    },
    {
      name: 'Mobile Safari',
      use: { ...devices['iPhone 12'] },
    },
  ],

  // Run local server before starting tests (optional - requires Go build)
  // webServer: {
  //   command: './domain-check serve --port 8080',
  //   url: 'http://localhost:8080',
  //   reuseExistingServer: !process.env.CI,
  //   timeout: 120 * 1000,
  // },
});
