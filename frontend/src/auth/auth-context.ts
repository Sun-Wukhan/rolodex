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
  login(username: string, password: string): Promise<void>;
  logout(): void;
}

export const AuthContext = createContext<AuthContextValue | null>(null);

/** Returns the current auth state and API client. Must be used under AuthProvider. */
export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used within an AuthProvider');
  return ctx;
}
