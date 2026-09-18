import type { SetResolution } from '../types'

/**
 * The words for each rung of the match ladder.
 *
 * `satisfies Record<SetResolution, string>` is the point of the map: a rung
 * added in Go arrives in the generated enum and stops this file compiling,
 * rather than reaching a pocket as a blank line.
 */
export const resolutionLabelKeys = {
  exact: 'binder.resolution.exact',
  by_prefix_and_number: 'binder.resolution.byPrefixAndNumber',
  by_number: 'binder.resolution.byNumber',
  by_name: 'binder.resolution.byName',
  manual: 'binder.resolution.manual',
  unresolved: 'binder.resolution.unresolved',
} as const satisfies Record<SetResolution, string>

/**
 * True when the slot records the set the card was printed in.
 *
 * The two rungs that do not are what US3 says has to be shown rather than
 * guessed — and a pocket says so in words, never with a colour alone
 * (003-frontend.md §10). `manual` is on the other side of the line: the user
 * picked a printing in the review sheet, so there is a set, and it is the one
 * they read off the card (`db/migrations/005_manual_set_resolution.sql`).
 */
export const hasKnownSet = (
  resolution: SetResolution,
): resolution is Exclude<SetResolution, UnknownSetResolution> =>
  resolution !== 'by_name' && resolution !== 'unresolved'

/**
 * The two rungs that record no set. Named because it is the type the caller
 * gets when `hasKnownSet` is false, which is what lets a pocket index
 * `noSetNoteKeys` without a cast.
 */
export type UnknownSetResolution = Extract<SetResolution, 'by_name' | 'unresolved'>

/**
 * The sentence a pocket shows when it has no set to show.
 *
 * Keyed on the rung, not on `hasKnownSet`, because the two rungs need opposite
 * advice: a card added by name has never been scanned, and an `unresolved` one
 * already was. Both previously shared `binder.add.note` — the add sheet's own
 * sentence — which told the owner of a scanned card to go and scan it.
 *
 * `satisfies` over exactly the two rungs, so a rung that stops having a known
 * set stops this file compiling rather than reaching a pocket blank.
 */
export const noSetNoteKeys = {
  by_name: 'binder.actions.noSetByName',
  unresolved: 'binder.actions.noSetUnresolved',
} as const satisfies Record<UnknownSetResolution, string>
