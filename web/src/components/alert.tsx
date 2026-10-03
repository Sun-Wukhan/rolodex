import type { ReactNode } from 'react';
import styled from 'styled-components';
import { toneColors, type Tone } from '../styles/theme';

const Box = styled.div<{ $tone: Tone }>`
  padding: ${({ theme }) => `${theme.space(3)} ${theme.space(4)}`};
  border-radius: ${({ theme }) => theme.radii.sm};
  font-size: 14px;
  color: ${({ theme, $tone }) => toneColors(theme, $tone).fg};
  background: ${({ theme, $tone }) => toneColors(theme, $tone).bg};
`;

const Ref = styled.span`
  display: block;
  margin-top: 4px;
  font-family: ${({ theme }) => theme.font.mono};
  font-size: 11px;
  opacity: 0.8;
`;

interface AlertProps {
  tone?: Tone;
  requestId?: string;
  children: ReactNode;
}

/** Inline message. Shows the API request ID so issues can be traced in logs. */
export function Alert({ tone = 'danger', requestId, children }: AlertProps) {
  return (
    <Box $tone={tone} role={tone === 'danger' ? 'alert' : 'status'}>
      {children}
      {requestId && <Ref>Request ID: {requestId}</Ref>}
    </Box>
  );
}
