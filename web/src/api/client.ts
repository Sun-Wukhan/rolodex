import type {
  AccessToken,
  CreateUserInput,
  EnrichedProfile,
  ErrorBody,
  Me,
  SearchQuery,
  SearchResponse,
  User,
  UserDetails,
} from './types';

/** An error returned by the Rolodex API, carrying the standard error envelope. */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly fields: Record<string, string>;
  readonly requestId?: string;

  /** Builds an ApiError from an HTTP status and (possibly empty) error body. */
  constructor(status: number, body?: Partial<ErrorBody>) {
    super(body?.error?.message ?? `Request failed with status ${status}`);
    this.name = 'ApiError';
    this.status = status;
    this.code = body?.error?.code ?? 'unknown';
    this.fields = body?.error?.fields ?? {};
    this.requestId = body?.error?.request_id;
  }
}

export interface ApiClientOptions {
  baseUrl: string;
  onUnauthorized?: () => void;
  fetchImpl?: typeof fetch;
}

/** Typed client for every Rolodex endpoint used by the UI. */
export interface ApiClient {
  /** Sets (or clears) the bearer token used for authenticated calls. */
  setToken(token: string | null): void;
  login(username: string, password: string): Promise<AccessToken>;
  me(): Promise<Me>;
  searchUsers(q: SearchQuery): Promise<SearchResponse>;
  getUser(id: string): Promise<UserDetails>;
  createUser(input: CreateUserInput): Promise<User>;
  enrichUser(id: string, providers: string[]): Promise<EnrichedProfile>;
  listProviders(): Promise<string[]>;
}

/**
 * Creates an ApiClient. The token is held in this closure (memory only). A 401
 * on an authenticated call clears it and invokes onUnauthorized.
 */
export function createApiClient(opts: ApiClientOptions): ApiClient {
  const doFetch = opts.fetchImpl ?? fetch.bind(globalThis);
  const base = opts.baseUrl.replace(/\/$/, '');
  let token: string | null = null;

  async function request<T>(method: string, path: string, body?: unknown, auth = true): Promise<T> {
    const headers: Record<string, string> = { Accept: 'application/json' };
    if (body !== undefined) headers['Content-Type'] = 'application/json';
    if (auth && token) headers.Authorization = `Bearer ${token}`;

    let res: Response;
    try {
      res = await doFetch(`${base}${path}`, {
        method,
        headers,
        body: body === undefined ? undefined : JSON.stringify(body),
      });
    } catch {
      throw new ApiError(0, {
        error: { code: 'network_error', message: 'Unable to reach the Rolodex API' },
      });
    }

    const text = await res.text();
    const data: unknown = text ? safeParse(text) : undefined;
    if (!res.ok) {
      if (res.status === 401 && auth) {
        token = null;
        opts.onUnauthorized?.();
      }
      throw new ApiError(res.status, data as Partial<ErrorBody>);
    }
    return data as T;
  }

  return {
    setToken: (t) => {
      token = t;
    },
    login: (username, password) =>
      request<AccessToken>('POST', '/api/v1/auth/login', { username, password }, false),
    me: () => request<Me>('GET', '/api/v1/me'),
    searchUsers: (q) => request<SearchResponse>('GET', `/api/v1/users?${toQuery(q)}`),
    getUser: (id) => request<UserDetails>('GET', `/api/v1/users/${encodeURIComponent(id)}`),
    createUser: (input) => request<User>('POST', '/api/v1/users', input),
    enrichUser: (id, providers) =>
      request<EnrichedProfile>(
        'POST',
        `/api/v1/users/${encodeURIComponent(id)}/enrich?provider=${encodeURIComponent(
          providers.length ? providers.join(',') : 'all',
        )}`,
      ),
    listProviders: async () =>
      (await request<{ providers: string[] }>('GET', '/api/v1/providers')).providers,
  };
}

function toQuery(q: SearchQuery): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(q)) {
    if (value !== undefined && value !== '') params.set(key, String(value));
  }
  return params.toString();
}

function safeParse(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch {
    return undefined;
  }
}
