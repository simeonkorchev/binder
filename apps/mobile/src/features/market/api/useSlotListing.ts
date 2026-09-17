import { useState } from 'react'

import { ApiError, writeEmpty, writeJson } from '@/lib/apiRequest'

import type { ListingBody } from '../types'

/**
 * What `POST /listings` answers when the card is already for sale. The server
 * refuses the second listing rather than making a duplicate, and a seller is
 * owed the sentence rather than the status.
 */
const alreadyForSale = 409

/** Where one pocket's card stands with the marketplace, as far as this sheet knows. */
export type SaleStatus =
  | 'idle'
  | 'saving'
  | 'listed'
  | 'alreadyListed'
  | 'unlisted'
  | 'listFailed'
  | 'unlistFailed'

export interface SlotListing {
  status: SaleStatus
  /**
   * True once the listing's id is known, which is the only thing that can take
   * it down again.
   */
  canUnlist: boolean
  list: () => Promise<void>
  unlist: () => Promise<void>
}

/**
 * Putting one card in a pocket up for sale, and taking it down again.
 *
 * Every failure gets its own sentence, and the 409 is not a failure at all: it
 * is the server saying the card was already listed, which is a state to report
 * rather than an error to apologise for.
 *
 * **A 404 is deliberately indistinguishable from any other failure here.** The
 * server answers 404 — not 403 — for a slot belonging to somebody else,
 * precisely so a stranger cannot learn that it exists; a client that then said
 * "that card is not yours" would hand back the fact the status code was chosen
 * to withhold.
 *
 * Unlisting is offered for a listing this sheet made. Nothing in the contract
 * maps a binder slot back to its listing — `SlotBody` carries no listing id and
 * the browse feed carries no slot id — so a card listed in an earlier session
 * answers 409 and says so, which is the truth the client actually has.
 */
export const useSlotListing = (slotId: string): SlotListing => {
  const [status, setStatus] = useState<SaleStatus>('idle')
  const [listingId, setListingId] = useState<string | null>(null)

  return {
    status,
    canUnlist: listingId !== null,

    list: async (): Promise<void> => {
      setStatus('saving')
      try {
        const listing = await writeJson<ListingBody>('POST', '/listings', { binderSlotId: slotId })
        setListingId(listing.id)
        setStatus('listed')
      } catch (error) {
        setStatus(
          error instanceof ApiError && error.status === alreadyForSale
            ? 'alreadyListed'
            : 'listFailed',
        )
      }
    },

    unlist: async (): Promise<void> => {
      if (listingId === null) return
      setStatus('saving')
      try {
        await writeEmpty('DELETE', `/listings/${encodeURIComponent(listingId)}`)
        setListingId(null)
        setStatus('unlisted')
      } catch {
        setStatus('unlistFailed')
      }
    },
  }
}
