import { themes, type ThemeName } from '@/theme/tokens'

import { navigationTheme } from './navigationTheme'

// Completeness: every colour React Navigation draws with is asserted, not just
// the one a change happens to touch (000-principles.md §9). A key left on the
// library default is a header that stays light in dark mode.
const EXPECTED_COLORS: Record<ThemeName, Record<string, string>> = {
  light: {
    primary: themes.light.accent,
    background: themes.light.background,
    card: themes.light.surface,
    text: themes.light.textPrimary,
    border: themes.light.border,
    notification: themes.light.error,
  },
  dark: {
    primary: themes.dark.accent,
    background: themes.dark.background,
    card: themes.dark.surface,
    text: themes.dark.textPrimary,
    border: themes.dark.border,
    notification: themes.dark.error,
  },
}

describe('navigationTheme', () => {
  it.each<ThemeName>(['light', 'dark'])('maps every %s colour from the tokens', (name) => {
    const mapped = navigationTheme({ name, colors: themes[name] })

    expect(mapped.colors).toEqual(EXPECTED_COLORS[name])
  })

  it('tells React Navigation which palette is active', () => {
    expect(navigationTheme({ name: 'dark', colors: themes.dark }).dark).toBe(true)
    expect(navigationTheme({ name: 'light', colors: themes.light }).dark).toBe(false)
  })

  it('keeps the library fonts', () => {
    expect(navigationTheme({ name: 'light', colors: themes.light }).fonts).toBeDefined()
  })
})
