import type { BinderPage, PageBody, SlotBody } from '../types'
import { pocketsPerPage } from '../lib/positions'

/**
 * Shapes one `GET /binders/{binderId}` response into the page the grid draws.
 *
 * The one thing it does beyond carrying the fields across is guarantee the
 * length: the grid indexes nine pockets whatever the response held, so a body
 * that ever carried fewer draws empty pockets rather than `undefined` ones.
 * `noUncheckedIndexedAccess` makes that a compile error at every call site
 * otherwise, which is the same fact stated less usefully.
 *
 * Deliberately not carried: `$schema`, the JSON-Schema URL huma adds to every
 * body, which describes the response rather than the page.
 */
export const toBinderPage = (body: PageBody): BinderPage => {
  const pockets: (SlotBody | null)[] = []
  for (let pocket = 0; pocket < pocketsPerPage; pocket += 1) {
    pockets.push(body.slots[pocket] ?? null)
  }

  return { page: body.page, pageCount: body.pageCount, pockets }
}
