import type {
  Address,
  CreateUserInput,
  Credential,
  FieldSource,
  Profile,
  ProviderResult,
} from '../api/types';
import {
  DEMO_ABC_RECORDS,
  DEMO_SEED_USERS,
  DEMO_XYC_RECORDS,
  type DemoVendorRecord,
} from './demo-data';

/** Same bounds as internal/service/profile.go. */
const MIN_PASSWORD_LEN = 12;
const MAX_PASSWORD_LEN = 128;
const TOKEN_TTL_MS = 15 * 60 * 1000;
const USERNAME_PATTERN = /^[a-z0-9._@-]{3,64}$/;
const COUNTRY_PATTERN = /^[A-Z]{2}$/;

const MESSAGES = {
  name: 'is required (max 200 characters)',
  phone: 'phone must have 10-15 digits',
  username: 'must be 3-64 characters: a-z, 0-9, . _ @ -',
  secretLength: `must be ${MIN_PASSWORD_LEN}-${MAX_PASSWORD_LEN} characters`,
  country: 'must be an ISO 3166-1 alpha-2 code',
} as const;

const VENDORS: Record<string, readonly DemoVendorRecord[]> = {
  abc: DEMO_ABC_RECORDS,
  xyc: DEMO_XYC_RECORDS,
};

interface StoredUser {
  profile: Profile;
  credential: Credential;
  createdAt: string;
}

interface Session {
  userId: string;
  username: string;
  expiresAt: Date;
}

/** Options for the demo backend. */
export interface DemoApiOptions {
  /** Simulated network latency, so loading states are visible. */
  latencyMs?: number;
  now?: () => Date;
}

class HttpError extends Error {
  readonly status: number;
  readonly code: string;
  readonly fields?: Record<string, string>;

  /** Creates an error that is rendered as the API's standard error envelope. */
  constructor(status: number, code: string, message: string, fields?: Record<string, string>) {
    super(message);
    this.status = status;
    this.code = code;
    this.fields = fields;
  }
}

/**
 * NormalizePhone ported from internal/domain/phone.go: ten digits are assumed
 * North American (+1); 11-15 digits get a leading +.
 */
export function normalizePhone(raw: string): string | null {
  let digits = '';
  for (const [i, ch] of [...raw.trim()].entries()) {
    if (ch >= '0' && ch <= '9') digits += ch;
    else if (ch === '+' && i === 0) continue;
    else if (' -().'.includes(ch)) continue;
    else return null;
  }
  if (digits.length === 10) return `+1${digits}`;
  if (digits.length >= 11 && digits.length <= 15) return `+${digits}`;
  return null;
}

const equalFold = (a: string, b: string) =>
  a.toLowerCase().replace(/\s+/g, '') === b.toLowerCase().replace(/\s+/g, '');

/**
 * Creates a fetch implementation that serves the Rolodex API from memory.
 * Used for the static GitHub Pages deployment, where no backend exists. It
 * mirrors the real service's validation, search and enrichment merge rules.
 * Any seeded username signs in with any password of 12+ characters; no
 * password is stored or embedded.
 */
