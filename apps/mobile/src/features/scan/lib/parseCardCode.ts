/**
 * Glyphs ML Kit reads in place of the hyphen in a printed code — the ASCII
 * hyphen itself, the Unicode dashes and the minus sign.
 */
const dashLikeGlyphs = /[-‐‑‒–—―−]/gu

/**
 * Letters ML Kit reads in place of a digit. Only the digit half of a code is
 * repaired with these: the set prefix legitimately mixes letters and digits
 * (`MP21`, `15AX`), so "fixing" a character there would corrupt real codes.
 */
const digitLookAlikes: Readonly<Record<string, string>> = {
  O: '0',
  I: '1',
  L: '1',
  S: '5',
  B: '8',
}

/**
 * A printed code is `{PREFIX}-{REGION}{SERIAL}`: `LOB-EN001`, `MP21-EN123`,
 * and — before region letters were printed — `SDK-001`.
 *
 * The prefix must carry at least one letter, which is what keeps a printed
 * year range off the candidate list.
 */
const printedCodeShape = /^(?=[A-Z0-9]*[A-Z])([A-Z0-9]{2,6})-([A-Z]{0,3})([A-Z0-9]{3,5})$/u

const repairedSerial = (serial: string): string =>
  [...serial].map((character) => digitLookAlikes[character] ?? character).join('')

const parseToken = (token: string): string | null => {
  const trimmed = token
    .toUpperCase()
    .replace(dashLikeGlyphs, '-')
    .replace(/^[^A-Z0-9]+/u, '')
    .replace(/[^A-Z0-9]+$/u, '')

  const shape = printedCodeShape.exec(trimmed)
  if (shape === null) return null

  const [, prefix, region, serial] = shape
  if (prefix === undefined || region === undefined || serial === undefined) return null

  const digits = repairedSerial(serial)
  if (!/^[0-9]{3,5}$/u.test(digits)) return null

  return `${prefix}-${region}${digits}`
}

/**
 * Extracts the card code printed below the artwork from a block of OCR text,
 * or `null` when no token in it has the shape of one.
 *
 * This **finds a candidate, it does not match one**: the match ladder lives on
 * the server (`POST /scans/resolve`), which upper-cases and splits the code
 * itself and deliberately does not guess at character confusions, because its
 * lower rungs — number plus prefix, number alone, name — are what rescue a
 * misread. The only repair made here is the one the server cannot make: a
 * letter standing where the code's serial can only hold a digit.
 *
 * The separator has to be a hyphen (or a Unicode dash). Any other punctuation
 * disqualifies the token, because the stat line every monster card carries —
 * `ATK/2500` — is otherwise a perfect code shape and would be sent as one.
 */
export const parseCardCode = (ocrText: string): string | null => {
  for (const token of ocrText.split(/\s+/u)) {
    const code = parseToken(token)
    if (code !== null) return code
  }
  return null
}
