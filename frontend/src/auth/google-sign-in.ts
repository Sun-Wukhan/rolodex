/** Public Firebase web-app settings (identifiers, not secrets). */
export interface FirebaseWebConfig {
  apiKey: string;
  authDomain: string;
  projectId: string;
  appId: string;
}

/** Opens Google sign-in and resolves with a Firebase ID token. */
export type GoogleSignIn = () => Promise<string>;

/** Thrown when the user closes or cancels the Google popup. */
export class SignInCancelledError extends Error {
  /** Creates the error with a fixed message. */
  constructor() {
    super('Google sign-in was cancelled');
    this.name = 'SignInCancelledError';
  }
}

const cancelledCodes = new Set(['auth/popup-closed-by-user', 'auth/cancelled-popup-request']);

/**
 * Reads the Firebase web config from VITE_FIREBASE_* variables. Returns null
 * (Google sign-in hidden) unless every value is present.
 */
export function firebaseConfigFromEnv(env: ImportMetaEnv): FirebaseWebConfig | null {
  const config = {
    apiKey: env.VITE_FIREBASE_API_KEY?.trim() ?? '',
    authDomain: env.VITE_FIREBASE_AUTH_DOMAIN?.trim() ?? '',
    projectId: env.VITE_FIREBASE_PROJECT_ID?.trim() ?? '',
    appId: env.VITE_FIREBASE_APP_ID?.trim() ?? '',
  };
  return Object.values(config).every(Boolean) ? config : null;
}

/**
 * Creates a GoogleSignIn backed by a Firebase popup. The SDK is loaded on
 * first use so it stays out of the main bundle. Firebase keeps its session in
 * memory and is signed out as soon as the ID token is read: the API exchanges
 * that token for its own, and nothing is left in browser storage.
 */
export function createGoogleSignIn(config: FirebaseWebConfig): GoogleSignIn {
  return async () => {
    const [{ getApps, initializeApp }, firebaseAuth] = await Promise.all([
      import('@firebase/app'),
      import('@firebase/auth'),
    ]);
    const {
      GoogleAuthProvider,
      getAuth,
      inMemoryPersistence,
      setPersistence,
      signInWithPopup,
      signOut,
    } = firebaseAuth;

    const app = getApps()[0] ?? initializeApp(config);
    const auth = getAuth(app);
    await setPersistence(auth, inMemoryPersistence);
    const provider = new GoogleAuthProvider();
    provider.setCustomParameters({ prompt: 'select_account' });

    let result;
    try {
      result = await signInWithPopup(auth, provider);
    } catch (err) {
      const code = (err as { code?: unknown } | null)?.code;
      if (typeof code === 'string' && cancelledCodes.has(code)) throw new SignInCancelledError();
      throw err;
    }
    try {
      return await result.user.getIdToken();
    } finally {
      await signOut(auth);
    }
  };
}
