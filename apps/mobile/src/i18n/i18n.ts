import { getLocales } from 'expo-localization'
import { init, use as registerPlugin } from 'i18next'
import { initReactI18next } from 'react-i18next'

import { catalogues } from './catalogues'
import { FALLBACK_LANGUAGE, LANGUAGES, resolveLanguage } from './resolveLanguage'

// Imported for its side effect (`@/i18n/i18n` in App.tsx): i18next is a
// singleton, and `useTranslation` in any screen reads this one instance.
//
// `use` is aliased because React 19 has a hook of that name, so
// `react-hooks/rules-of-hooks` reads a top-level `use(...)` as a hook called
// outside a component. The alias is the fix; a lint suppression would not be.
registerPlugin(initReactI18next)

void init({
  // Built from the catalogue map rather than listed again here, so a language
  // cannot be offered in `LANGUAGES` yet silently left unregistered.
  resources: Object.fromEntries(
    LANGUAGES.map((language) => [language, { translation: catalogues[language] }]),
  ),
  lng: resolveLanguage(getLocales().map((locale) => locale.languageCode)),
  fallbackLng: FALLBACK_LANGUAGE,
  supportedLngs: LANGUAGES,
  // React escapes everything it renders; i18next escaping on top of that turns
  // an apostrophe into `&#39;` on screen.
  interpolation: { escapeValue: false },
})
