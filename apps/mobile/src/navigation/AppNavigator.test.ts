import type { RootStackParamList, RootTabParamList } from './AppNavigator'

// The param list is the contract between the navigator and the five screens
// W9–W11 land: a route's params are the whole input its screen gets. Renaming
// or dropping one here stops this file compiling, which is the point — the
// screens are not written yet, so nothing else would notice until they are.
describe('route params', () => {
  it('gives each pushed route the id its screen must load', () => {
    const binderPage: RootStackParamList['BinderPage'] = { binderId: 'b-1' }
    const sellerContact: RootStackParamList['SellerContact'] = { sellerId: 's-1' }

    expect([binderPage.binderId, sellerContact.sellerId]).toEqual(['b-1', 's-1'])
  })

  it('takes no params for the three top-level destinations', () => {
    const tabs: RootTabParamList = { Scan: undefined, Binders: undefined, Market: undefined }

    expect(Object.values(tabs).every((params) => params === undefined)).toBe(true)
  })
})
