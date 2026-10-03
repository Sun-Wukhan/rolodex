import { useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import styled from 'styled-components';
import { describeError, type DescribedError } from '../api/describe-error';
import type { EnrichedProfile, UserDetails } from '../api/types';
import { useAuth } from '../auth/auth-context';
import { Alert } from '../components/alert';
import { Badge } from '../components/badge';
import { Button } from '../components/button';
import { Card, CardHeader, CardTitle, Muted } from '../components/card';
import { EnrichmentResults } from '../components/enrichment-results';
import { Spinner } from '../components/spinner';

const Grid = styled.div`
  display: grid;
  grid-template-columns: 2fr 1fr;
  gap: ${({ theme }) => theme.space(6)};
  @media (max-width: 800px) {
    grid-template-columns: 1fr;
  }
`;

const Details = styled.dl`
  display: grid;
  grid-template-columns: 140px 1fr;
  gap: ${({ theme }) => `${theme.space(3)} ${theme.space(4)}`};
  margin: 0;
  dt {
    color: ${({ theme }) => theme.colors.textMuted};
    font-size: 13px;
  }
  dd {
    margin: 0;
  }
`;

const CredList = styled.ul`
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: ${({ theme }) => theme.space(3)};
`;

const CredItem = styled.li`
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: ${({ theme }) => theme.space(2)};
`;

const Actions = styled.div`
  display: flex;
  gap: ${({ theme }) => theme.space(2)};
  flex-wrap: wrap;
`;

const Center = styled.div`
  display: grid;
  place-items: center;
  padding: ${({ theme }) => theme.space(10)};
  color: ${({ theme }) => theme.colors.textMuted};
`;

/** Profile detail with credential methods and provider enrichment. */
export function ProfilePage() {
  const { id = '' } = useParams();
  const { api } = useAuth();
  const [details, setDetails] = useState<UserDetails | null>(null);
  const [providers, setProviders] = useState<string[]>([]);
  const [loadError, setLoadError] = useState<DescribedError | null>(null);
  const [enrichment, setEnrichment] = useState<EnrichedProfile | null>(null);
  const [enriching, setEnriching] = useState<string | null>(null);
  const [enrichError, setEnrichError] = useState<DescribedError | null>(null);

  useEffect(() => {
    let cancelled = false;
    Promise.all([api.getUser(id), api.listProviders()])
      .then(([d, p]) => {
        if (cancelled) return;
        setDetails(d);
        setProviders(p);
      })
      .catch((err: unknown) => !cancelled && setLoadError(describeError(err)));
    return () => {
      cancelled = true;
    };
  }, [api, id]);

  async function enrich(which: string[], key: string) {
    setEnriching(key);
    setEnrichError(null);
    try {
      setEnrichment(await api.enrichUser(id, which));
    } catch (err) {
      setEnrichError(describeError(err));
    } finally {
      setEnriching(null);
    }
  }

  if (loadError) {
    return (
      <>
        <Alert requestId={loadError.requestId}>{loadError.message}</Alert>
        <Link to="/">Back to search</Link>
      </>
    );
  }
  if (!details) {
    return (
      <Center>
        <Spinner $size={24} />
      </Center>
    );
  }

  const { profile, credentials } = details;
  const a = profile.address;
  const addressLine = [a.street_address, a.locality, a.region, a.postal_code, a.country]
    .filter(Boolean)
    .join(', ');

  return (
    <>
      <Link to="/">&larr; Back to search</Link>
      <Grid>
        <Card>
          <CardHeader>
            <div>
              <CardTitle as="h1">{profile.name}</CardTitle>
              <Muted>Stored profile</Muted>
            </div>
          </CardHeader>
          <Details>
            <dt>Phone</dt>
            <dd>{profile.phone}</dd>
            <dt>Address</dt>
            <dd>{addressLine || <Muted as="span">No address on file</Muted>}</dd>
            <dt>User ID</dt>
            <dd>
              <code>{profile.user_id}</code>
            </dd>
          </Details>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Credentials</CardTitle>
          </CardHeader>
          <CredList>
            {credentials.map((c) => (
              <CredItem key={c.id}>
                <span>{c.username}</span>
                <Badge $tone="info">{c.method}</Badge>
              </CredItem>
            ))}
          </CredList>
        </Card>
      </Grid>

      <Card>
        <CardHeader>
          <div>
            <CardTitle>Identity verification</CardTitle>
            <Muted>
              Query third-party providers. Local data stays the source of truth; providers fill gaps
              and confirm matching fields.
            </Muted>
          </div>
          <Actions>
            {providers.map((p) => (
              <Button
                key={p}
                variant="secondary"
                loading={enriching === p}
                disabled={enriching !== null}
                onClick={() => enrich([p], p)}
              >
                Check {p.toUpperCase()}
              </Button>
            ))}
            {providers.length > 1 && (
              <Button
                loading={enriching === 'all'}
                disabled={enriching !== null}
                onClick={() => enrich([], 'all')}
              >
                Check all
              </Button>
            )}
          </Actions>
        </CardHeader>
        {providers.length === 0 && <Alert tone="neutral">No identity providers configured.</Alert>}
        {enrichError && <Alert requestId={enrichError.requestId}>{enrichError.message}</Alert>}
        {enrichment && <EnrichmentResults result={enrichment} />}
      </Card>
    </>
  );
}
