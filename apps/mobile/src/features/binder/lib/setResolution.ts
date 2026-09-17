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
  unresolved: 'binder.resolution.unresolved',
} as const satisfies Record<SetResolution, string>

/**
 * True when the slot records the set the card was printed in.
 *
 * The two rungs that do not are what US3 says has to be shown rather than
 * guessed — and a pocket says so in words, never with a colour alone
 * (003-frontend.md §10).
 */
export const hasKnownSet = (resolution: SetResolution): boolean =>
  resolution !== 'by_name' && resolution !== 'unresolved'
