export type Language = 'en' | 'bg'

export const FALLBACK_LANGUAGE: Language = 'en'

export const LANGUAGES: readonly Language[] = ['en', 'bg']

const isSupported = (code: string): code is Language => LANGUAGES.includes(code as Language)

/**
 * Picks the app language from the device's ordered preference list.
 *
 * The device reports a whole list ("bg, then en"), and a code can be a region
 * tag (`bg-BG`) or absent altogether on a device that never set one — so the
 * first entry is a guess, not an answer. Take the first preference the app
 * actually speaks, and English when it speaks none of them.
 */
export const resolveLanguage = (deviceLanguageCodes: readonly (string | null)[]): Language => {
  for (const code of deviceLanguageCodes) {
    const base = code?.split('-')[0]?.toLowerCase()
    if (base !== undefined && isSupported(base)) return base
  }

  return FALLBACK_LANGUAGE
}
