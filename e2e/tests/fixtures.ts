import { randomBytes } from 'node:crypto';
import { test as base, expect, type Page } from '@playwright/test';

/** Account used to sign in; the pipeline injects a seeded user's credentials. */
export interface Credentials {
  username: string;
  password: string;
}

/**
 * Reads sign-in credentials from E2E_USERNAME / E2E_PASSWORD. Demo mode accepts
 * any password of 12+ characters, so a random one is generated when unset.
 */
export function credentialsFromEnv(env: NodeJS.ProcessEnv = process.env): Credentials {
  return {
    username: env.E2E_USERNAME ?? 'admin',
    password: env.E2E_PASSWORD ?? randomBytes(12).toString('hex'),
  };
}

/** Returns a random lowercase hex string, handy for unique usernames. */
export function randomId(bytes = 4): string {
  return randomBytes(bytes).toString('hex');
}

/**
 * Signs in through the login form and waits for the authenticated shell. Starts
 * from the root because static hosts such as GitHub Pages answer deep links via
 * a 404 fallback page, which the browser logs as a console error.
 */
export async function signIn(page: Page, { username, password }: Credentials): Promise<void> {
  await page.goto('./');
  await expect(page).toHaveURL(/\/login$/);
  await page.getByLabel('Username').fill(username);
  await page.getByLabel('Password').fill(password);
  await page.getByRole('button', { name: 'Sign in' }).click();
  await expect(page.getByText(`Signed in as ${username}`)).toBeVisible();
}

interface Fixtures {
  credentials: Credentials;
  /** Console errors and uncaught exceptions captured while the test ran. */
  browserErrors: string[];
}

/** Playwright test with credentials and a browser error collector. */
export const test = base.extend<Fixtures>({
  credentials: async ({}, use) => {
    await use(credentialsFromEnv());
  },
  browserErrors: async ({ page }, use) => {
    const errors: string[] = [];
    page.on('pageerror', (err) => errors.push(err.message));
    page.on('console', (msg) => {
      if (msg.type() === 'error') errors.push(msg.text());
    });
    await use(errors);
  },
});

export { expect };
