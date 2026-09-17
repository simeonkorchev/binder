import { catalogues, type Catalogue } from './catalogues'
import en from './locales/en.json'
import { LANGUAGES, resolveLanguage } from './resolveLanguage'

const flatten = (catalogue: Catalogue, prefix = ''): Record<string, string> => {
  const flat: Record<string, string> = {}

  for (const [key, value] of Object.entries(catalogue)) {
    const path = prefix === '' ? key : `${prefix}.${key}`
    if (typeof value === 'string') {
      flat[path] = value
      continue
    }
    Object.assign(flat, flatten(value, path))
  }

  return flat
}

describe('locale catalogues', () => {
  // A key present in one locale and missing from the other ships as the raw key
  // on screen — `nav.market` where a word belongs. This is the test that turns
  // that into a red build. English is the authority: `i18next.d.ts` types
  // `t()` against it, so a key it does not have will not compile.
  it.each(LANGUAGES)('%s has exactly the keys English has', (language) => {
    expect(Object.keys(flatten(catalogues[language])).sort()).toEqual(Object.keys(flatten(en)).sort())
  })

  it.each(LANGUAGES)('%s translates every key to a non-empty string', (language) => {
    const blank = Object.entries(flatten(catalogues[language]))
      .filter(([, value]) => value.trim() === '')
      .map(([key]) => key)

    expect(blank).toEqual([])
  })
})

describe('resolveLanguage', () => {
  it('takes the first device preference the app speaks', () => {
    expect(resolveLanguage(['de', 'bg', 'en'])).toBe('bg')
  })

  it('ignores the region tag', () => {
    expect(resolveLanguage(['bg-BG'])).toBe('bg')
  })

  it('falls back to English when the device speaks nothing the app does', () => {
    expect(resolveLanguage(['de', 'fr'])).toBe('en')
  })

  it('falls back to English when the device reports no language at all', () => {
    expect(resolveLanguage([null])).toBe('en')
  })

  it('falls back to English for an empty preference list', () => {
    expect(resolveLanguage([])).toBe('en')
  })
})
