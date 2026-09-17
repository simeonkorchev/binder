import { useEffect, useState } from 'react'

import { ApiError, readJson } from '@/lib/apiRequest'

import { contactMethods, type ContactMethod } from '../lib/contactMethods'
import type { SellerContactBody } from '../types'

/**
 * Four states, and none of them collapses into another.
 *
 * `ready` with no methods is the seller who shared nothing — a successful
 * answer, and the one the screen must render as a sentence rather than as a
 * blank or a failure (D4). A read that did not come back is `error`, which is a
 * different thing to tell a buyer.
 *
 * `sign-in-required` is the visitor who came here from the public browse feed.
 * This is the one thing in the market that needs an account — a signed-in caller
 * is what separates a buyer asking about a card from a script reading every
 * seller in the database (internal/listing/api/seller.go) — and "sign in" is
 * something the visitor can act on, where "could not be loaded" is not (T082).
 */
type SellerContactState =
  | { status: 'loading' }
  | { status: 'error' }
  | { status: 'sign-in-required' }
  | { status: 'ready'; methods: ContactMethod[] }

export interface SellerContactRead {
  state: SellerContactState
  /** Asks again — the way out of the error state. */
  reload: () => void
}

/** Which of the two failures happened: no session, or no answer. */
const refusedAs = (failure: unknown): 'error' | 'sign-in-required' =>
  failure instanceof ApiError && failure.status === 401 ? 'sign-in-required' : 'error'

/**
 * How to reach one seller: only what they opted into publishing.
 *
 * This is the one request that reveals a person, and it is made because a buyer
 * asked for it rather than because a feed scrolled past them.
 */
export const useSellerContact = (sellerId: string): SellerContactRead => {
  const [attempt, setAttempt] = useState(0)
  const [answered, setAnswered] = useState<{ attempt: number; state: SellerContactState } | null>(
    null,
  )

  useEffect(() => {
    let cancelled = false

    readJson<SellerContactBody>(`/sellers/${encodeURIComponent(sellerId)}/contact`).then(
      (body) => {
        if (!cancelled) {
          setAnswered({ attempt, state: { status: 'ready', methods: contactMethods(body) } })
        }
      },
      (failure: unknown) => {
        if (!cancelled) setAnswered({ attempt, state: { status: refusedAs(failure) } })
      },
    )

    return () => {
      cancelled = true
    }
  }, [sellerId, attempt])

  return {
    state: answered?.attempt === attempt ? answered.state : { status: 'loading' },
    reload: (): void => setAttempt((current) => current + 1),
  }
}
