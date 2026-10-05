import { useId, type InputHTMLAttributes } from 'react';
import styled from 'styled-components';

interface TextFieldProps extends InputHTMLAttributes<HTMLInputElement> {
  label: string;
  error?: string;
  hint?: string;
}

const Wrapper = styled.div`
  display: flex;
  flex-direction: column;
  gap: ${({ theme }) => theme.space(1.5)};
`;

const Label = styled.label`
  font-size: 13px;
  font-weight: 600;
  color: ${({ theme }) => theme.colors.text};
`;

const Input = styled.input<{ $invalid: boolean }>`
  padding: ${({ theme }) => `${theme.space(2.5)} ${theme.space(3)}`};
  border-radius: ${({ theme }) => theme.radii.sm};
  border: 1px solid
    ${({ theme, $invalid }) => ($invalid ? theme.colors.danger : theme.colors.border)};
  background: ${({ theme }) => theme.colors.surface};
  font: inherit;
  color: inherit;

  &:focus {
    outline: none;
    border-color: ${({ theme }) => theme.colors.primary};
    box-shadow: 0 0 0 3px rgba(79, 70, 229, 0.15);
  }
`;

const Help = styled.span<{ $error?: boolean }>`
  font-size: 12px;
  color: ${({ theme, $error }) => ($error ? theme.colors.danger : theme.colors.textMuted)};
`;

/** Labelled input with optional hint and accessible error message. */
export function TextField({ label, error, hint, id, ...rest }: TextFieldProps) {
  const generated = useId();
  const inputId = id ?? generated;
  const helpId = `${inputId}-help`;
  return (
    <Wrapper>
      <Label htmlFor={inputId}>{label}</Label>
      <Input
        id={inputId}
        $invalid={Boolean(error)}
        aria-invalid={Boolean(error) || undefined}
        aria-describedby={error || hint ? helpId : undefined}
        {...rest}
      />
      {(error || hint) && (
        <Help id={helpId} $error={Boolean(error)}>
          {error ?? hint}
        </Help>
      )}
    </Wrapper>
  );
}
