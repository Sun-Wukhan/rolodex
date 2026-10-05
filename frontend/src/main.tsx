import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { ThemeProvider } from 'styled-components';
import { App } from './app';
import { AuthProvider } from './auth/auth-provider';
import { createGoogleSignIn, firebaseConfigFromEnv } from './auth/google-sign-in';
import { createDemoFetch } from './demo/demo-api';
import { GlobalStyle } from './styles/global-style';
import { theme } from './styles/theme';

const demo = import.meta.env.VITE_DEMO_MODE === 'true';
const apiUrl = demo ? '' : (import.meta.env.VITE_API_URL ?? 'http://localhost:8080');
const fetchImpl = demo ? createDemoFetch({ latencyMs: 250 }) : undefined;
const firebaseConfig = demo ? null : firebaseConfigFromEnv(import.meta.env);
const googleSignIn = firebaseConfig ? createGoogleSignIn(firebaseConfig) : undefined;

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ThemeProvider theme={theme}>
      <GlobalStyle />
      <BrowserRouter basename={import.meta.env.BASE_URL}>
        <AuthProvider
          baseUrl={apiUrl}
          fetchImpl={fetchImpl}
          demo={demo}
          googleSignIn={googleSignIn}
        >
          <App />
        </AuthProvider>
      </BrowserRouter>
    </ThemeProvider>
  </StrictMode>,
);
