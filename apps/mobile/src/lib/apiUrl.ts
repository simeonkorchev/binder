/**
 * Where the API lives.
 *
 * Read per call rather than at module load: under Metro `EXPO_PUBLIC_*` is
 * inlined at build time, but a module that captures it at import time cannot be
 * configured by a test, and an unset value would be baked into a URL built out
 * of an empty string. Throwing instead means a developer sees the missing
 * configuration on the first request rather than a 404 from `/binders`.
 *
 * Every caller goes through here: the scan queue's own copy, written when it was
 * the only caller, was folded in when the review sheet became the third.
 */
export const apiUrl = (path: string): string => {
  const baseUrl = process.env.EXPO_PUBLIC_API_URL
  if (baseUrl === undefined || baseUrl === '') {
    throw new Error(`EXPO_PUBLIC_API_URL is not set, so ${path} cannot be requested`)
  }
  return `${baseUrl}${path}`
}
