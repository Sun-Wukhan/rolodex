import type { ButtonHTMLAttributes } from 'react';
import styled, { css } from 'styled-components';
import { Spinner } from './spinner';

export type ButtonVariant = 'primary' | 'secondary' | 'ghost';

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  loading?: boolean;
  /** Set only for toggle buttons; exposes aria-pressed. */
  active?: boolean;
}

const StyledButton = styled.button<{ $variant: ButtonVariant; $active: boolean }>`
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: ${({ theme }) => theme.space(2)};
  padding: ${({ theme }) => `${theme.space(2.5)} ${theme.space(4)}`};
  border-radius: ${({ theme }) => theme.radii.sm};
  border: 1px solid transparent;
  font: inherit;
  font-weight: 600;
  cursor: pointer;
  transition:
    background 0.15s,
    border-color 0.15s,
    color 0.15s;

  &:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  ${({ $variant, $active, theme }) => {
    switch ($variant) {
      case 'secondary':
        return css`
          background: ${$active ? theme.colors.primary : theme.colors.surface};
          color: ${$active ? theme.colors.primaryText : theme.colors.text};
          border-color: ${$active ? theme.colors.primary : theme.colors.border};
          &:hover:not(:disabled) {
            border-color: ${theme.colors.primary};
          }
        `;
      case 'ghost':
        return css`
          background: transparent;
          color: ${theme.colors.textMuted};
          &:hover:not(:disabled) {
            color: ${theme.colors.text};
            background: ${theme.colors.surfaceMuted};
          }
        `;
      default:
        return css`
          background: ${theme.colors.primary};
          color: ${theme.colors.primaryText};
          &:hover:not(:disabled) {
            background: ${theme.colors.primaryHover};
          }
        `;
    }
  }}
`;

/** Button with visual variants and a built-in loading state. */
export function Button({
  variant = 'primary',
  loading = false,
  active,
  disabled,
  children,
  type = 'button',
  ...rest
}: ButtonProps) {
  return (
    <StyledButton
      type={type}
      $variant={variant}
      $active={Boolean(active)}
      aria-pressed={active}
      aria-busy={loading || undefined}
      disabled={disabled || loading}
      {...rest}
    >
      {loading && <Spinner $size={14} />}
      {children}
    </StyledButton>
  );
}
