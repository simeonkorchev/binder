import type { components } from '@binder/types'

/**
 * The feature's re-export of the generated contract: `@binder/types` is
 * imported here and in the API hooks, never in a screen or a component, so a
 * regenerated schema has one blast radius (003-frontend.md §5).
 */

/** One card for sale, as the browse feed lists it. */
export type ListedCard = components['schemas']['ListedCardBody']

/** `GET /listings` response body. The list is empty, never null, when nothing matches. */
export type BrowseListingsBody = components['schemas']['BrowseListingsBody']

/** `POST /listings` answers with the listing it made — its id is how it is taken down again. */
export type ListingBody = components['schemas']['ListingBody']

/**
 * `GET /sellers/{sellerId}/contact`. **Both fields null is a success**: a seller
 * who opted into publishing neither is the ordinary case, not a 404 and not an
 * error (D4).
 */
export type SellerContactBody = components['schemas']['SellerContactBody']
