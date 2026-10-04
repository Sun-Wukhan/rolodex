import { useState, type FormEvent } from 'react';
import { Navigate, useLocation, useNavigate } from 'react-router-dom';
import styled from 'styled-components';
import { describeError, type DescribedError } from '../api/describe-error';
import { useAuth } from '../auth/auth-context';
import { Alert } from '../components/alert';
import { Button } from '../components/button';
import { Card, Muted } from '../components/card';
import { TextField } from '../components/text-field';

const Screen = styled.div`
  min-height: 100%;
  display: grid;
  place-items: center;
  padding: ${({ theme }) => theme.space(6)};
`;

const Panel = styled(Card)`
  width: 100%;
  max-width: 400px;
  padding: ${({ theme }) => theme.space(8)};
`;

const Title = styled.h1`
  font-size: 24px;
  margin-bottom: ${({ theme }) => theme.space(1)};
`;

const Form = styled.form`
  display: flex;
  flex-direction: column;
  gap: ${({ theme }) => theme.space(4)};
  margin-top: ${({ theme }) => theme.space(6)};
`;

/** Username/password sign-in screen. */
export function LoginPage() {
  const { session, login, demo } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<DescribedError | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const from = (location.state as { from?: string } | null)?.from ?? '/';
  if (session) return <Navigate to={from} replace />;

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    setError(null);
    setSubmitting(true);
    try {
      await login(username.trim(), password);
      navigate(from, { replace: true });
    } catch (err) {
      setError(describeError(err));
      setPassword('');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Screen>
      <Panel>
        <Title>Sign in to Rolodex</Title>
        <Muted>Search profiles and verify identities with ABC and XYC.</Muted>
        <Form onSubmit={onSubmit} noValidate>
          {demo && (
            <Alert tone="info">
              Demo mode: data lives in your browser. Sign in as <strong>admin</strong>,{' '}
              <strong>ada</strong>, <strong>grace</strong>, <strong>alan</strong> or{' '}
              <strong>katherine</strong> with any password of 12+ characters.
            </Alert>
          )}
          {error && <Alert requestId={error.requestId}>{error.message}</Alert>}
          <TextField
            label="Username"
            name="username"
            autoComplete="username"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            required
            autoFocus
          />
          <TextField
            label="Password"
            name="password"
            type="password"
            autoComplete="current-password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            required
          />
          <Button type="submit" loading={submitting} disabled={!username || !password}>
            Sign in
          </Button>
        </Form>
      </Panel>
    </Screen>
  );
}
