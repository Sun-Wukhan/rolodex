import styled from 'styled-components';
import { toneColors, type Tone } from '../styles/theme';

/** Compact pill for statuses, sources and credential methods. */
export const Badge = styled.span<{ $tone?: Tone }>`
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 10px;
  border-radius: ${({ theme }) => theme.radii.pill};
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;
  color: ${({ theme, $tone = 'neutral' }) => toneColors(theme, $tone).fg};
  background: ${({ theme, $tone = 'neutral' }) => toneColors(theme, $tone).bg};
`;
