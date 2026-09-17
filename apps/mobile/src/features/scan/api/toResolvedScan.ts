import type { ResolvedScan, ScanMatchBody } from '../types'

/**
 * Shapes one `POST /scans/resolve` response into the scan the session holds.
 *
 * The card and the printing are carried across as they arrive — they are the
 * generated shapes, and copying them field by field would only add somewhere to
 * forget one. Two things do change, and they are why this is a function and not
 * an inline spread in the queue's `catch`-free path:
 *
 * - the code the camera read is joined onto the answer, because the resolver
 *   echoes nothing back and the review sheet has to show the user what was read
 *   when the ladder resolved nothing at all;
 * - `candidates` becomes a list. A Go nil slice marshals to `null`, so the
 *   generated type admits it; the handler builds a non-nil slice precisely so a
 *   client can iterate without a null check, and this keeps that promise for the
 *   one release where the two disagree. It narrows a documented null, it does
 *   not hide a failure — a failed request never reaches this function.
 *
 * Deliberately not carried: `$schema`, the JSON-Schema URL huma adds to every
 * body, which describes the response rather than the card.
 */
export const toResolvedScan = (code: string, match: ScanMatchBody): ResolvedScan => ({
  code,
  resolution: match.resolution,
  card: match.card,
  printing: match.printing,
  candidates: match.candidates ?? [],
})
