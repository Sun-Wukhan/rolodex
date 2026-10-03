import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { createApiClient } from '../api/client';
import { AuthContext, type AuthContextValue, type Session } from './auth-context';

interface AuthProviderProps {
  baseUrl: string;
  children: ReactNode;
}

/**
 * Holds the session in memory only. Tokens are never written to localStorage,
 * which limits exposure to XSS at the cost of signing in again after a reload.
 */
export function AuthProvider({ baseUrl, children }: AuthProviderProps) {
  const [session, setSession] = useState<Session | null>(null);
  const [api] = useState(() =>
    createApiClient({ baseUrl, onUnauthorized: () => setSession(null) }),
  );

  const logout = useCallback(() => {
    api.setToken(null);
    setSession(null);
  }, [api]);

  const login = useCallback(
    async (username: string, password: string) => {
      const tok = await api.login(username, password);
      api.setToken(tok.access_token);
      const me = await api.me();
      setSession({
        token: tok.access_token,
        username: me.username,
        expiresAt: new Date(tok.expires_at),
      });
    },
    [api],
  );

  useEffect(() => {
    if (!session) return;
    const ms = session.expiresAt.getTime() - Date.now();
    const timer = setTimeout(logout, Math.max(ms, 0));
    return () => clearTimeout(timer);
  }, [session, logout]);

  const value = useMemo<AuthContextValue>(
    () => ({ session, api, login, logout }),
    [session, api, login, logout],
  );
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
