import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '../api/client';
import type { UserDetails } from '../api/types';
import { mockApi, renderWithProviders } from '../test/render';
import { CreateUserPage } from './create-user-page';
import { LoginPage } from './login-page';
import { ProfilePage } from './profile-page';
import { clearSearchCache } from './search-cache';
import { SearchPage } from './search-page';

const grace: UserDetails = {
  profile: {
    user_id: 'u-grace',
    name: 'Grace Hopper',
    phone: '+12125550102',
    address: {
      street_address: '',
      locality: 'New York',
      region: 'NY',
      postal_code: '',
      country: 'US',
    },
  },
  credentials: [
    {
      id: 'c1',
      user_id: 'u-grace',
      method: 'password',
      username: 'grace',
      created_at: '2026-01-01T00:00:00Z',
    },
  ],
};

describe('LoginPage', () => {
  it('signs in and navigates away', async () => {
    const login = vi.fn().mockResolvedValue(undefined);
    renderWithProviders(<LoginPage />, {
      auth: { session: null, login },
      route: '/login',
      path: '/login',
    });

    await userEvent.type(screen.getByLabelText('Username'), ' admin ');
    await userEvent.type(screen.getByLabelText('Password'), 'secret-password');
    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }));

    expect(login).toHaveBeenCalledWith('admin', 'secret-password');
    expect(await screen.findByText('navigated')).toBeInTheDocument();
  });

  it('shows the API error and clears the password', async () => {
    const login = vi.fn().mockRejectedValue(
      new ApiError(401, {
        error: { code: 'unauthorized', message: 'invalid credentials', request_id: 'r9' },
      }),
    );
    renderWithProviders(<LoginPage />, {
      auth: { session: null, login },
      route: '/login',
      path: '/login',
    });

    await userEvent.type(screen.getByLabelText('Username'), 'admin');
    await userEvent.type(screen.getByLabelText('Password'), 'wrong');
    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('invalid credentials');
    expect(screen.getByLabelText('Password')).toHaveValue('');
  });

  it('redirects when already signed in', () => {
    renderWithProviders(<LoginPage />, { route: '/login', path: '/login' });
    expect(screen.getByText('navigated')).toBeInTheDocument();
  });
});

describe('SearchPage', () => {
  beforeEach(() => clearSearchCache());

  it('searches by the selected field and lists results', async () => {
    const api = mockApi({
      searchUsers: vi.fn().mockResolvedValue({ data: [grace.profile], limit: 50, offset: 0 }),
    });
    renderWithProviders(<SearchPage />, { auth: { api } });

    await userEvent.click(screen.getByRole('button', { name: 'Phone' }));
    await userEvent.type(screen.getByLabelText('Search by phone'), '212-555-0102');
    await userEvent.click(screen.getByRole('button', { name: 'Search' }));

    expect(api.searchUsers).toHaveBeenCalledWith({ phone: '212-555-0102', limit: 50 });
    const link = await screen.findByRole('link', { name: /Grace Hopper/ });
    expect(link).toHaveAttribute('href', '/users/u-grace');
    expect(screen.getByText('1 result')).toBeInTheDocument();
  });

  it('shows empty and validation states', async () => {
    const searchUsers = vi
      .fn()
      .mockResolvedValueOnce({ data: [], limit: 50, offset: 0 })
      .mockRejectedValueOnce(
        new ApiError(400, {
          error: {
            code: 'invalid_input',
            message: 'request validation failed',
            fields: { phone: 'invalid phone number' },
          },
        }),
      );
    renderWithProviders(<SearchPage />, { auth: { api: mockApi({ searchUsers }) } });

    await userEvent.type(screen.getByLabelText('Search by name'), 'zzz');
    await userEvent.click(screen.getByRole('button', { name: 'Search' }));
    expect(await screen.findByText('No profiles matched.')).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Search' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('invalid phone number');
  });
});

describe('ProfilePage', () => {
  it('loads the profile and enriches from a provider', async () => {
    const api = mockApi({
      getUser: vi.fn().mockResolvedValue(grace),
      listProviders: vi.fn().mockResolvedValue(['abc', 'xyc']),
      enrichUser: vi.fn().mockResolvedValue({
        user_id: 'u-grace',
        fields: { postal_code: { value: '10118', source: 'abc', verified_by: [] } },
        providers: [{ provider: 'abc', status: 'ok' }],
      }),
    });
    renderWithProviders(<ProfilePage />, {
      auth: { api },
      route: '/users/u-grace',
      path: '/users/:id',
    });

    expect(await screen.findByRole('heading', { name: 'Grace Hopper' })).toBeInTheDocument();
    expect(screen.getByText('New York, NY, US')).toBeInTheDocument();
    expect(screen.getByText('password')).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Check ABC' }));
    expect(api.enrichUser).toHaveBeenCalledWith('u-grace', ['abc']);
    expect(await screen.findByText('10118')).toBeInTheDocument();

    await userEvent.click(screen.getByRole('button', { name: 'Check all' }));
    expect(api.enrichUser).toHaveBeenLastCalledWith('u-grace', []);
  });

  it('shows load and enrichment errors', async () => {
    const notFound = mockApi({
      getUser: vi
        .fn()
        .mockRejectedValue(
          new ApiError(404, { error: { code: 'not_found', message: 'resource not found' } }),
        ),
    });
    const { unmount } = renderWithProviders(<ProfilePage />, {
      auth: { api: notFound },
      route: '/users/x',
      path: '/users/:id',
    });
    expect(await screen.findByRole('alert')).toHaveTextContent('resource not found');
    unmount();

    const failing = mockApi({
      getUser: vi.fn().mockResolvedValue(grace),
      listProviders: vi.fn().mockResolvedValue(['abc']),
      enrichUser: vi.fn().mockRejectedValue(new ApiError(0)),
    });
    renderWithProviders(<ProfilePage />, {
      auth: { api: failing },
      route: '/users/u-grace',
      path: '/users/:id',
    });
    await userEvent.click(await screen.findByRole('button', { name: 'Check ABC' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Unable to reach');
  });
});

describe('CreateUserPage', () => {
  it('maps server validation errors to fields', async () => {
    const createUser = vi.fn().mockRejectedValue(
      new ApiError(400, {
        error: {
          code: 'invalid_input',
          message: 'request validation failed',
          fields: {
            password: 'must be 12-128 characters',
            'address.country': 'ISO 3166-1 alpha-2 code',
          },
        },
      }),
    );
    renderWithProviders(<CreateUserPage />, { auth: { api: mockApi({ createUser }) } });

    await userEvent.type(screen.getByLabelText('Full name'), 'Linus');
    await userEvent.type(screen.getByLabelText('Country'), 'Finland');
    await userEvent.click(screen.getByRole('button', { name: 'Create user' }));

    await waitFor(() =>
      expect(screen.getByLabelText('Password')).toHaveAccessibleDescription(
        'must be 12-128 characters',
      ),
    );
    expect(screen.getByLabelText('Country')).toHaveAccessibleDescription('ISO 3166-1 alpha-2 code');
    expect(createUser.mock.calls[0]![0]).toMatchObject({
      name: 'Linus',
      address: { country: 'Finland' },
    });
  });

  it('navigates to the new profile on success', async () => {
    const createUser = vi.fn().mockResolvedValue({ id: 'new-id', created_at: '', updated_at: '' });
    renderWithProviders(<CreateUserPage />, { auth: { api: mockApi({ createUser }) } });
    await userEvent.click(screen.getByRole('button', { name: 'Create user' }));
    expect(await screen.findByText('navigated')).toBeInTheDocument();
  });
});
