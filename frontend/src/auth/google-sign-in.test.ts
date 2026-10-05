import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createGoogleSignIn, firebaseConfigFromEnv, SignInCancelledError } from './google-sign-in';

const sdk = vi.hoisted(() => ({
  initializeApp: vi.fn(() => ({ name: 'app' })),
  getApps: vi.fn((): unknown[] => []),
  getAuth: vi.fn(() => ({ name: 'auth' })),
  setPersistence: vi.fn(() => Promise.resolve()),
  signInWithPopup: vi.fn(),
  signOut: vi.fn(() => Promise.resolve()),
  setCustomParameters: vi.fn(),
}));

vi.mock('@firebase/app', () => ({ initializeApp: sdk.initializeApp, getApps: sdk.getApps }));
vi.mock('@firebase/auth', () => ({
  getAuth: sdk.getAuth,
  setPersistence: sdk.setPersistence,
  signInWithPopup: sdk.signInWithPopup,
  signOut: sdk.signOut,
  inMemoryPersistence: { type: 'NONE' },
  GoogleAuthProvider: class {
    setCustomParameters = sdk.setCustomParameters;
  },
}));

const config = {
  apiKey: 'key',
  authDomain: 'demo.firebaseapp.com',
  projectId: 'demo',
  appId: '1:2:web:3',
};

beforeEach(() => vi.clearAllMocks());

describe('firebaseConfigFromEnv', () => {
  it('requires every value', () => {
    const env = {
      VITE_FIREBASE_API_KEY: ' key ',
      VITE_FIREBASE_AUTH_DOMAIN: 'demo.firebaseapp.com',
      VITE_FIREBASE_PROJECT_ID: 'demo',
      VITE_FIREBASE_APP_ID: '1:2:web:3',
    } as unknown as ImportMetaEnv;
    expect(firebaseConfigFromEnv(env)).toEqual(config);
    expect(
      firebaseConfigFromEnv({ ...env, VITE_FIREBASE_APP_ID: ' ' } as unknown as ImportMetaEnv),
    ).toBeNull();
    expect(firebaseConfigFromEnv({} as ImportMetaEnv)).toBeNull();
  });
});

describe('createGoogleSignIn', () => {
  it('returns the ID token and leaves no Firebase session behind', async () => {
    sdk.signInWithPopup.mockResolvedValue({
      user: { getIdToken: () => Promise.resolve('id-token') },
    });

    await expect(createGoogleSignIn(config)()).resolves.toBe('id-token');

    expect(sdk.initializeApp).toHaveBeenCalledWith(config);
    expect(sdk.setPersistence).toHaveBeenCalledWith({ name: 'auth' }, { type: 'NONE' });
    expect(sdk.setCustomParameters).toHaveBeenCalledWith({ prompt: 'select_account' });
    expect(sdk.signOut).toHaveBeenCalledOnce();
  });

  it('reuses an initialised app', async () => {
    sdk.getApps.mockReturnValueOnce([{ name: 'existing' }]);
    sdk.signInWithPopup.mockResolvedValue({ user: { getIdToken: () => Promise.resolve('t') } });

    await createGoogleSignIn(config)();

    expect(sdk.initializeApp).not.toHaveBeenCalled();
    expect(sdk.getAuth).toHaveBeenCalledWith({ name: 'existing' });
  });

  it('reports a closed popup as a cancellation', async () => {
    sdk.signInWithPopup.mockRejectedValue({ code: 'auth/popup-closed-by-user' });
    await expect(createGoogleSignIn(config)()).rejects.toBeInstanceOf(SignInCancelledError);
  });

  it('passes other Firebase errors through', async () => {
    const err = { code: 'auth/unauthorized-domain' };
    sdk.signInWithPopup.mockRejectedValue(err);
    await expect(createGoogleSignIn(config)()).rejects.toBe(err);
    expect(sdk.signOut).not.toHaveBeenCalled();
  });
});
