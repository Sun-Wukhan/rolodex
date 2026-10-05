import { useState, type FormEvent } from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';
import { describeError, type DescribedError } from '../api/describe-error';
import type { Profile } from '../api/types';
import { useAuth } from '../auth/auth-context';
import { Alert } from '../components/alert';
import { Badge } from '../components/badge';
import { Button } from '../components/button';
import { Card, CardHeader, CardTitle, Muted } from '../components/card';
import { TextField } from '../components/text-field';
import { getLastSearch, setLastSearch, type SearchField } from './search-cache';

const FIELDS: { key: SearchField; label: string; placeholder: string }[] = [
  { key: 'name', label: 'Name', placeholder: 'e.g. Grace' },
  { key: 'phone', label: 'Phone', placeholder: 'e.g. 212-555-0102' },
  { key: 'username', label: 'Username', placeholder: 'e.g. ada' },
];

const Segmented = styled.div`
  display: flex;
  gap: ${({ theme }) => theme.space(2)};
  margin-bottom: ${({ theme }) => theme.space(4)};
`;

const Row = styled.form`
  display: flex;
  gap: ${({ theme }) => theme.space(3)};
  align-items: flex-end;
  & > :first-child {
    flex: 1;
  }
`;

const Results = styled.ul`
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: ${({ theme }) => theme.space(4)};
`;

const ResultLink = styled(Link)`
  display: block;
  height: 100%;
  padding: ${({ theme }) => theme.space(5)};
  border: 1px solid ${({ theme }) => theme.colors.border};
  border-radius: ${({ theme }) => theme.radii.md};
  background: ${({ theme }) => theme.colors.surface};
  color: inherit;
  transition:
    border-color 0.15s,
    box-shadow 0.15s;
  &:hover {
    text-decoration: none;
    border-color: ${({ theme }) => theme.colors.primary};
    box-shadow: ${({ theme }) => theme.shadow.md};
  }
`;

const Name = styled.div`
  font-weight: 700;
  margin-bottom: ${({ theme }) => theme.space(1)};
`;

const Mono = styled.div`
  font-family: ${({ theme }) => theme.font.mono};
  font-size: 13px;
  color: ${({ theme }) => theme.colors.textMuted};
`;

const Meta = styled.div`
  margin-top: ${({ theme }) => theme.space(3)};
  display: flex;
  gap: ${({ theme }) => theme.space(2)};
  flex-wrap: wrap;
`;

/** Search profiles by name, phone or username. */
export function SearchPage() {
  const { api } = useAuth();
  const [initial] = useState(getLastSearch);
  const [field, setField] = useState<SearchField>(initial.field);
  const [term, setTerm] = useState(initial.term);
  const [results, setResults] = useState<Profile[] | null>(initial.results);
  const [error, setError] = useState<DescribedError | null>(null);
  const [loading, setLoading] = useState(false);

  const active = FIELDS.find((f) => f.key === field) ?? FIELDS[0]!;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!term.trim()) return;
    setError(null);
    setLoading(true);
    try {
      const res = await api.searchUsers({ [field]: term.trim(), limit: 50 });
      setResults(res.data);
      setLastSearch({ field, term, results: res.data });
    } catch (err) {
      setError(describeError(err));
      setResults(null);
    } finally {
      setLoading(false);
    }
  }

  return (
    <>
      <Card>
        <CardHeader>
          <div>
            <CardTitle>Find a person</CardTitle>
            <Muted>Search the directory, then open a profile to verify it with providers.</Muted>
          </div>
        </CardHeader>
        <Segmented role="group" aria-label="Search by">
          {FIELDS.map((f) => (
            <Button
              key={f.key}
              variant="secondary"
              active={f.key === field}
              onClick={() => setField(f.key)}
            >
              {f.label}
            </Button>
          ))}
        </Segmented>
        <Row onSubmit={onSubmit} role="search">
          <TextField
            label={`Search by ${active.label.toLowerCase()}`}
            placeholder={active.placeholder}
            value={term}
            onChange={(e) => setTerm(e.target.value)}
            type={field === 'phone' ? 'tel' : 'search'}
          />
          <Button type="submit" loading={loading} disabled={!term.trim()}>
            Search
          </Button>
        </Row>
      </Card>

      {error && (
        <Alert requestId={error.requestId}>{Object.values(error.fields)[0] ?? error.message}</Alert>
      )}

      {results && results.length === 0 && <Alert tone="neutral">No profiles matched.</Alert>}

      {results && results.length > 0 && (
        <section aria-label="Search results">
          <Muted style={{ marginBottom: 12 }}>
            {results.length} result{results.length === 1 ? '' : 's'}
          </Muted>
          <Results>
            {results.map((p) => (
              <li key={p.user_id}>
                <ResultLink to={`/users/${p.user_id}`}>
                  <Name>{p.name}</Name>
                  <Mono>{p.phone}</Mono>
                  <Meta>
                    {p.address.locality && <Badge>{p.address.locality}</Badge>}
                    {p.address.country && <Badge $tone="info">{p.address.country}</Badge>}
                  </Meta>
                </ResultLink>
              </li>
            ))}
          </Results>
        </section>
      )}
    </>
  );
}
