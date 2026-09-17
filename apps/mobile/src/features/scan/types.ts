import type { components } from '@binder/types'

/**
 * The feature's re-export of the generated contract: `@binder/types` is
 * imported here and in the API hooks, never in a screen or a component, so a
 * regenerated schema has one blast radius (003-frontend.md §5).
 *
 * A piece is exported here once something outside this file names it. The set
 * resolution is still reachable through `ResolvedScan` alone, so it stays
 * private.
 */

/** A card as the resolver returns it. */
export type ScannedCard = components['schemas']['ScanCard']

/** One card as printed in one set. */
export type ScannedPrinting = components['schemas']['ScanPrinting']

/**
 * How the scanned card's set was determined — the rungs of the match ladder,
 * `unresolved` being the bottom one that US3 says must be flagged rather than
 * guessed. Aliased from the generated enum so a rung added in Go cannot drift
 * out of sync with a union re-typed by hand.
 */
type SetResolution = components['schemas']['ScanMatch']['resolution']

/**
 * A card the ladder could not rule out. `printing` is null for a candidate the
 * name rung produced, which names a card and no set — and the generated type
 * says so.
 */
export type ScanCandidate = components['schemas']['ScanCandidate']

/** `POST /scans/resolve`'s response body. */
export type ScanMatchBody = components['schemas']['ScanMatch']

/** `GET /cards`'s response body: the cards a name search found. */
export type SearchCardsBody = components['schemas']['SearchCardsBody']

/** One scanned card, resolved: what the camera read and what the ladder made of it. */
export interface ResolvedScan {
  /** The printed code the scanner read, as it was sent to the resolver. */
  code: string
  resolution: SetResolution
  card: ScannedCard | null
  printing: ScannedPrinting | null
  /** Always a list: an unambiguous match carries an empty one, never null. */
  candidates: ScanCandidate[]
}

/**
 * A scan the resolver refused. The status is kept because it is the whole
 * difference between "this scan is wrong" and "the server is" — and because
 * dropping the code would lose the card the user swept past.
 */
export interface RejectedScan {
  code: string
  status: number
}
