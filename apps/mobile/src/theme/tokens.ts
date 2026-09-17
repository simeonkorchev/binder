// The palette is a contract, not a decoration: every token names the surface it
// sits on, so a screen that picks the token for what it is painting is legible
// in both themes by construction. Using a token on a different surface class is
// a bug even when it compiles and looks fine in one theme
// (003-frontend.md, "Theme tokens").
//
// The pairings are enforced — `tokens.test.ts` holds the WCAG contrast matrix
// and the separation floor between adjacent fills. A palette tweak that breaks a
// pairing is resolved in the palette, never by deleting the assertion; a new
// token is added together with its pairings.

export type ThemeName = 'light' | 'dark'

export interface ThemeColors {
  /** Screen background. */
  background: string
  /** Cards, rows, sheets — anything raised off the screen background. */
  surface: string
  /** Inset areas on a surface: an empty binder slot, a search field. */
  surfaceMuted: string
  /** Card and slot borders, dividers. */
  border: string
  /** Body and heading text on any `background`, `surface` or `surfaceMuted`. */
  textPrimary: string
  /** Supporting text: set names, counts, captions. */
  textSecondary: string
  /** Primary action fills and borders — the one CTA per screen. */
  accent: string
  /** Text and icons **only** on an `accent` fill. */
  onAccent: string
  /** Failure text and borders. */
  error: string
}

export const themes: Record<ThemeName, ThemeColors> = {
  light: {
    background: '#f6f3ee',
    surface: '#ffffff',
    surfaceMuted: '#e7e2d9',
    border: '#cfc8bc',
    textPrimary: '#1b1614',
    textSecondary: '#5c544c',
    accent: '#8a5a12',
    onAccent: '#ffffff',
    error: '#a3201b',
  },
  dark: {
    background: '#141211',
    surface: '#1f1c1a',
    surfaceMuted: '#2b2724',
    border: '#3b3633',
    textPrimary: '#f5f1ec',
    textSecondary: '#b3a99e',
    accent: '#e0b062',
    onAccent: '#1b1614',
    error: '#f08d84',
  },
}
