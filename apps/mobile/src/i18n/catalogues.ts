import type { Language } from './resolveLanguage'

import bg from './locales/bg.json'
import en from './locales/en.json'

export type Catalogue = { [key: string]: string | Catalogue }

/** Every language the app offers, keyed by the code i18next is initialised with. */
export const catalogues: Record<Language, Catalogue> = { en, bg }
