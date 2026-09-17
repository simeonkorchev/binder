import type { PageBody, SlotBody } from '../types'

import { toBinderPage } from './toBinderPage'

const slot = (over: Partial<SlotBody> = {}): SlotBody => ({
  id: 'slot-1',
  cardId: 'card-1',
  cardPrintingId: 'printing-1',
  page: 1,
  position: 9,
  slotOnPage: 0,
  setResolution: 'exact',
  ...over,
})

describe('toBinderPage', () => {
  it('carries every field of the body and of every slot across', () => {
    const first = slot()
    const last = slot({
      id: 'slot-2',
      cardId: 'card-2',
      cardPrintingId: null,
      position: 17,
      slotOnPage: 8,
      setResolution: 'unresolved',
    })

    const page = toBinderPage({
      page: 1,
      pageCount: 4,
      slots: [first, null, null, null, null, null, null, null, last],
    })

    expect(page).toEqual({
      page: 1,
      pageCount: 4,
      pockets: [first, null, null, null, null, null, null, null, last],
    })
    expect(page.pockets[0]).toEqual({
      id: 'slot-1',
      cardId: 'card-1',
      cardPrintingId: 'printing-1',
      page: 1,
      position: 9,
      slotOnPage: 0,
      setResolution: 'exact',
    })
  })

  it('draws nine empty pockets for the page of a binder with no cards', () => {
    const page = toBinderPage({ page: 0, pageCount: 0, slots: [null, null, null, null, null, null, null, null, null] })

    expect(page.pockets).toHaveLength(9)
    expect(page.pockets.every((pocket) => pocket === null)).toBe(true)
  })

  it('always hands the grid nine pockets, however many the body carried', () => {
    const page = toBinderPage({ page: 0, pageCount: 1, slots: [slot({ page: 0, position: 0 })] })

    expect(page.pockets).toHaveLength(9)
    expect(page.pockets.slice(1)).toEqual([null, null, null, null, null, null, null, null])
  })

  it('leaves the JSON-Schema URL behind', () => {
    const page = toBinderPage({
      $schema: 'https://example.com/schemas/PageBody.json',
      page: 0,
      pageCount: 1,
      slots: [null, null, null, null, null, null, null, null, null],
    })

    expect(Object.keys(page).sort()).toEqual(['page', 'pageCount', 'pockets'])
  })
})
