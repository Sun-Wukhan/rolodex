import styled from 'styled-components';

/** Elevated surface used to group related content. */
export const Card = styled.section`
  background: ${({ theme }) => theme.colors.surface};
  border: 1px solid ${({ theme }) => theme.colors.border};
  border-radius: ${({ theme }) => theme.radii.md};
  box-shadow: ${({ theme }) => theme.shadow.sm};
  padding: ${({ theme }) => theme.space(6)};
`;

/** Header row inside a Card: title on the left, actions on the right. */
export const CardHeader = styled.header`
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: ${({ theme }) => theme.space(3)};
  margin-bottom: ${({ theme }) => theme.space(4)};
`;

/** Card heading. */
export const CardTitle = styled.h2`
  font-size: 17px;
  font-weight: 700;
`;

/** Secondary text below a title. */
export const Muted = styled.p`
  margin: 0;
  color: ${({ theme }) => theme.colors.textMuted};
  font-size: 13px;
`;
