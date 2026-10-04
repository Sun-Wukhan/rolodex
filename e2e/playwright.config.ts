import { defineConfig, devices } from '@playwright/test';

const baseURL = process.env.BASE_URL ?? 'http://localhost:3000/';

export default defineConfig({
  testDir: './tests',
  timeout: 30_000,
  expect: { timeout: 10_000 },
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  // The API rate-limits logins per client IP; keep concurrent sign-ins low.
  workers: 2,
  reporter: process.env.CI
    ? [['github'], ['list'], ['html', { open: 'never' }], ['junit', { outputFile: 'results.xml' }]]
    : [['list'], ['html', { open: 'never' }]],
  use: {
    // Relative paths in tests resolve against this, so a sub-path such as
    // GitHub Pages' /rolodex/ works when the URL ends with a slash.
    baseURL: baseURL.endsWith('/') ? baseURL : `${baseURL}/`,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  projects: [{ name: 'chromium', use: { ...devices['Desktop Chrome'] } }],
});
