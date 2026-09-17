import { useColorScheme } from 'react-native'

import { themes, type ThemeColors, type ThemeName } from './tokens'

export interface Theme {
  /** Which palette is active — for the status bar style and native surfaces. */
  name: ThemeName
  colors: ThemeColors
}

/**
 * The seam every screen reads colours through.
 *
 * It follows the device appearance and nothing else: there is no in-app theme
 * switch in the MVP, so there is no provider and no stored preference to read.
 * Adding one later changes this hook's body, not its callers — which is the
 * point of the seam.
 */
export const useTheme = (): Theme => {
  const name: ThemeName = useColorScheme() === 'dark' ? 'dark' : 'light'

  return { name, colors: themes[name] }
}
