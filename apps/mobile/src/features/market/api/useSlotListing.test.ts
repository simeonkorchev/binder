import { act, renderHook } from '@testing-library/react-native'

import { useSlotListing, type SlotListing } from './useSlotListing'

const mockFetch: jest.MockedFunction<typeof fetch> = jest.fn()

const created = (id = 'listing-1'): Response =>
  new Response(
    JSON.stringify({
      id,
      binderSlotId: 'slot-1',
      createdAt: '2026-09-17T10:00:00Z',
      updatedAt: '2026-09-17T10:00:00Z',
    }),
    { status: 201 },
  )

const refused = (status: number): Response => new Response('{}', { status })

const renderListing = async (slotId = 'slot-1'): Promise<{ current: SlotListing }> => {
  const { result } = await renderHook(() => useSlotListing(slotId))
  return result
}

/** Lists the card and leaves the hook holding the listing it made. */
const listIt = async (listing: SlotListing): Promise<void> => {
  await act(async () => {
    await listing.list()
  })
}

describe('useSlotListing', () => {
  beforeEach(() => {
    mockFetch.mockReset()
    mockFetch.mockImplementation(() => Promise.resolve(created()))
    globalThis.fetch = mockFetch
    process.env.EXPO_PUBLIC_API_URL = 'https://api.binder.test'
  })

  afterEach(() => {
    delete process.env.EXPO_PUBLIC_API_URL
  })

  it('says nothing about a card nobody has offered for sale yet', async () => {
    const listing = await renderListing()

    expect(listing.current.status).toBe('idle')
    expect(listing.current.canUnlist).toBe(false)
    expect(mockFetch).not.toHaveBeenCalled()
  })

  it('offers the slot for sale, naming the slot and not the card', async () => {
    const listing = await renderListing('slot-7')

    await listIt(listing.current)

    expect(mockFetch).toHaveBeenCalledWith(
      'https://api.binder.test/listings',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ binderSlotId: 'slot-7' }),
      }),
    )
    expect(listing.current.status).toBe('listed')
  })

  // The 409 is the server refusing a duplicate listing, not a breakage. The
  // seller gets a sentence about the card's state, never a raw status.
  it('reads the 409 as "already for sale" rather than as a failure', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(refused(409)))
    const listing = await renderListing()

    await listIt(listing.current)

    expect(listing.current.status).toBe('alreadyListed')
    expect(listing.current.canUnlist).toBe(false)
  })

  // Somebody else's slot answers 404 rather than 403 so that a stranger cannot
  // learn it exists. The client must not undo that: a 404 is told exactly like
  // any other failure, and says nothing about whose card it is.
  it('tells a 404 no differently from any other failure, so it confirms nothing', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(refused(404)))
    const listing = await renderListing()
    await listIt(listing.current)
    const notYours = listing.current.status

    mockFetch.mockImplementation(() => Promise.resolve(refused(500)))
    const other = await renderListing()
    await listIt(other.current)

    expect(notYours).toBe('listFailed')
    expect(other.current.status).toBe('listFailed')
  })

  it('takes down the listing it made, by the id the server gave it', async () => {
    const listing = await renderListing()
    await listIt(listing.current)
    expect(listing.current.canUnlist).toBe(true)

    mockFetch.mockImplementation(() => Promise.resolve(new Response(null, { status: 204 })))
    await act(async () => {
      await listing.current.unlist()
    })

    expect(mockFetch).toHaveBeenLastCalledWith(
      'https://api.binder.test/listings/listing-1',
      expect.objectContaining({ method: 'DELETE' }),
    )
    expect(listing.current.status).toBe('unlisted')
    expect(listing.current.canUnlist).toBe(false)
  })

  it('says the take-down did not land, and keeps offering it', async () => {
    const listing = await renderListing()
    await listIt(listing.current)

    mockFetch.mockImplementation(() => Promise.resolve(refused(500)))
    await act(async () => {
      await listing.current.unlist()
    })

    expect(listing.current.status).toBe('unlistFailed')
    expect(listing.current.canUnlist).toBe(true)
  })

  it('cannot take down a listing whose id it never learned', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(refused(409)))
    const listing = await renderListing()
    await listIt(listing.current)

    await act(async () => {
      await listing.current.unlist()
    })

    expect(mockFetch).toHaveBeenCalledTimes(1)
    expect(listing.current.status).toBe('alreadyListed')
  })
})
