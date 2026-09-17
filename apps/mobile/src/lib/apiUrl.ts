/**
 * Where the API lives.
 *
 * Read per call rather than at module load: under Metro `EXPO_PUBLIC_*` is
 * inlined at build time, but a module that captures it at import time cannot be
 * configured by a test, and an unset value would be baked into a URL built out
 * of an empty string. Throwing instead means a developer sees the missing
 * configuration on the first request rather than a 404 from `/binders`.
 *
 * `features/scan/api/useResolveScan.ts` still carries its own copy of this,
 * written when the scan queue was the only caller. It folds into this one the
 * next time that file is opened — it was left alone here only because W9 was
 * being finished in parallel.
 */
export const apiUrl = (path: string): string => {
  const baseUrl = process.env.EXPO_PUBLIC_API_URL
  if (baseUrl === undefined || baseUrl === '') {
    throw new Error(`EXPO_PUBLIC_API_URL is not set, so ${path} cannot be requested`)
  }
  return `${baseUrl}${path}`
}
