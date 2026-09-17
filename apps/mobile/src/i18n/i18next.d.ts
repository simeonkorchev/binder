import type en from './locales/en.json'

// English is the key authority: `t('nav.scan')` type-checks, `t('nav.scna')`
// does not compile. `locales.test.ts` is what holds Bulgarian to the same set,
// because a missing translation must fail the gate rather than ship as a key.
declare module 'i18next' {
  interface CustomTypeOptions {
    defaultNS: 'translation'
    resources: { translation: typeof en }
  }
}
