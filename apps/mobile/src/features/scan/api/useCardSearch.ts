import { useRef, useState } from 'react'

import type { ScannedCard, SearchCardsBody } from '../types'

/**
 * Where the API lives. Read per request for the same reason `useResolveScan`
 * reads it per request: a missing value must be an error a developer sees, not
 * a URL built out of an empty string. The two hooks will share one client when
 * a third screen needs one — two endpoints do not make a data layer.
 */
const searchCardsUrl = (query: string): string => {
  const baseUrl = process.env.EXPO_PUBLIC_API_URL
  if (baseUrl === undefined || baseUrl === '') {
    throw new Error('EXPO_PUBLIC_API_URL is not set, so cards cannot be searched')
  }
  return `${baseUrl}/cards?q=${encodeURIComponent(query)}`
}

export interface CardSearch {
  /** What the last finished search found, in the order the server ranked them. */
  results: ScannedCard[]
  /** True from the moment a search is asked for until its answer lands. */
  isSearching: boolean
  /** True when the last search did not come back. The user can simply ask again. */
  hasFailed: boolean
  /** True once a search has finished, so "nothing found" can be told from "not asked yet". */
  hasSearched: boolean
  /** Runs one search. A blank query is ignored — it would ask for the whole database. */
  search: (query: string) => void
}

/**
 * Finds a card by name, for the review sheet's correction step.
 *
 * It searches when it is asked to and not while the user types. A debounce
 * would mean a timer deciding when state changes, and on a phone it would also
 * mean a request per keystroke on a connection that just failed to resolve a
 * scan; a submit is one intent, one request, and one thing to assert in a test.
 *
 * Only the newest search may write results. Two searches in flight can land in
 * either order, and the older one landing last would silently replace the
 * user's current question with the answer to their previous one.
 */
export const useCardSearch = (): CardSearch => {
  const latestSearch = useRef(0)
  const [results, setResults] = useState<ScannedCard[]>([])
  const [isSearching, setIsSearching] = useState(false)
  const [hasFailed, setHasFailed] = useState(false)
  const [hasSearched, setHasSearched] = useState(false)

  const run = async (query: string, searchId: number): Promise<void> => {
    try {
      const response = await fetch(searchCardsUrl(query))
      if (!response.ok) throw new Error(`the card search answered ${response.status}`)
      const body: SearchCardsBody = await response.json()
      if (latestSearch.current !== searchId) return
      setResults(body.cards)
    } catch {
      if (latestSearch.current !== searchId) return
      setResults([])
      setHasFailed(true)
    } finally {
      if (latestSearch.current === searchId) {
        setIsSearching(false)
        setHasSearched(true)
      }
    }
  }

  return {
    results,
    isSearching,
    hasFailed,
    hasSearched,
    search: (query: string): void => {
      if (query.trim() === '') return
      latestSearch.current += 1
      setIsSearching(true)
      setHasFailed(false)
      void run(query.trim(), latestSearch.current)
    },
  }
}
