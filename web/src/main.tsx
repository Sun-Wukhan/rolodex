import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router-dom';
import { ThemeProvider } from 'styled-components';
import { App } from './app';
import { AuthProvider } from './auth/auth-provider';
import { GlobalStyle } from './styles/global-style';
import { theme } from './styles/theme';

const apiUrl = import.meta.env.VITE_API_URL ?? 'http://localhost:8080';

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ThemeProvider theme={theme}>
      <GlobalStyle />
      <BrowserRouter>
        <AuthProvider baseUrl={apiUrl}>
          <App />
        </AuthProvider>
      </BrowserRouter>
    </ThemeProvider>
  </StrictMode>,
);
