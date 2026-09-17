import { themes, type ThemeColors, type ThemeName } from './tokens'

// WCAG 2.1 relative luminance and contrast ratio, kept here rather than in the
// app: nothing the app ships needs to compute a ratio, only to honour one.
const channel = (value: number): number => {
  const srgb = value / 255
  return srgb <= 0.03928 ? srgb / 12.92 : ((srgb + 0.055) / 1.055) ** 2.4
}

const luminance = (hex: string): number => {
  const [red, green, blue] = [1, 3, 5].map((offset) => parseInt(hex.slice(offset, offset + 2), 16))
  return 0.2126 * channel(red ?? 0) + 0.7152 * channel(green ?? 0) + 0.0722 * channel(blue ?? 0)
}

const contrastRatio = (a: string, b: string): number => {
  const [lighter, darker] = [luminance(a), luminance(b)].sort((x, y) => y - x)
  return ((lighter ?? 0) + 0.05) / ((darker ?? 0) + 0.05)
}

type Pairing = [foreground: keyof ThemeColors, background: keyof ThemeColors, minimumRatio: number]

// 4.5:1 is the AA floor for body text (003-frontend.md §10). `accent` is a fill
// and a border rather than body text, so it is held to the 3:1 non-text floor
// against the surfaces it is painted on.
const TEXT_PAIRINGS: readonly Pairing[] = [
  ['textPrimary', 'background', 4.5],
  ['textPrimary', 'surface', 4.5],
  ['textPrimary', 'surfaceMuted', 4.5],
  ['textSecondary', 'background', 4.5],
  ['textSecondary', 'surface', 4.5],
  ['textSecondary', 'surfaceMuted', 4.5],
  ['onAccent', 'accent', 4.5],
  ['error', 'background', 4.5],
  ['error', 'surface', 4.5],
  ['accent', 'background', 3],
  ['accent', 'surface', 3],
]

// Adjacent fills need to stay apart or an inset box dissolves into the page —
// contrast that no text pairing would ever catch.
const SURFACE_PAIRINGS: readonly Pairing[] = [
  ['surface', 'background', 1.1],
  ['surfaceMuted', 'surface', 1.1],
  ['border', 'surface', 1.3],
  ['border', 'background', 1.2],
]

const themeNames = Object.keys(themes) as ThemeName[]

describe('theme tokens', () => {
  it.each(themeNames)('%s declares every token as a six-digit hex value', (name) => {
    for (const value of Object.values(themes[name])) {
      expect(value).toMatch(/^#[0-9a-f]{6}$/)
    }
  })

  it.each(themeNames)('%s has the same token set as every other theme', (name) => {
    expect(Object.keys(themes[name]).sort()).toEqual(Object.keys(themes.light).sort())
  })

  describe.each(themeNames)('%s contrast', (name) => {
    const colors = themes[name]

    it.each(TEXT_PAIRINGS)('%s on %s clears %s:1', (foreground, background, minimum) => {
      expect(contrastRatio(colors[foreground], colors[background])).toBeGreaterThanOrEqual(minimum)
    })

    it.each(SURFACE_PAIRINGS)('%s stays separable from %s (%s:1)', (fill, beneath, minimum) => {
      expect(contrastRatio(colors[fill], colors[beneath])).toBeGreaterThanOrEqual(minimum)
    })
  })
})
