import { useEffect, useState } from 'react'

import { readJson } from '@/lib/apiRequest'

import { listingsPath, wholeMarket, type ListingFilter } from '../lib/listingsPath'
import type { BrowseListingsBody, ListedCard } from '../types'

/**
 * Three states, never two: a feed that failed to load is not a market with
 * nothing in it, and the screen has to be able to tell a buyer which one they
 * are looking at (003-frontend.md §11).
 *
 * The ready state carries the filter it was read with. An empty list means
 * "nothing is for sale" or "nothing matched you" depending on it, and the
 * answer alone cannot say which.
 */
export type ListingsState =
  | { status: 'loading' }
  | { status: 'error' }
  | { status: 'ready'; listings: ListedCard[]; filter: ListingFilter }

export interface ListingsBrowse {
  state: ListingsState
  /** Reads the feed again through a filter. The whole market is a filter with nothing in it. */
  browse: (filter: ListingFilter) => void
  /** Reads the current filter again — the way out of the error state. */
  reload: () => void
}

/** One read: which filter, and which attempt, so two reads of the same filter are two requests. */
interface ListingsRequest {
  filter: ListingFilter
  attempt: number
}

/**
 * What is for sale: the newest listings, narrowed by name and set.
 *
 * The feed carries no contact details by design — a seller is a `sellerId` here
 * and nothing more, so scrolling cannot collect other people's email addresses.
 * Revealing one is a separate, deliberate request on the seller's own screen.
 */
export const useListings = (): ListingsBrowse => {
  const [request, setRequest] = useState<ListingsRequest>({ filter: wholeMarket, attempt: 0 })
  // What came back, beside which request it answers. `loading` is then derived
  // from the two disagreeing rather than written into state as the request goes
  // out (003-frontend.md §1c).
  const [answered, setAnswered] = useState<{ attempt: number; state: ListingsState } | null>(null)

  useEffect(() => {
    let cancelled = false
    const { filter, attempt } = request

    readJson<BrowseListingsBody>(listingsPath(filter)).then(
      (body) => {
        if (!cancelled) {
          setAnswered({ attempt, state: { status: 'ready', listings: body.listings, filter } })
        }
      },
      // Surfaced, never swallowed: a market that could not be reached must not
      // look like a market nobody is selling in.
      () => {
        if (!cancelled) setAnswered({ attempt, state: { status: 'error' } })
      },
    )

    return () => {
      cancelled = true
    }
  }, [request])

  return {
    state: answered?.attempt === request.attempt ? answered.state : { status: 'loading' },
    browse: (filter: ListingFilter): void =>
      setRequest((current) => ({ filter, attempt: current.attempt + 1 })),
    reload: (): void => setRequest((current) => ({ ...current, attempt: current.attempt + 1 })),
  }
}
