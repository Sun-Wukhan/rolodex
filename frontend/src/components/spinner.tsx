import styled, { keyframes } from 'styled-components';

const spin = keyframes`to { transform: rotate(360deg); }`;

/** Inline loading indicator that inherits the current text colour. */
export const Spinner = styled.span.attrs({ role: 'status', 'aria-label': 'Loading' })<{
  $size?: number;
}>`
  display: inline-block;
  width: ${({ $size = 16 }) => $size}px;
  height: ${({ $size = 16 }) => $size}px;
  border: 2px solid currentColor;
  border-right-color: transparent;
  border-radius: 50%;
  animation: ${spin} 0.7s linear infinite;
  vertical-align: middle;
`;
