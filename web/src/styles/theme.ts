/** Design tokens shared by every styled component. */
export const theme = {
  colors: {
    bg: '#f5f6fa',
    surface: '#ffffff',
    surfaceMuted: '#f0f2f7',
    border: '#e2e5ee',
    text: '#1c2033',
    textMuted: '#646b85',
    primary: '#4f46e5',
    primaryHover: '#4338ca',
    primaryText: '#ffffff',
    success: '#15803d',
    successBg: '#dcfce7',
    warning: '#b45309',
    warningBg: '#fef3c7',
    danger: '#b91c1c',
    dangerBg: '#fee2e2',
    info: '#1d4ed8',
    infoBg: '#dbeafe',
    neutralBg: '#eceef4',
  },
  radii: { sm: '6px', md: '10px', lg: '16px', pill: '999px' },
  space: (n: number): string => `${n * 4}px`,
  shadow: {
    sm: '0 1px 2px rgba(16, 24, 40, 0.06)',
    md: '0 4px 16px rgba(16, 24, 40, 0.08)',
  },
  font: {
    body: "'Inter', system-ui, -apple-system, 'Segoe UI', Roboto, sans-serif",
    mono: "'JetBrains Mono', ui-monospace, SFMono-Regular, Menlo, monospace",
  },
} as const;

export type Theme = typeof theme;

/** Semantic tones used by badges and alerts. */
export type Tone = 'success' | 'warning' | 'danger' | 'info' | 'neutral';

/** Resolves a tone to its foreground and background colours. */
export function toneColors(t: Theme, tone: Tone): { fg: string; bg: string } {
  switch (tone) {
    case 'success':
      return { fg: t.colors.success, bg: t.colors.successBg };
    case 'warning':
      return { fg: t.colors.warning, bg: t.colors.warningBg };
    case 'danger':
      return { fg: t.colors.danger, bg: t.colors.dangerBg };
    case 'info':
      return { fg: t.colors.info, bg: t.colors.infoBg };
    default:
      return { fg: t.colors.textMuted, bg: t.colors.neutralBg };
  }
}
