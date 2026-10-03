import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type { ReactElement } from 'react';
import { ThemeProvider } from 'styled-components';
import { describe, expect, it, vi } from 'vitest';
import type { EnrichedProfile } from '../api/types';
import { theme } from '../styles/theme';
import { Alert } from './alert';
import { Button } from './button';
import { EnrichmentResults } from './enrichment-results';
import { TextField } from './text-field';

const themed = (ui: ReactElement) => render(<ThemeProvider theme={theme}>{ui}</ThemeProvider>);

describe('Button', () => {
  it('fires onClick and is disabled while loading', async () => {
    const onClick = vi.fn();
    const { rerender } = themed(<Button onClick={onClick}>Save</Button>);
    await userEvent.click(screen.getByRole('button', { name: 'Save' }));
    expect(onClick).toHaveBeenCalledOnce();

    rerender(
      <ThemeProvider theme={theme}>
        <Button loading onClick={onClick}>
          Save
        </Button>
      </ThemeProvider>,
    );
    expect(screen.getByRole('button')).toBeDisabled();
    expect(screen.getByRole('status', { name: 'Loading' })).toBeInTheDocument();
  });

  it('exposes pressed state for secondary toggles', () => {
    themed(
      <Button variant="secondary" active>
        Name
      </Button>,
    );
    expect(screen.getByRole('button', { name: 'Name' })).toHaveAttribute('aria-pressed', 'true');
  });

  it('does not expose pressed state for plain buttons', () => {
    themed(<Button variant="secondary">Check ABC</Button>);
    expect(screen.getByRole('button', { name: 'Check ABC' })).not.toHaveAttribute('aria-pressed');
  });
});

describe('TextField', () => {
  it('associates label and error message', () => {
    themed(<TextField label="Phone" error="invalid phone number" />);
    const input = screen.getByLabelText('Phone');
    expect(input).toHaveAttribute('aria-invalid', 'true');
    expect(input).toHaveAccessibleDescription('invalid phone number');
  });

  it('shows a hint when there is no error', () => {
    themed(<TextField label="Country" hint="ISO code" />);
    expect(screen.getByLabelText('Country')).toHaveAccessibleDescription('ISO code');
  });
});

describe('Alert', () => {
  it('renders the request id for support', () => {
    themed(<Alert requestId="req-42">Failed</Alert>);
    expect(screen.getByRole('alert')).toHaveTextContent('Failed');
    expect(screen.getByText(/req-42/)).toBeInTheDocument();
  });
});

describe('EnrichmentResults', () => {
  it('shows provider status, provenance and verification', () => {
    const result: EnrichedProfile = {
      user_id: 'u1',
      fields: {
        name: { value: 'Grace Hopper', source: 'local', verified_by: ['abc'] },
        street_address: { value: '350 Fifth Ave', source: 'abc', verified_by: [] },
        country: { value: '', source: '', verified_by: [] },
      },
      providers: [
        { provider: 'abc', status: 'ok' },
        { provider: 'xyc', status: 'unavailable', error: 'provider temporarily unavailable' },
      ],
    };
    themed(<EnrichmentResults result={result} />);

    expect(screen.getByTestId('provider-abc')).toHaveTextContent('Matched');
    expect(screen.getByTestId('provider-xyc')).toHaveTextContent('Unavailable');
    expect(screen.getByText('Added by ABC')).toBeInTheDocument();
    expect(screen.getByRole('row', { name: /Name Grace Hopper Local ABC/ })).toBeInTheDocument();
    expect(screen.getByRole('row', { name: /Country Unknown/ })).toBeInTheDocument();
  });
});
