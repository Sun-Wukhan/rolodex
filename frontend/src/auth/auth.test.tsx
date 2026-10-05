import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { ThemeProvider } from 'styled-components';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { App } from '../app';
import { theme } from '../styles/theme';
import { AuthProvider } from './auth-provider';
import type { GoogleSignIn } from './google-sign-in';

function reply(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

function renderApp(route = '/', googleSignIn?: GoogleSignIn) {
  return render(
    <ThemeProvider theme={theme}>
      <MemoryRouter initialEntries={[route]}>
        <AuthProvider baseUrl="http://api" googleSignIn={googleSignIn}>
          <App />
        </AuthProvider>
      </MemoryRouter>
    </ThemeProvider>,
  );
}

afterEach(() => vi.unstubAllGlobals());

describe('AuthProvider + routing', () => {
  it('redirects to login, signs in, then signs out', async () => {
    const fetchMock = vi.fn((url: string) => {
      if (url.endsWith('/auth/login'))
        return Promise.resolve(
          reply(200, {
            access_token: 'jwt',
            token_type: 'Bearer',
            expires_at: new Date(Date.now() + 60_000).toISOString(),
          }),
        );
      if (url.endsWith('/me'))
        return Promise.resolve(reply(200, { user_id: 'u', username: 'admin', expires_at: '' }));
      return Promise.resolve(reply(404, {}));
    });
    vi.stubGlobal('fetch', fetchMock);

    renderApp('/users/new');
    expect(screen.getByRole('heading', { name: 'Sign in to Rolodex' })).toBeInTheDocument();

    await userEvent.type(screen.getByLabelText('Username'), 'admin');
    await userEvent.type(screen.getByLabelText('Password'), 'secret-password');
    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }));

    expect(await screen.findByRole('heading', { name: 'Add user' })).toBeInTheDocument();
    expect(screen.getByText('Signed in as admin')).toBeInTheDocument();
    const meCall = fetchMock.mock.calls.find(([u]) => u.endsWith('/me')) as unknown as [
      string,
      RequestInit,
    ];
    expect((meCall[1].headers as Record<string, string>).Authorization).toBe('Bearer jwt');

    await userEvent.click(screen.getByRole('button', { name: 'Sign out' }));
    expect(await screen.findByRole('heading', { name: 'Sign in to Rolodex' })).toBeInTheDocument();
  });

  it('exchanges a Google ID token for a Rolodex session', async () => {
    const fetchMock = vi.fn((url: string) => {
      if (url.endsWith('/auth/firebase'))
        return Promise.resolve(
          reply(200, {
            access_token: 'jwt-google',
            token_type: 'Bearer',
            expires_at: new Date(Date.now() + 60_000).toISOString(),
          }),
        );
      if (url.endsWith('/me'))
        return Promise.resolve(
          reply(200, { user_id: 'u', username: 'grace@navy.mil', expires_at: '' }),
        );
      return Promise.resolve(reply(404, {}));
    });
    vi.stubGlobal('fetch', fetchMock);
    const googleSignIn = vi.fn<GoogleSignIn>().mockResolvedValue('firebase-id-token');

    renderApp('/users/new', googleSignIn);
    await userEvent.click(screen.getByRole('button', { name: 'Sign in with Google' }));

    expect(await screen.findByRole('heading', { name: 'Add user' })).toBeInTheDocument();
    expect(screen.getByText('Signed in as grace@navy.mil')).toBeInTheDocument();
    const exchange = fetchMock.mock.calls.find(([u]) =>
      u.endsWith('/auth/firebase'),
    ) as unknown as [string, RequestInit];
    expect(JSON.parse(exchange[1].body as string)).toEqual({ id_token: 'firebase-id-token' });
    const meCall = fetchMock.mock.calls.find(([u]) => u.endsWith('/me')) as unknown as [
      string,
      RequestInit,
    ];
    expect((meCall[1].headers as Record<string, string>).Authorization).toBe('Bearer jwt-google');
  });

  it('logs out when the API rejects the token', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn((url: string) => {
        if (url.endsWith('/auth/login'))
          return Promise.resolve(
            reply(200, {
              access_token: 'jwt',
              token_type: 'Bearer',
              expires_at: new Date(Date.now() + 60_000).toISOString(),
            }),
          );
        if (url.endsWith('/me'))
          return Promise.resolve(reply(200, { user_id: 'u', username: 'admin', expires_at: '' }));
        return Promise.resolve(
          reply(401, { error: { code: 'unauthorized', message: 'invalid or expired token' } }),
        );
      }),
    );

    renderApp('/');
    await userEvent.type(screen.getByLabelText('Username'), 'admin');
    await userEvent.type(screen.getByLabelText('Password'), 'secret-password');
    await userEvent.click(screen.getByRole('button', { name: 'Sign in' }));
    await userEvent.type(await screen.findByLabelText('Search by name'), 'ada');
    await userEvent.click(screen.getByRole('button', { name: 'Search' }));

    expect(await screen.findByRole('heading', { name: 'Sign in to Rolodex' })).toBeInTheDocument();
  });
});
