import { act, renderHook, waitFor } from '@testing-library/react-native'

import type { SellerContactBody } from '../types'

import { useSellerContact, type SellerContactRead } from './useSellerContact'

const mockFetch: jest.MockedFunction<typeof fetch> = jest.fn()

const shared = (contact: SellerContactBody): Response =>
  new Response(JSON.stringify(contact), { status: 200 })

const renderContact = async (sellerId = 'seller-1'): Promise<{ current: SellerContactRead }> => {
  const { result } = await renderHook(() => useSellerContact(sellerId))
  await waitFor(() => {
    expect(result.current.state.status).not.toBe('loading')
  })
  return result
}

describe('useSellerContact', () => {
  beforeEach(() => {
    mockFetch.mockReset()
    mockFetch.mockImplementation(() => Promise.resolve(shared({ email: null, phone: null })))
    globalThis.fetch = mockFetch
    process.env.EXPO_PUBLIC_API_URL = 'https://api.binder.test'
  })

  afterEach(() => {
    delete process.env.EXPO_PUBLIC_API_URL
  })

  it('asks the seller the buyer picked, id and all', async () => {
    await renderContact('seller-7')

    expect(mockFetch).toHaveBeenCalledWith('https://api.binder.test/sellers/seller-7/contact')
  })

  it('reveals what the seller opted into sharing', async () => {
    mockFetch.mockImplementation(() =>
      Promise.resolve(shared({ email: 'seller@binder.test', phone: '0888123456' })),
    )

    const contact = await renderContact()

    expect(contact.current.state).toEqual({
      status: 'ready',
      methods: [
        { channel: 'email', value: 'seller@binder.test', url: 'mailto:seller@binder.test' },
        { channel: 'phone', value: '0888123456', url: 'tel:0888123456' },
      ],
    })
  })

  // Two nulls is a 200 and the ordinary state of an account that published
  // neither field. It must not read as a failure anywhere in the client.
  it('reads a seller who shared nothing as a successful answer with no methods', async () => {
    const contact = await renderContact()

    expect(contact.current.state).toEqual({ status: 'ready', methods: [] })
  })

  it('reports a failed read as a failure, which is not the same as sharing nothing', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(new Response('{}', { status: 500 })))

    const contact = await renderContact()

    expect(contact.current.state).toEqual({ status: 'error' })
  })

  it('asks again when the buyer retries', async () => {
    mockFetch.mockImplementation(() => Promise.resolve(new Response('{}', { status: 500 })))

    const contact = await renderContact()
    await act(async () => {
      contact.current.reload()
    })

    await waitFor(() => {
      expect(mockFetch).toHaveBeenCalledTimes(2)
    })
  })
})
