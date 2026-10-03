import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';
import styled from 'styled-components';
import { useAuth } from '../auth/auth-context';
import { Button } from './button';
import { Page } from './page';

const Header = styled.header`
  background: ${({ theme }) => theme.colors.surface};
  border-bottom: 1px solid ${({ theme }) => theme.colors.border};
`;

const HeaderInner = styled.div`
  max-width: 1040px;
  margin: 0 auto;
  padding: ${({ theme }) => `${theme.space(3)} ${theme.space(6)}`};
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: ${({ theme }) => theme.space(4)};
`;

const Brand = styled(Link)`
  display: flex;
  align-items: center;
  gap: ${({ theme }) => theme.space(2)};
  font-weight: 800;
  font-size: 18px;
  color: ${({ theme }) => theme.colors.text};
  &:hover {
    text-decoration: none;
  }
`;

const Logo = styled.span`
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  border-radius: 8px;
  background: ${({ theme }) => theme.colors.primary};
  color: ${({ theme }) => theme.colors.primaryText};
  font-size: 15px;
`;

const Nav = styled.nav`
  display: flex;
  align-items: center;
  gap: ${({ theme }) => theme.space(2)};
  font-size: 14px;
`;

const User = styled.span`
  color: ${({ theme }) => theme.colors.textMuted};
  margin: 0 ${({ theme }) => theme.space(2)};
`;

/** Authenticated application shell with header navigation. */
export function AppLayout({ children }: { children: ReactNode }) {
  const { session, logout } = useAuth();
  return (
    <>
      <Header>
        <HeaderInner>
          <Brand to="/">
            <Logo aria-hidden>R</Logo>
            Rolodex
          </Brand>
          <Nav aria-label="Main">
            <Link to="/">Search</Link>
            <Link to="/users/new">Add user</Link>
            {session && <User>Signed in as {session.username}</User>}
            <Button variant="ghost" onClick={logout}>
              Sign out
            </Button>
          </Nav>
        </HeaderInner>
      </Header>
      <Page>{children}</Page>
    </>
  );
}
