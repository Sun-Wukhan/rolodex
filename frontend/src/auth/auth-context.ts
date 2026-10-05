import { createContext, useContext } from 'react';
import type { ApiClient } from '../api/client';

export interface Session {
  token: string;
  username: string;
  expiresAt: Date;
}

export interface AuthContextValue {
  session: Session | null;
  api: ApiClient;
  /** True when the UI is backed by the in-browser demo API (static hosting). */
  demo: boolean;
  /** True when Google sign-in (Firebase) is configured. */
  googleEnabled: boolean;
  login(username: string, password: string): Promise<void>;
  /** Signs in through a Google popup. Rejects with SignInCancelledError if the user backs out. */
  loginWithGoogle(): Promise<void>;
  logout(): void;
}

export const AuthContext = createContext<AuthContextValue | null>(null);

/** Returns the current auth state and API client. Must be used under AuthProvider. */
export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within an AuthProvider');
  return ctx;
}
