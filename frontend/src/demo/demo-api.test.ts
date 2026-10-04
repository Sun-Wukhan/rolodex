import { describe, expect, it } from 'vitest';
import { ApiError, createApiClient, type ApiClient } from '../api/client';
import { createDemoFetch, normalizePhone } from './demo-api';

// Generated at runtime: the demo accepts any secret of 12+ characters.
const PASSWORD = 'x'.repeat(16);
const SHORT = 'x'.repeat(5);

async function signedIn(username = 'admin'): Promise<ApiClient> {
  const api = createApiClient({ baseUrl: '', fetchImpl: createDemoFetch() });
  const tok = await api.login(username, PASSWORD);
  api.setToken(tok.access_token);
  return api;
}

async function idOf(api: ApiClient, name: string): Promise<string> {
  const res = await api.searchUsers({ name });
  return res.data[0]?.user_id ?? '';
}

describe('normalizePhone', () => {
  it('mirrors the backend rules', () => {
    expect(normalizePhone('(416) 555-0101')).toBe('+14165550101');
    expect(normalizePhone('+44 20 7946 0103')).toBe('+442079460103');
    expect(normalizePhone('12345')).toBeNull();
    expect(normalizePhone('416-555-01x1')).toBeNull();
  });
});

describe('demo API', () => {
  it('rejects unknown users and short passwords', async () => {
    const api = createApiClient({ baseUrl: '', fetchImpl: createDemoFetch() });
    await expect(api.login('nobody', PASSWORD)).rejects.toMatchObject({ status: 401 });
    await expect(api.login('admin', SHORT)).rejects.toMatchObject({ status: 401 });
  });

  it('requires a token for protected routes', async () => {
    const api = createApiClient({ baseUrl: '', fetchImpl: createDemoFetch() });
    await expect(api.me()).rejects.toMatchObject({ status: 401, code: 'unauthorized' });
  });

  it('expires sessions after the token TTL', async () => {
    let clock = new Date('2026-01-01T00:00:00Z');
    const api = createApiClient({ baseUrl: '', fetchImpl: createDemoFetch({ now: () => clock }) });
    api.setToken((await api.login('admin', PASSWORD)).access_token);
    clock = new Date('2026-01-01T00:16:00Z');
    await expect(api.me()).rejects.toMatchObject({ status: 401 });
  });

  it('returns the signed-in user and providers', async () => {
    const api = await signedIn('ada');
    expect((await api.me()).username).toBe('ada');
    expect(await api.listProviders()).toEqual(['abc', 'xyc']);
  });

  it('searches by name, phone and username and requires a filter', async () => {
    const api = await signedIn();
    expect((await api.searchUsers({ name: 'ADA' })).data.map((p) => p.name)).toEqual([
      'Ada Lovelace',
    ]);
    expect((await api.searchUsers({ phone: '212 555 0102' })).data[0]?.name).toBe('Grace Hopper');
    expect((await api.searchUsers({ username: 'kath' })).data[0]?.name).toBe('Katherine Johnson');
    await expect(api.searchUsers({})).rejects.toMatchObject({ status: 400 });
    await expect(api.searchUsers({ phone: '1' })).rejects.toMatchObject({
      fields: { phone: expect.any(String) },
    });
  });

  it('paginates sorted by name', async () => {
    const api = await signedIn();
    const page = await api.searchUsers({ name: 'a', limit: 2, offset: 1 });
    expect(page.data.map((p) => p.name)).toEqual(['Alan Turing', 'Grace Hopper']);
  });

  it('creates users, validates input and rejects duplicates', async () => {
    const api = await signedIn();
    const input = {
      name: 'Margaret Hamilton',
      phone: '617-555-0199',
      username: 'Margaret',
      password: PASSWORD,
      address: {
        street_address: '',
        locality: 'Boston',
        region: 'MA',
        postal_code: '',
        country: 'us',
      },
    };
    const user = await api.createUser(input);
    const details = await api.getUser(user.id);
    expect(details.profile.phone).toBe('+16175550199');
    expect(details.profile.address.country).toBe('US');
    expect(details.credentials[0]?.username).toBe('margaret');

    await expect(api.createUser(input)).rejects.toMatchObject({ status: 409 });
    const bad = api.createUser({
      ...input,
      username: 'x',
      password: SHORT,
      phone: '1',
      name: '',
    });
    await expect(bad).rejects.toBeInstanceOf(ApiError);
    await expect(bad).rejects.toMatchObject({
      fields: {
        name: expect.any(String),
        phone: expect.any(String),
        username: expect.any(String),
        password: expect.any(String),
      },
    });
    await expect(
      api.createUser({
        ...input,
        username: 'other',
        address: { ...input.address, country: 'USA' },
      }),
    ).rejects.toMatchObject({ fields: { country: expect.any(String) } });
  });

  it('returns 404 for unknown users and routes', async () => {
    const api = await signedIn();
    await expect(api.getUser('missing')).rejects.toMatchObject({ status: 404 });
    const res = await createDemoFetch()('/nope', { method: 'GET' });
    expect(res.status).toBe(401);
  });

  it('merges vendor data with provenance like the backend', async () => {
    const api = await signedIn();

    const katherine = await api.enrichUser(await idOf(api, 'Katherine'), []);
    expect(katherine.fields.name?.verified_by).toEqual(['abc', 'xyc']);
    expect(katherine.fields.street_address).toMatchObject({
      source: 'local',
      verified_by: ['xyc'],
    });

    const grace = await api.enrichUser(await idOf(api, 'Grace'), ['abc']);
    expect(grace.fields.street_address).toMatchObject({ value: '350 Fifth Ave', source: 'abc' });

    const alan = await api.enrichUser(await idOf(api, 'Alan'), []);
    expect(alan.providers.map((p) => [p.provider, p.status])).toEqual([
      ['abc', 'not_found'],
      ['xyc', 'ok'],
    ]);
    expect(alan.fields.country).toMatchObject({ value: 'GB', source: 'xyc' });

    await expect(api.enrichUser(await idOf(api, 'Ada'), ['nope'])).rejects.toMatchObject({
      fields: { provider: expect.any(String) },
    });
  });

  it('serves health checks and reports malformed bodies', async () => {
    const demoFetch = createDemoFetch({ latencyMs: 1 });
    expect((await demoFetch('/healthz')).status).toBe(200);
    const res = await demoFetch(new URL('http://x/api/v1/auth/login'), {
      method: 'POST',
      body: '{not json',
    });
    expect(res.status).toBe(400);
    expect(res.headers.get('X-Request-Id')).toMatch(/^demo-/);
  });
});
