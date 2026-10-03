import { expect, randomId, signIn, test } from './fixtures';

test.describe('authentication', () => {
  test('redirects unauthenticated deep links to the login page', async ({ page }) => {
    await page.goto('users/new');
    await expect(page).toHaveURL(/\/login$/);
    await expect(page.getByRole('heading', { name: 'Sign in to Rolodex' })).toBeVisible();
  });

  test('rejects invalid credentials', async ({ page, credentials }) => {
    await page.goto('login');
    await page.getByLabel('Username').fill(credentials.username);
    await page.getByLabel('Password').fill(randomId());
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(page.getByRole('alert')).toBeVisible();
    await expect(page).toHaveURL(/\/login$/);
  });

  test('signs in and out', async ({ page, credentials }) => {
    await signIn(page, credentials);
    await page.getByRole('button', { name: 'Sign out' }).click();
    await expect(page).toHaveURL(/\/login$/);
  });
});

test.describe('directory', () => {
  test.beforeEach(async ({ page, credentials }) => {
    await signIn(page, credentials);
  });

  test('searches, opens a profile and verifies it with ABC', async ({ page }) => {
    await page.getByRole('searchbox', { name: 'Search by name' }).fill('katherine');
    await page.getByRole('button', { name: 'Search', exact: true }).click();

    const results = page.getByRole('region', { name: 'Search results' });
    await results.getByRole('link', { name: /Katherine Johnson/ }).click();
    await expect(page.getByRole('heading', { level: 1, name: 'Katherine Johnson' })).toBeVisible();

    await page.getByRole('button', { name: 'Check ABC' }).click();
    await expect(page.getByTestId('provider-abc')).toContainText('Matched');
  });

  test('creates a user and lands on the new profile', async ({ page }) => {
    const username = `e2e-${randomId()}`;
    await page.getByRole('link', { name: 'Add user' }).click();
    await page.getByLabel('Full name').fill('End To End');
    await page.getByLabel('Phone').fill('416-555-0142');
    await page.getByLabel('Country').fill('CA');
    await page.getByLabel('Username').fill(username);
    await page.getByLabel('Password').fill(randomId(12));
    await page.getByRole('button', { name: 'Create user' }).click();

    await expect(page.getByRole('heading', { level: 1, name: 'End To End' })).toBeVisible();
    await expect(page.getByText('+14165550142')).toBeVisible();
    await expect(page.getByText(username)).toBeVisible();
  });

  test('shows field-level validation errors from the API', async ({ page }) => {
    await page.getByRole('link', { name: 'Add user' }).click();
    await page.getByLabel('Full name').fill('Bad Phone');
    await page.getByLabel('Phone').fill('not-a-phone');
    await page.getByLabel('Username').fill(`e2e-${randomId()}`);
    await page.getByLabel('Password').fill(randomId(12));
    await page.getByRole('button', { name: 'Create user' }).click();

    await expect(page.getByLabel('Phone')).toHaveAttribute('aria-invalid', 'true');
  });
});

test('loads without console errors or CSP violations', async ({
  page,
  credentials,
  browserErrors,
}) => {
  await signIn(page, credentials);
  await page.getByRole('searchbox', { name: 'Search by name' }).fill('ada');
  await page.getByRole('button', { name: 'Search', exact: true }).click();
  await expect(page.getByRole('region', { name: 'Search results' })).toBeVisible();
  expect(browserErrors).toEqual([]);
});
