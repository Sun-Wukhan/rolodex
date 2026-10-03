import { describe, expect, it, vi } from 'vitest';
import { ApiError, createApiClient } from './client';
import { describeError } from './describe-error';

function jsonResponse(status: number, body: unknown): Response {
  return new Response(body === undefined ? '' : JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
}

describe('createApiClient', () => {
  it('sends bearer token and builds search query without empty params', async () => {
    const fetchImpl = vi
      .fn()
      .mockResolvedValue(jsonResponse(200, { data: [], limit: 20, offset: 0 }));
    const api = createApiClient({ baseUrl: 'http://api/', fetchImpl });
    api.setToken('tok');

    await api.searchUsers({ name: 'Grace Hopper', phone: '', limit: 10 });

    const [url, init] = fetchImpl.mock.calls[0]!;
    expect(url).toBe('http://api/api/v1/users?name=Grace+Hopper&limit=10');
    expect(init.headers.Authorization).toBe('Bearer tok');
    expect(init.method).toBe('GET');
  });

  it('does not send a token on login', async () => {
    const fetchImpl = vi
      .fn()
      .mockResolvedValue(
        jsonResponse(200, { access_token: 'a', token_type: 'Bearer', expires_at: '' }),
      );
    const api = createApiClient({ baseUrl: 'http://api', fetchImpl });
    api.setToken('old');

    await api.login('ada', 'pw');

    const [, init] = fetchImpl.mock.calls[0]!;
    expect(init.headers.Authorization).toBeUndefined();
    expect(JSON.parse(init.body)).toEqual({ username: 'ada', password: 'pw' });
  });

  it('parses the error envelope, clears the token and calls onUnauthorized on 401', async () => {
    const onUnauthorized = vi.fn();
    const fetchImpl = vi.fn().mockResolvedValue(
      jsonResponse(401, {
        error: { code: 'unauthorized', message: 'invalid or expired token', request_id: 'r-1' },
      }),
    );
    const api = createApiClient({ baseUrl: 'http://api', onUnauthorized, fetchImpl });
    api.setToken('t');

    const err = await api.me().catch((e: unknown) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err).toMatchObject({ status: 401, code: 'unauthorized', requestId: 'r-1' });
    expect(onUnauthorized).toHaveBeenCalledOnce();

    await api.me().catch(() => undefined);
    expect(fetchImpl.mock.calls[1]![1].headers.Authorization).toBeUndefined();
  });

  it('does not treat a failed login as a session expiry', async () => {
    const onUnauthorized = vi.fn();
    const fetchImpl = vi
      .fn()
      .mockResolvedValue(
        jsonResponse(401, { error: { code: 'unauthorized', message: 'invalid credentials' } }),
      );
    const api = createApiClient({ baseUrl: 'http://api', onUnauthorized, fetchImpl });

    await expect(api.login('a', 'b')).rejects.toThrow('invalid credentials');
    expect(onUnauthorized).not.toHaveBeenCalled();
  });

  it('maps network failures and non-JSON errors', async () => {
    const down = createApiClient({
      baseUrl: 'http://api',
      fetchImpl: vi.fn().mockRejectedValue(new TypeError('Failed to fetch')),
    });
    await expect(down.listProviders()).rejects.toMatchObject({ status: 0, code: 'network_error' });

    const html = createApiClient({
      baseUrl: 'http://api',
      fetchImpl: vi.fn().mockResolvedValue(new Response('<html>', { status: 502 })),
    });
    await expect(html.getUser('x')).rejects.toMatchObject({ status: 502, code: 'unknown' });
  });

  it('encodes enrich providers and defaults to all', async () => {
    const fetchImpl = vi
      .fn()
      .mockImplementation(() => Promise.resolve(jsonResponse(200, { providers: [] })));
    const api = createApiClient({ baseUrl: 'http://api', fetchImpl });

    await api.enrichUser('u1', ['abc', 'xyc']);
    await api.enrichUser('u1', []);
    await api.createUser({
      name: 'n',
      phone: 'p',
      username: 'u',
      password: 'pw',
      address: { street_address: '', locality: '', region: '', postal_code: '', country: '' },
    });

    expect(fetchImpl.mock.calls[0]![0]).toBe(
      'http://api/api/v1/users/u1/enrich?provider=abc%2Cxyc',
    );
    expect(fetchImpl.mock.calls[1]![0]).toBe('http://api/api/v1/users/u1/enrich?provider=all');
    expect(fetchImpl.mock.calls[2]![1].method).toBe('POST');
  });
});

describe('describeError', () => {
  it('produces friendly messages', () => {
    expect(describeError(new ApiError(0)).message).toMatch(/Unable to reach/);
    expect(describeError(new ApiError(429)).message).toMatch(/Too many attempts/);
    const v = describeError(
      new ApiError(400, {
        error: { code: 'invalid_input', message: 'bad', fields: { phone: 'invalid' } },
      }),
    );
    expect(v).toEqual({ message: 'bad', requestId: undefined, fields: { phone: 'invalid' } });
    expect(describeError(new Error('boom')).message).toMatch(/Something went wrong/);
  });
});
