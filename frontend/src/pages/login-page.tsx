import { useState, type FormEvent } from 'react';
import { Navigate, useLocation, useNavigate } from 'react-router-dom';
import styled from 'styled-components';
import { describeError, type DescribedError } from '../api/describe-error';
import { useAuth } from '../auth/auth-context';
import { SignInCancelledError } from '../auth/google-sign-in';
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

const Divider = styled.div`
  display: flex;
  align-items: center;
  gap: ${({ theme }) => theme.space(3)};
  margin: ${({ theme }) => `${theme.space(5)} 0`};
  color: ${({ theme }) => theme.colors.textMuted};
  font-size: 13px;

  &::before,
  &::after {
    content: '';
    flex: 1;
    border-top: 1px solid ${({ theme }) => theme.colors.border};
  }
`;

const FullWidthButton = styled(Button)`
  width: 100%;
`;

/** Google "G" mark, as required by Google's sign-in branding guidelines. */
function GoogleMark() {
  return (
    <svg width="18" height="18" viewBox="0 0 48 48" aria-hidden="true">
      <path
        fill="#FFC107"
        d="M43.6 20.5H42V20H24v8h11.3C33.7 32.7 29.2 36 24 36c-6.6 0-12-5.4-12-12s5.4-12 12-12c3.1 0 5.8 1.2 7.9 3.1l5.7-5.7C34 6.1 29.3 4 24 4 12.9 4 4 12.9 4 24s8.9 20 20 20 20-8.9 20-20c0-1.3-.1-2.4-.4-3.5z"
      />
      <path
        fill="#FF3D00"
        d="M6.3 14.7l6.6 4.8C14.7 15.1 19 12 24 12c3.1 0 5.8 1.2 7.9 3.1l5.7-5.7C34 6.1 29.3 4 24 4 16.3 4 9.7 8.3 6.3 14.7z"
      />
      <path
        fill="#4CAF50"
        d="M24 44c5.2 0 9.9-2 13.4-5.2l-6.2-5.2C29.2 35.1 26.7 36 24 36c-5.2 0-9.6-3.3-11.3-8l-6.5 5C9.5 39.6 16.2 44 24 44z"
      />
      <path
        fill="#1976D2"
        d="M43.6 20.5H42V20H24v8h11.3c-.8 2.2-2.2 4.2-4.1 5.6l6.2 5.2C37 39.2 44 34 44 24c0-1.3-.1-2.4-.4-3.5z"
      />
    </svg>
  );
}

const firebaseMessages: Record<string, string> = {
  'auth/popup-blocked':
    'Your browser blocked the Google sign-in window. Allow pop-ups and try again.',
  'auth/unauthorized-domain':
    'This site is not an authorised domain for Google sign-in. Add it in the Firebase console.',
  'auth/network-request-failed': 'Unable to reach Google. Check your connection and try again.',
};

/** Maps Google sign-in failures to a message; null means the user cancelled. */
function describeGoogleError(err: unknown): DescribedError | null {
  if (err instanceof SignInCancelledError) return null;
  const code = (err as { code?: unknown } | null)?.code;
  const message = typeof code === 'string' ? firebaseMessages[code] : undefined;
  return message ? { message, fields: {} } : describeError(err);
}

/** Sign-in screen: username/password, plus Google when Firebase is configured. */
export function LoginPage() {
  const { session, login, loginWithGoogle, googleEnabled, demo } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<DescribedError | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [googleBusy, setGoogleBusy] = useState(false);

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

  async function onGoogle() {
    setError(null);
    setGoogleBusy(true);
    try {
      await loginWithGoogle();
      navigate(from, { replace: true });
    } catch (err) {
      setError(describeGoogleError(err));
    } finally {
      setGoogleBusy(false);
    }
  }

  const busy = submitting || googleBusy;
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
          <Button
            type="submit"
            loading={submitting}
            disabled={!username || !password || googleBusy}
          >
            Sign in
          </Button>
        </Form>
        {googleEnabled && (
          <>
            <Divider>or</Divider>
            <FullWidthButton
              variant="secondary"
              onClick={onGoogle}
              loading={googleBusy}
              disabled={busy}
            >
              {!googleBusy && <GoogleMark />}
              Sign in with Google
            </FullWidthButton>
          </>
        )}
      </Panel>
    </Screen>
  );
}
