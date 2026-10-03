import { useState, type ChangeEvent, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import styled from 'styled-components';
import { describeError, type DescribedError } from '../api/describe-error';
import type { CreateUserInput } from '../api/types';
import { useAuth } from '../auth/auth-context';
import { Alert } from '../components/alert';
import { Button } from '../components/button';
import { Card, CardHeader, CardTitle, Muted } from '../components/card';
import { TextField } from '../components/text-field';

type FormState = Omit<CreateUserInput, 'address'> & {
  street_address: string;
  locality: string;
  region: string;
  postal_code: string;
  country: string;
};

const initial: FormState = {
  name: '',
  phone: '',
  username: '',
  password: '',
  street_address: '',
  locality: '',
  region: '',
  postal_code: '',
  country: '',
};

const Form = styled.form`
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: ${({ theme }) => theme.space(4)};
  @media (max-width: 700px) {
    grid-template-columns: 1fr;
  }
`;

const Section = styled.h3`
  grid-column: 1 / -1;
  font-size: 13px;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: ${({ theme }) => theme.colors.textMuted};
  margin-top: ${({ theme }) => theme.space(2)};
`;

const Footer = styled.div`
  grid-column: 1 / -1;
  display: flex;
  justify-content: flex-end;
`;

/** Form to register a user; renders server-side validation errors per field. */
export function CreateUserPage() {
  const { api } = useAuth();
  const navigate = useNavigate();
  const [form, setForm] = useState<FormState>(initial);
  const [error, setError] = useState<DescribedError | null>(null);
  const [saving, setSaving] = useState(false);

  const bind = (key: keyof FormState) => ({
    name: key,
    value: form[key],
    onChange: (e: ChangeEvent<HTMLInputElement>) => setForm({ ...form, [key]: e.target.value }),
    error: error?.fields[key] ?? error?.fields[`address.${key}`],
  });

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    const { street_address, locality, region, postal_code, country, ...rest } = form;
    try {
      const user = await api.createUser({
        ...rest,
        address: { street_address, locality, region, postal_code, country },
      });
      navigate(`/users/${user.id}`);
    } catch (err) {
      setError(describeError(err));
    } finally {
      setSaving(false);
    }
  }

  const hasFieldErrors = error && Object.keys(error.fields).length > 0;

  return (
    <Card>
      <CardHeader>
        <div>
          <CardTitle as="h1">Add user</CardTitle>
          <Muted>Creates a profile and a password credential.</Muted>
        </div>
      </CardHeader>
      {error && !hasFieldErrors && <Alert requestId={error.requestId}>{error.message}</Alert>}
      <Form onSubmit={onSubmit} noValidate>
        <Section>Profile</Section>
        <TextField label="Full name" {...bind('name')} required />
        <TextField label="Phone" type="tel" {...bind('phone')} required />
        <Section>Address</Section>
        <TextField label="Street address" {...bind('street_address')} />
        <TextField label="Locality" {...bind('locality')} />
        <TextField label="Region" {...bind('region')} />
        <TextField label="Postal code" {...bind('postal_code')} />
        <TextField label="Country" hint="ISO code, e.g. CA" {...bind('country')} />
        <Section>Credential</Section>
        <TextField label="Username" autoComplete="off" {...bind('username')} required />
        <TextField
          label="Password"
          type="password"
          autoComplete="new-password"
          hint="At least 12 characters"
          {...bind('password')}
          required
        />
        <Footer>
          <Button type="submit" loading={saving}>
            Create user
          </Button>
        </Footer>
      </Form>
    </Card>
  );
}
