import { act, renderHook } from '@testing-library/react-native'

import type { SlotBody } from '../types'

import { useBinderSlots } from './useBinderSlots'

const mockFetch: jest.MockedFunction<typeof fetch> = jest.fn()
globalThis.fetch = mockFetch

const addedSlot: SlotBody = {
  id: 'slot-9',
  cardId: 'card-1',
  cardPrintingId: null,
  page: 1,
  position: 9,
  slotOnPage: 0,
  setResolution: 'by_name',
}

const noContent = (): Response => new Response(null, { status: 204 })

describe('useBinderSlots', () => {
  beforeEach(() => {
    mockFetch.mockReset()
    process.env.EXPO_PUBLIC_API_URL = 'https://api.binder.test'
  })

  afterEach(() => {
    delete process.env.EXPO_PUBLIC_API_URL
  })

  describe('moveCard', () => {
    it('reorders with one request, whatever the distance', async () => {
      mockFetch.mockImplementation(() => Promise.resolve(noContent()))

      const { result } = await renderHook(() => useBinderSlots('binder-1'))

      let moved = false
      await act(async () => {
        moved = await result.current.moveCard('slot-3', 0)
      })

      expect(moved).toBe(true)
      expect(mockFetch).toHaveBeenCalledTimes(1)
      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.binder.test/binders/binder-1/slots',
        expect.objectContaining({
          method: 'PATCH',
          body: JSON.stringify({ slotId: 'slot-3', toPosition: 0 }),
        }),
      )
    })

    // The page boundary, end to end: a card dragged off the right edge of
    // page 0 asks for the first position of page 1, and the whole crossing is
    // still one call — the cards it displaces are the server's to shift.
    it('crosses a page boundary in the same single request', async () => {
      mockFetch.mockImplementation(() => Promise.resolve(noContent()))

      const { result } = await renderHook(() => useBinderSlots('binder-1'))

      await act(async () => {
        await result.current.moveCard('slot-3', 9)
      })

      expect(mockFetch).toHaveBeenCalledTimes(1)
      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.binder.test/binders/binder-1/slots',
        expect.objectContaining({ body: JSON.stringify({ slotId: 'slot-3', toPosition: 9 }) }),
      )
    })

    it('reports a refused move without claiming it landed', async () => {
      mockFetch.mockImplementation(() => Promise.resolve(new Response(null, { status: 422 })))

      const { result } = await renderHook(() => useBinderSlots('binder-1'))

      let moved = true
      await act(async () => {
        moved = await result.current.moveCard('slot-3', 99)
      })

      expect(moved).toBe(false)
      expect(result.current.failedAction).toBe('move')
    })
  })

  describe('addCard', () => {
    it('appends a card matched by name, with no position and no printing', async () => {
      mockFetch.mockImplementation(() =>
        Promise.resolve(new Response(JSON.stringify(addedSlot), { status: 201 })),
      )

      const { result } = await renderHook(() => useBinderSlots('binder-1'))

      let added: SlotBody | null = null
      await act(async () => {
        added = await result.current.addCard('card-1')
      })

      expect(added).toEqual(addedSlot)
      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.binder.test/binders/binder-1/slots',
        expect.objectContaining({
          method: 'POST',
          body: JSON.stringify({ cardId: 'card-1', setResolution: 'by_name' }),
        }),
      )
    })

    it('answers with nothing when the card could not be added', async () => {
      mockFetch.mockImplementation(() => Promise.resolve(new Response(null, { status: 500 })))

      const { result } = await renderHook(() => useBinderSlots('binder-1'))

      let added: SlotBody | null = addedSlot
      await act(async () => {
        added = await result.current.addCard('card-1')
      })

      expect(added).toBeNull()
      expect(result.current.failedAction).toBe('add')
    })
  })

  describe('removeCard', () => {
    it('takes one card out by its slot id', async () => {
      mockFetch.mockImplementation(() => Promise.resolve(noContent()))

      const { result } = await renderHook(() => useBinderSlots('binder-1'))

      let removed = false
      await act(async () => {
        removed = await result.current.removeCard('slot-3')
      })

      expect(removed).toBe(true)
      expect(mockFetch).toHaveBeenCalledWith(
        'https://api.binder.test/binders/binder-1/slots/slot-3',
        expect.objectContaining({ method: 'DELETE' }),
      )
    })

    it('reports a removal that did not happen', async () => {
      mockFetch.mockImplementation(() => Promise.reject(new Error('offline')))

      const { result } = await renderHook(() => useBinderSlots('binder-1'))

      let removed = true
      await act(async () => {
        removed = await result.current.removeCard('slot-3')
      })

      expect(removed).toBe(false)
      expect(result.current.failedAction).toBe('remove')
    })
  })

  it('forgets a failure when it is dismissed', async () => {
    mockFetch.mockImplementation(() => Promise.reject(new Error('offline')))

    const { result } = await renderHook(() => useBinderSlots('binder-1'))
    await act(async () => {
      await result.current.removeCard('slot-3')
    })

    await act(async () => {
      result.current.dismissFailure()
    })

    expect(result.current.failedAction).toBeNull()
  })
})
