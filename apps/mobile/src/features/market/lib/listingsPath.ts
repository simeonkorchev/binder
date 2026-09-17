/** What a buyer narrowed the feed to. Both parts blank is the whole market. */
export interface ListingFilter {
  /** Part of a card name, matched anywhere in it. */
  query: string
  /** A set prefix, e.g. `LOB`. A card whose set was never resolved is in no set, so a filtered feed cannot contain it. */
  setCode: string
}

/** The unfiltered feed: the newest listings, whatever they are. */
export const wholeMarket: ListingFilter = { query: '', setCode: '' }

/**
 * True when the buyer asked for something narrower than the whole market.
 *
 * The empty feed and the filtered-to-empty feed are the same response and two
 * different sentences, and this is what tells them apart.
 */
export const isNarrowed = (filter: ListingFilter): boolean =>
  filter.query.trim() !== '' || filter.setCode.trim() !== ''

/**
 * Builds the browse request for a filter.
 *
 * A blank part is left out rather than sent empty: `?q=` is a filter the server
 * would have to decide to ignore, and a request that says nothing about a
 * filter cannot be misread. Both parts are trimmed, because a trailing space
 * off a phone keyboard is not part of a card's name.
 */
export const listingsPath = (filter: ListingFilter): string => {
  const parts: string[] = []
  const query = filter.query.trim()
  const setCode = filter.setCode.trim()

  if (query !== '') parts.push(`q=${encodeURIComponent(query)}`)
  if (setCode !== '') parts.push(`set=${encodeURIComponent(setCode)}`)

  return parts.length === 0 ? '/listings' : `/listings?${parts.join('&')}`
}