export function createDemoFetch(opts: DemoApiOptions = {}): typeof fetch {
  const now = opts.now ?? (() => new Date());
  const latency = opts.latencyMs ?? 0;
  const users = new Map<string, StoredUser>();
  const sessions = new Map<string, Session>();

  for (const seed of DEMO_SEED_USERS) {
    insertUser(seed.username, seed.name, seed.phone, seed.address);
  }

  function insertUser(username: string, name: string, phone: string, address: Address): StoredUser {
    const id = crypto.randomUUID();
    const createdAt = now().toISOString();
    const user: StoredUser = {
      profile: { user_id: id, name, phone, address },
      credential: {
        id: crypto.randomUUID(),
        user_id: id,
        method: 'password',
        username,
        created_at: createdAt,
      },
      createdAt,
    };
    users.set(id, user);
    return user;
  }

  function findByUsername(username: string): StoredUser | undefined {
    return [...users.values()].find((u) => u.credential.username === username);
  }

  function authenticate(headers: Headers): Session {
    const token = headers.get('Authorization')?.replace(/^Bearer\s+/i, '') ?? '';
    const session = sessions.get(token);
    if (!session || session.expiresAt <= now()) {
      sessions.delete(token);
      throw new HttpError(401, 'unauthorized', 'authentication required');
    }
    return session;
  }

  function login(body: unknown) {
    const { username = '', password = '' } = (body ?? {}) as Record<string, string>;
    const user = findByUsername(String(username).trim().toLowerCase());
    if (!user || String(password).length < MIN_PASSWORD_LEN) {
      throw new HttpError(401, 'unauthorized', 'invalid username or password');
    }
    const token = `demo.${crypto.randomUUID()}`;
    const expiresAt = new Date(now().getTime() + TOKEN_TTL_MS);
    sessions.set(token, {
      userId: user.profile.user_id,
      username: user.credential.username,
      expiresAt,
    });
    user.credential.last_used_at = now().toISOString();
    return { access_token: token, token_type: 'Bearer', expires_at: expiresAt.toISOString() };
  }

  function search(params: URLSearchParams) {
    const name = params.get('name')?.trim().toLowerCase() ?? '';
    const rawPhone = params.get('phone')?.trim() ?? '';
    const username = params.get('username')?.trim().toLowerCase() ?? '';
    const limit = Math.min(Math.max(Number(params.get('limit')) || 20, 1), 100);
    const offset = Math.max(Number(params.get('offset')) || 0, 0);

    if (!name && !rawPhone && !username) {
      throw new HttpError(400, 'invalid_input', 'invalid input', {
        query: 'at least one of name, phone or username is required',
      });
    }
    const phone = rawPhone ? normalizePhone(rawPhone) : '';
    if (phone === null) {
      throw new HttpError(400, 'invalid_input', 'invalid input', {
        phone: MESSAGES.phone,
      });
    }

    const matches = [...users.values()]
      .filter(
        (u) =>
          (!name || u.profile.name.toLowerCase().includes(name)) &&
          (!phone || u.profile.phone === phone) &&
          (!username || u.credential.username.includes(username)),
      )
      .map((u) => u.profile)
      .sort((a, b) => a.name.localeCompare(b.name) || a.user_id.localeCompare(b.user_id));
    return { data: matches.slice(offset, offset + limit), limit, offset };
  }

  function getUser(id: string): StoredUser {
    const user = users.get(id);
    if (!user) throw new HttpError(404, 'not_found', 'resource not found');
    return user;
  }

  function createUser(body: unknown) {
    const input = (body ?? {}) as Partial<CreateUserInput>;
    const address: Address = {
      street_address: input.address?.street_address?.trim() ?? '',
      locality: input.address?.locality?.trim() ?? '',
      region: input.address?.region?.trim() ?? '',
      postal_code: input.address?.postal_code?.trim() ?? '',
      country: input.address?.country?.trim().toUpperCase() ?? '',
    };
    const name = input.name?.trim() ?? '';
    const username = input.username?.trim().toLowerCase() ?? '';
    const password = input.password ?? '';
    const phone = normalizePhone(input.phone ?? '');

    const fields: Record<string, string> = {};
    if (!name || name.length > 200) fields.name = MESSAGES.name;
    if (!phone) fields.phone = MESSAGES.phone;
    if (!USERNAME_PATTERN.test(username)) fields.username = MESSAGES.username;
    if (password.length < MIN_PASSWORD_LEN || password.length > MAX_PASSWORD_LEN) {
      fields.password = MESSAGES.secretLength;
    }
    if (address.country && !COUNTRY_PATTERN.test(address.country)) {
      fields.country = MESSAGES.country;
    }
    if (Object.keys(fields).length > 0 || !phone) {
      throw new HttpError(400, 'invalid_input', 'invalid input', fields);
    }
    if (findByUsername(username)) {
      throw new HttpError(409, 'conflict', 'resource already exists');
    }
    const user = insertUser(username, name, phone, address);
    return { id: user.profile.user_id, created_at: user.createdAt, updated_at: user.createdAt };
  }

  function enrich(id: string, params: URLSearchParams) {
    const requested = (params.get('provider') ?? 'all')
      .split(',')
      .map((p) => p.trim().toLowerCase())
      .filter(Boolean);
    const names =
      requested.length === 0 || (requested.length === 1 && requested[0] === 'all')
        ? Object.keys(VENDORS).sort()
        : [...new Set(requested)];
    const unknown = names.find((n) => !(n in VENDORS));
    if (unknown) {
      throw new HttpError(400, 'invalid_input', 'invalid input', {
        provider: `unknown provider "${unknown}"`,
      });
    }

    const { profile } = getUser(id);
    const results: ProviderResult[] = names.map((provider) => {
      const rec = VENDORS[provider]?.find((r) => r.phone === profile.phone);
      return rec
        ? { provider, status: 'ok', identity: { provider, ...rec } }
        : { provider, status: 'not_found' };
    });
    return { user_id: id, fields: merge(profile, results), providers: results };
  }

  async function handle(method: string, url: URL, headers: Headers, body: unknown) {
    const path = url.pathname.replace(/\/+$/, '');
    if (method === 'GET' && (path === '/healthz' || path === '/readyz')) return { status: 'ok' };
    if (method === 'POST' && path === '/api/v1/auth/login') return login(body);

    const session = authenticate(headers);
    if (method === 'GET' && path === '/api/v1/me') {
      return {
        user_id: session.userId,
        username: session.username,
        expires_at: session.expiresAt.toISOString(),
      };
    }
    if (method === 'GET' && path === '/api/v1/providers') {
      return { providers: Object.keys(VENDORS).sort() };
    }
    if (method === 'GET' && path === '/api/v1/users') return search(url.searchParams);
    if (method === 'POST' && path === '/api/v1/users') return [201, createUser(body)] as const;

    const enrichId = path.match(/^\/api\/v1\/users\/([^/]+)\/enrich$/)?.[1];
    if (method === 'POST' && enrichId) {
      return enrich(decodeURIComponent(enrichId), url.searchParams);
    }
    const userId = path.match(/^\/api\/v1\/users\/([^/]+)$/)?.[1];
    if (method === 'GET' && userId) {
      const user = getUser(decodeURIComponent(userId));
      return { profile: user.profile, credentials: [user.credential] };
    }
    throw new HttpError(404, 'not_found', 'route not found');
  }

  return async (input, init) => {
    if (latency > 0) await new Promise((r) => setTimeout(r, latency));
    const url = new URL(
      typeof input === 'string' || input instanceof URL ? input : input.url,
      'http://demo.local',
    );
    const method = (init?.method ?? 'GET').toUpperCase();
    const headers = new Headers(init?.headers);
    const requestId = `demo-${crypto.randomUUID().slice(0, 8)}`;
    const json = (status: number, payload: unknown) =>
      new Response(JSON.stringify(payload), {
        status,
        headers: { 'Content-Type': 'application/json', 'X-Request-Id': requestId },
      });

    try {
      const body = typeof init?.body === 'string' ? (JSON.parse(init.body) as unknown) : undefined;
      const out = await handle(method, url, headers, body);
      return Array.isArray(out) ? json(out[0], out[1]) : json(200, out);
    } catch (err) {
      if (err instanceof HttpError) {
        return json(err.status, {
          error: {
            code: err.code,
            message: err.message,
            fields: err.fields,
            request_id: requestId,
          },
        });
      }
      return json(400, {
        error: { code: 'invalid_request', message: 'malformed request', request_id: requestId },
      });
    }
  };
}

/** Port of the merge rules in internal/service/identity.go. */
function merge(p: Profile, results: ProviderResult[]): Record<string, FieldSource> {
  const fields: Record<string, FieldSource> = {};
  const local: Record<string, string> = { name: p.name, phone: p.phone, ...p.address };
  for (const [key, value] of Object.entries(local)) {
    fields[key] = { value, source: value ? 'local' : '', verified_by: [] };
  }
  for (const r of results) {
    if (!r.identity) continue;
    const vendor: Record<string, string> = {
      name: r.identity.name,
      phone: r.identity.phone,
      ...r.identity.address,
    };
    for (const [key, value] of Object.entries(vendor)) {
      if (!value) continue;
      const f = fields[key];
      if (!f?.value) fields[key] = { value, source: r.provider, verified_by: [] };
      else if (equalFold(f.value, value)) f.verified_by.push(r.provider);
    }
  }
  return fields;
}
