import { useEffect, useState } from 'react'

import { readJson } from '@/lib/apiRequest'

import { contactMethods, type ContactMethod } from '../lib/contactMethods'
import type { SellerContactBody } from '../types'

/**
 * Three states, never two.
 *
 * `ready` with no methods is the seller who shared nothing — a successful
 * answer, and the one the screen must render as a sentence rather than as a
 * blank or a failure (D4). A read that did not come back is `error`, which is a
 * different thing to tell a buyer.
 */
export type SellerContactState =
  | { status: 'loading' }
  | { status: 'error' }
  | { status: 'ready'; methods: ContactMethod[] }

export interface SellerContactRead {
  state: SellerContactState
  /** Asks again — the way out of the error state. */
  reload: () => void
}

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
      () => {
        if (!cancelled) setAnswered({ attempt, state: { status: 'error' } })
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
