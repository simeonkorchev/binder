import { useState } from 'react'

import type { SlotBody } from '../types'

import { writeEmpty, writeJson } from './binderRequest'

/** Which write failed, so the screen can say which one rather than "something". */
export type SlotAction = 'add' | 'move' | 'remove'

export interface BinderSlotWrites {
  /**
   * Puts a card at the end of the binder and answers with the pocket the server
   * chose, or null when the write failed. No position is sent: appending is the
   * only insert the binder's density allows without guessing at a count the
   * page cannot see.
   */
  addCard: (cardId: string) => Promise<SlotBody | null>
  /**
   * Moves one card to one position, in **one** request.
   *
   * The server takes every card between the old and the new position along, in
   * one transaction — which is why a reorder is never a loop over the pockets
   * that shifted. A loop would be N requests that can half-apply, and each one
   * would be racing the density the one before it just changed.
   */
  moveCard: (slotId: string, toPosition: number) => Promise<boolean>
  removeCard: (slotId: string) => Promise<boolean>
  isSaving: boolean
  /** The last write that did not land, until it is dismissed or retried. */
  failedAction: SlotAction | null
  dismissFailure: () => void
}

/** The three writes a binder page makes. Reading it back is `useBinderPage`. */
export const useBinderSlots = (binderId: string): BinderSlotWrites => {
  const [isSaving, setIsSaving] = useState(false)
  const [failedAction, setFailedAction] = useState<SlotAction | null>(null)
  const slotsPath = `/binders/${encodeURIComponent(binderId)}/slots`

  const write = async <T>(action: SlotAction, request: () => Promise<T>): Promise<T | null> => {
    setIsSaving(true)
    setFailedAction(null)
    try {
      return await request()
    } catch {
      setFailedAction(action)
      return null
    } finally {
      setIsSaving(false)
    }
  }

  return {
    addCard: (cardId: string): Promise<SlotBody | null> =>
      write('add', () =>
        writeJson<SlotBody>('POST', slotsPath, {
          cardId,
          // A card chosen from a name search names no set, and the server
          // refuses a printing alongside a resolution that did not find one.
          setResolution: 'by_name',
        }),
      ),

    moveCard: async (slotId: string, toPosition: number): Promise<boolean> =>
      (await write('move', async () => {
        await writeEmpty('PATCH', slotsPath, { slotId, toPosition })
        return true
      })) === true,

    removeCard: async (slotId: string): Promise<boolean> =>
      (await write('remove', async () => {
        await writeEmpty('DELETE', `${slotsPath}/${encodeURIComponent(slotId)}`)
        return true
      })) === true,

    isSaving,
    failedAction,
    dismissFailure: (): void => setFailedAction(null),
  }
}
