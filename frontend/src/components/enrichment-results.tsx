import styled from 'styled-components';
import type { EnrichedProfile, ProviderStatus } from '../api/types';
import type { Tone } from '../styles/theme';
import { Badge } from './badge';

const FIELD_ORDER: { key: string; label: string }[] = [
  { key: 'name', label: 'Name' },
  { key: 'phone', label: 'Phone' },
  { key: 'street_address', label: 'Street address' },
  { key: 'locality', label: 'Locality' },
  { key: 'region', label: 'Region' },
  { key: 'postal_code', label: 'Postal code' },
  { key: 'country', label: 'Country' },
];

const STATUS: Record<ProviderStatus, { tone: Tone; label: string }> = {
  ok: { tone: 'success', label: 'Matched' },
  not_found: { tone: 'neutral', label: 'No match' },
  unavailable: { tone: 'warning', label: 'Unavailable' },
  error: { tone: 'danger', label: 'Error' },
};

const Providers = styled.div`
  display: flex;
  flex-wrap: wrap;
  gap: ${({ theme }) => theme.space(3)};
  margin-bottom: ${({ theme }) => theme.space(5)};
`;

const ProviderChip = styled.div`
  display: flex;
  align-items: center;
  gap: ${({ theme }) => theme.space(2)};
  padding: ${({ theme }) => `${theme.space(2)} ${theme.space(3)}`};
  border: 1px solid ${({ theme }) => theme.colors.border};
  border-radius: ${({ theme }) => theme.radii.sm};
  font-weight: 600;
  text-transform: uppercase;
  font-size: 13px;
`;

const Table = styled.table`
  width: 100%;
  border-collapse: collapse;
  font-size: 14px;

  th,
  td {
    text-align: left;
    padding: ${({ theme }) => `${theme.space(2.5)} ${theme.space(3)}`};
    border-bottom: 1px solid ${({ theme }) => theme.colors.border};
    vertical-align: middle;
  }
  th {
    font-size: 12px;
    text-transform: uppercase;
    letter-spacing: 0.04em;
    color: ${({ theme }) => theme.colors.textMuted};
  }
`;

const Empty = styled.span`
  color: ${({ theme }) => theme.colors.textMuted};
`;

const Badges = styled.span`
  display: inline-flex;
  gap: 6px;
  flex-wrap: wrap;
`;

/** Displays per-provider status and the merged fields with provenance. */
export function EnrichmentResults({ result }: { result: EnrichedProfile }) {
  return (
    <div>
      <Providers aria-label="Provider results">
        {result.providers.map((p) => (
          <ProviderChip key={p.provider} data-testid={`provider-${p.provider}`}>
            {p.provider}
            <Badge $tone={STATUS[p.status].tone}>{STATUS[p.status].label}</Badge>
          </ProviderChip>
        ))}
      </Providers>
      <Table>
        <thead>
          <tr>
            <th scope="col">Field</th>
            <th scope="col">Value</th>
            <th scope="col">Source</th>
            <th scope="col">Verified by</th>
          </tr>
        </thead>
        <tbody>
          {FIELD_ORDER.map(({ key, label }) => {
            const f = result.fields[key];
            return (
              <tr key={key}>
                <th scope="row">{label}</th>
                <td>{f?.value ? f.value : <Empty>Unknown</Empty>}</td>
                <td>
                  {f?.source ? (
                    <Badge $tone={f.source === 'local' ? 'neutral' : 'info'}>
                      {f.source === 'local' ? 'Local' : `Added by ${f.source.toUpperCase()}`}
                    </Badge>
                  ) : (
                    <Empty>-</Empty>
                  )}
                </td>
                <td>
                  {f && f.verified_by.length > 0 ? (
                    <Badges>
                      {f.verified_by.map((v) => (
                        <Badge key={v} $tone="success">
                          {v.toUpperCase()}
                        </Badge>
                      ))}
                    </Badges>
                  ) : (
                    <Empty>Not verified</Empty>
                  )}
                </td>
              </tr>
            );
          })}
        </tbody>
      </Table>
    </div>
  );
}
