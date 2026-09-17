import { useEffect, useState } from 'react'

import type { Binder, BinderListBody } from '../types'

import { readJson, writeJson } from './binderRequest'

type BinderListState =
  | { status: 'loading' }
  | { status: 'error' }
  | { status: 'ready'; binders: Binder[] }

export interface BinderList {
  state: BinderListState
  /** Reads the list again — the way out of the error state, and what a create does on its own. */
  reload: () => void
  /** Creates a binder and answers with it, or null when the write failed. */
  createBinder: (name: string) => Promise<Binder | null>
  isCreating: boolean
  /** True after a create that did not land. Cleared by the next attempt. */
  createFailed: boolean
}

/**
 * The binders the signed-in collector owns, and the one write the list offers.
 *
 * A successful create re-reads the list rather than appending locally: the
 * server owns the order and the timestamps, and a list assembled from two
 * sources is a list that can disagree with itself.
 */
export const useBinders = (): BinderList => {
  const [reloadCount, setReloadCount] = useState(0)
  const [isCreating, setIsCreating] = useState(false)
  const [createFailed, setCreateFailed] = useState(false)
  // Which read the answer belongs to, so `loading` is derived from the answer
  // being older than the request rather than written by the effect that starts
  // it (003-frontend.md §1c).
  const [answered, setAnswered] = useState<{ request: number; state: BinderListState } | null>(null)

  useEffect(() => {
    let cancelled = false

    readJson<BinderListBody>('/binders').then(
      (body) => {
        if (!cancelled) setAnswered({ request: reloadCount, state: { status: 'ready', binders: body.binders } })
      },
      // Surfaced, never swallowed: "no binders yet" and "we could not ask" are
      // different things to tell a collector (003-frontend.md §11).
      () => {
        if (!cancelled) setAnswered({ request: reloadCount, state: { status: 'error' } })
      },
    )

    return () => {
      cancelled = true
    }
  }, [reloadCount])

  const createBinder = async (name: string): Promise<Binder | null> => {
    setIsCreating(true)
    setCreateFailed(false)
    try {
      const binder = await writeJson<Binder>('POST', '/binders', { name })
      setReloadCount((count) => count + 1)
      return binder
    } catch {
      setCreateFailed(true)
      return null
    } finally {
      setIsCreating(false)
    }
  }

  return {
    state: answered?.request === reloadCount ? answered.state : { status: 'loading' },
    reload: (): void => setReloadCount((count) => count + 1),
    createBinder,
    isCreating,
    createFailed,
  }
}
