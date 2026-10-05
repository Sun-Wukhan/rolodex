import { useEffect, type ReactNode } from 'react';
import { Navigate, Route, Routes } from 'react-router-dom';
import { useAuth } from './auth/auth-context';
import { RequireAuth } from './auth/require-auth';
import { AppLayout } from './components/layout';
import { CreateUserPage } from './pages/create-user-page';
import { LoginPage } from './pages/login-page';
import { ProfilePage } from './pages/profile-page';
import { clearSearchCache } from './pages/search-cache';
import { SearchPage } from './pages/search-page';

/** Application routes. Everything except /login requires a session. */
export function App() {
  const { session } = useAuth();

  useEffect(() => {
    if (!session) clearSearchCache();
  }, [session]);

  const protect = (page: ReactNode) => (
    <RequireAuth>
      <AppLayout>{page}</AppLayout>
    </RequireAuth>
  );

  return (
    <Routes>
      <Route path="/login" element={<LoginPage />} />
      <Route path="/" element={protect(<SearchPage />)} />
      <Route path="/users/new" element={protect(<CreateUserPage />)} />
      <Route path="/users/:id" element={protect(<ProfilePage />)} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
