import type { components } from '@binder/types'

/**
 * The feature's re-export of the generated contract: `@binder/types` is
 * imported here and in the API hooks, never in a screen or a component, so a
 * regenerated schema has one blast radius (003-frontend.md §5).
 */

/** One binder as `GET /binders` lists it. */
export type Binder = components['schemas']['BinderBody']

/** `GET /binders` response body. */
export type BinderListBody = components['schemas']['ListBindersOutputBody']

/** One card in one pocket, as the server sends it. */
export type SlotBody = components['schemas']['SlotBody']

/** `GET /binders/{binderId}?page=n` response body: nine pockets and a page count. */
export type PageBody = components['schemas']['PageBody']

/**
 * How a card's set was determined when it was scanned — the rungs of the match
 * ladder. `by_name` and `unresolved` are the two that leave the set unknown,
 * which US3 says has to be shown rather than guessed.
 */
export type SetResolution = SlotBody['setResolution']

/** A card the database knows, as `GET /cards` returns it. */
export type CardSummary = components['schemas']['ScanCard']

/** `GET /cards` response body. */
export type CardSearchBody = components['schemas']['SearchCardsBody']

/**
 * One 3x3 page, shaped for the grid: always nine pockets, `null` where the
 * pocket is empty. The server sends nine; the mapper builds nine regardless, so
 * the grid never indexes past the end of a short array.
 */
export interface BinderPage {
  /** Zero-based, as the API numbers pages. */
  page: number
  /** How many pages the binder has. Zero for a binder with no cards at all. */
  pageCount: number
  pockets: (SlotBody | null)[]
}
