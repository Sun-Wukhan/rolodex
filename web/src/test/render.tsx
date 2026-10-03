import { render, type RenderResult } from '@testing-library/react';
import type { ReactElement } from 'react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { ThemeProvider } from 'styled-components';
import { vi } from 'vitest';
import type { ApiClient } from '../api/client';
import { AuthContext, type AuthContextValue } from '../auth/auth-context';
import { theme } from '../styles/theme';

/** Returns an ApiClient whose methods are all vi.fn() mocks. */
export function mockApi(overrides: Partial<ApiClient> = {}): ApiClient {
  return {
    setToken: vi.fn(),
    login: vi.fn(),
    me: vi.fn(),
    searchUsers: vi.fn(),
    getUser: vi.fn(),
    createUser: vi.fn(),
    enrichUser: vi.fn(),
    listProviders: vi.fn().mockResolvedValue([]),
    ...overrides,
  };
}

interface Options {
  auth?: Partial<AuthContextValue>;
  route?: string;
  path?: string;
}

/** Renders ui with theme, router and a controllable auth context. */
export function renderWithProviders(
  ui: ReactElement,
  { auth = {}, route = '/', path = '/' }: Options = {},
): RenderResult & { auth: AuthContextValue } {
  const value: AuthContextValue = {
    session: { token: 't', username: 'admin', expiresAt: new Date(Date.now() + 60_000) },
    api: mockApi(),
    login: vi.fn(),
    logout: vi.fn(),
    ...auth,
  };
  const result = render(
    <ThemeProvider theme={theme}>
      <AuthContext.Provider value={value}>
        <MemoryRouter initialEntries={[route]}>
          <Routes>
            <Route path={path} element={ui} />
            <Route path="*" element={<div>navigated</div>} />
          </Routes>
        </MemoryRouter>
      </AuthContext.Provider>
    </ThemeProvider>,
  );
  return { ...result, auth: value };
}
