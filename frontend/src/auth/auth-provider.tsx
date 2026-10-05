import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { createApiClient } from '../api/client';
import type { AccessToken } from '../api/types';
import { AuthContext, type AuthContextValue, type Session } from './auth-context';
import type { GoogleSignIn } from './google-sign-in';

interface AuthProviderProps {
  baseUrl: string;
  /** Overrides the transport; the static demo passes an in-browser API. */
  fetchImpl?: typeof fetch;
  demo?: boolean;
  /** Enables "Sign in with Google"; omitted when Firebase is not configured. */
  googleSignIn?: GoogleSignIn;
  children: ReactNode;
}

/**
 * Holds the session in memory only. Tokens are never written to localStorage,
 * which limits exposure to XSS at the cost of signing in again after a reload.
 */
export function AuthProvider({
  baseUrl,
  fetchImpl,
  demo = false,
  googleSignIn,
  children,
}: AuthProviderProps) {
  const [session, setSession] = useState<Session | null>(null);
  const [api] = useState(() =>
    createApiClient({ baseUrl, fetchImpl, onUnauthorized: () => setSession(null) }),
  );

  const logout = useCallback(() => {
    api.setToken(null);
    setSession(null);
  }, [api]);

  const startSession = useCallback(
    async (tok: AccessToken) => {
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

  const login = useCallback(
    async (username: string, password: string) => {
      await startSession(await api.login(username, password));
    },
    [api, startSession],
  );

  const loginWithGoogle = useCallback(async () => {
    if (!googleSignIn) throw new Error('Google sign-in is not configured');
    const idToken = await googleSignIn();
    await startSession(await api.firebaseLogin(idToken));
  }, [api, googleSignIn, startSession]);

  useEffect(() => {
    if (!session) return;
    const ms = session.expiresAt.getTime() - Date.now();
    const timer = setTimeout(logout, Math.max(ms, 0));
    return () => clearTimeout(timer);
  }, [session, logout]);

  const googleEnabled = Boolean(googleSignIn);
  const value = useMemo<AuthContextValue>(
    () => ({ session, api, demo, googleEnabled, login, loginWithGoogle, logout }),
    [session, api, demo, googleEnabled, login, loginWithGoogle, logout],
  );
  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
