import type { components } from '@binder/types'

/**
 * The feature's re-export of the generated contract: `@binder/types` is
 * imported here and in the API hooks, never in a screen or a component, so a
 * regenerated schema has one blast radius (003-frontend.md §5).
 *
 * The card database belongs to no other feature. Both the scanner's review
 * sheet and the binder's add sheet search it, so the search and the two shapes
 * it speaks live here rather than under whichever screen happened to need them
 * first.
 */

/** A card the database knows, as `GET /cards` returns it. */
export type Card = components['schemas']['ScanCard']

/** `GET /cards`'s response body: the cards a name search found. */
export type SearchCardsBody = components['schemas']['SearchCardsBody']
