import styled from 'styled-components';

/** Centered content column. */
export const Page = styled.main`
  max-width: 1040px;
  margin: 0 auto;
  padding: ${({ theme }) => theme.space(8)} ${({ theme }) => theme.space(6)};
  display: flex;
  flex-direction: column;
  gap: ${({ theme }) => theme.space(6)};
`;
