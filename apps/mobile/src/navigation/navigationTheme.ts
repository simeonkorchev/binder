import { DarkTheme, DefaultTheme, type Theme as NavigationTheme } from '@react-navigation/native'

import type { Theme } from '@/theme/useTheme'

/**
 * Maps the app palette onto React Navigation's own theme, so the chrome the
 * library draws — headers, tab bar, card backgrounds — comes from the same
 * tokens the screens use. Without this the header stays white in dark mode.
 *
 * Only `fonts` is inherited from the library's base theme; every colour is
 * ours, and `navigationTheme.test.ts` asserts that for every key.
 */
export const navigationTheme = (theme: Theme): NavigationTheme => {
  const isDark = theme.name === 'dark'

  return {
    ...(isDark ? DarkTheme : DefaultTheme),
    dark: isDark,
    colors: {
      primary: theme.colors.accent,
      background: theme.colors.background,
      card: theme.colors.surface,
      text: theme.colors.textPrimary,
      border: theme.colors.border,
      notification: theme.colors.error,
    },
  }
}
