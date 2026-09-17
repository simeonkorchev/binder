import type { ResolvedScan, ScanMatchBody } from '../types'

/**
 * Shapes one `POST /scans/resolve` response into the scan the session holds.
 *
 * The card, the printing and the candidates are carried across as they arrive —
 * they are the generated shapes, and copying them field by field would only add
 * somewhere to forget one. One thing is added: the code the camera read, because
 * the resolver echoes nothing back and the review sheet has to show the user
 * what was read when the ladder resolved nothing at all.
 *
 * Deliberately not carried: `$schema`, the JSON-Schema URL huma adds to every
 * body, which describes the response rather than the card.
 */
export const toResolvedScan = (code: string, match: ScanMatchBody): ResolvedScan => ({
  code,
  resolution: match.resolution,
  card: match.card,
  printing: match.printing,
  candidates: match.candidates,
})
