/** Wire types mirroring api/openapi.yaml. */

export interface Address {
  street_address: string;
  locality: string;
  region: string;
  postal_code: string;
  country: string;
}

export interface Profile {
  user_id: string;
  name: string;
  phone: string;
  address: Address;
}

export type CredentialMethod = 'password' | 'oauth' | 'passkey';

export interface Credential {
  id: string;
  user_id: string;
  method: CredentialMethod;
  username: string;
  created_at: string;
  last_used_at?: string;
}

export interface UserDetails {
  profile: Profile;
  credentials: Credential[];
}

export interface AccessToken {
  access_token: string;
  token_type: string;
  expires_at: string;
}

export interface Me {
  user_id: string;
  username: string;
  expires_at: string;
}

export interface SearchQuery {
  name?: string;
  phone?: string;
  username?: string;
  limit?: number;
  offset?: number;
}

export interface SearchResponse {
  data: Profile[];
  limit: number;
  offset: number;
}

export interface CreateUserInput {
  name: string;
  phone: string;
  username: string;
  password: string;
  address: Address;
}

export interface User {
  id: string;
  created_at: string;
  updated_at: string;
}

export interface FieldSource {
  value: string;
  source: string;
  verified_by: string[];
}

export type ProviderStatus = 'ok' | 'not_found' | 'unavailable' | 'error';

export interface ProviderResult {
  provider: string;
  status: ProviderStatus;
  error?: string;
  identity?: { provider: string; name: string; phone: string; address: Address };
}

export interface EnrichedProfile {
  user_id: string;
  fields: Record<string, FieldSource>;
  providers: ProviderResult[];
}

export interface ErrorBody {
  error: { code: string; message: string; fields?: Record<string, string>; request_id?: string };
}
