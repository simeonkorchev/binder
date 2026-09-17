import { useEffect, useState } from 'react'

import type { BinderPage, PageBody } from '../types'

import { readJson } from './binderRequest'
import { toBinderPage } from './toBinderPage'

/**
 * Three states, never two: a page that failed to load is not an empty binder,
 * and the grid has to be able to tell the user which one it is
 * (003-frontend.md §11).
 */
type BinderPageState =
  | { status: 'loading' }
  | { status: 'error' }
  | { status: 'ready'; page: BinderPage }

export interface BinderPageRead {
  state: BinderPageState
  /** Re-reads the page. Every write goes through here rather than patching state locally: the server owns density, so it is the only thing that knows where the cards ended up. */
  reload: () => void
}

/**
 * Reads one 3x3 page of a binder, and again whenever the page turns or a card
 * moves.
 *
 * A page past the end of the binder is a legitimate empty page, not an error —
 * the server says so — which is what lets the pager show the one page of a
 * binder with nothing in it yet.
 */
export const useBinderPage = (binderId: string, page: number): BinderPageRead => {
  const [reloadCount, setReloadCount] = useState(0)
  // What was asked for, beside what came back. `loading` is then derived from
  // the two disagreeing rather than written into state as the request goes
  // out — a page that has turned is loading by definition, and an effect that
  // says so again is a second render nobody needs.
  const request = `${binderId}|${page}|${reloadCount}`
  const [answered, setAnswered] = useState<{ request: string; state: BinderPageState } | null>(null)

  useEffect(() => {
    let cancelled = false

    readJson<PageBody>(`/binders/${encodeURIComponent(binderId)}?page=${page}`).then(
      (body) => {
        if (!cancelled) setAnswered({ request, state: { status: 'ready', page: toBinderPage(body) } })
      },
      // The failure is the screen's to render: a retry belongs to the user, and
      // a silent empty grid would tell them their binder lost its cards.
      () => {
        if (!cancelled) setAnswered({ request, state: { status: 'error' } })
      },
    )

    return () => {
      cancelled = true
    }
  }, [binderId, page, request])

  return {
    state: answered?.request === request ? answered.state : { status: 'loading' },
    reload: (): void => setReloadCount((count) => count + 1),
  }
}
