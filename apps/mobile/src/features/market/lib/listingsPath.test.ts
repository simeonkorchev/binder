import { isNarrowed, listingsPath, wholeMarket } from './listingsPath'

describe('listingsPath', () => {
  it('asks for the whole feed when nothing is filtered', () => {
    expect(listingsPath(wholeMarket)).toBe('/listings')
  })

  it('sends every part of a filter that was filled in', () => {
    expect(listingsPath({ query: 'Dark Magician', setCode: 'LOB' })).toBe(
      '/listings?q=Dark%20Magician&set=LOB',
    )
  })

  it('sends only the name when no set was named', () => {
    expect(listingsPath({ query: 'Kuriboh', setCode: '' })).toBe('/listings?q=Kuriboh')
  })

  it('sends only the set when no name was typed', () => {
    expect(listingsPath({ query: '', setCode: 'SDK' })).toBe('/listings?set=SDK')
  })

  it('leaves a part the user only put spaces in out of the request', () => {
    expect(listingsPath({ query: '   ', setCode: ' ' })).toBe('/listings')
  })

  it('trims what it does send, so a stray space is not part of the name', () => {
    expect(listingsPath({ query: '  Kuriboh ', setCode: ' LOB ' })).toBe(
      '/listings?q=Kuriboh&set=LOB',
    )
  })

  it('escapes a name that would otherwise be read as more query parameters', () => {
    expect(listingsPath({ query: 'a&set=X', setCode: '' })).toBe('/listings?q=a%26set%3DX')
  })
})

describe('isNarrowed', () => {
  it('is false for the whole market', () => {
    expect(isNarrowed(wholeMarket)).toBe(false)
  })

  it('is false when the user only typed spaces, which the request drops', () => {
    expect(isNarrowed({ query: '  ', setCode: '' })).toBe(false)
  })

  it('is true when a name was typed', () => {
    expect(isNarrowed({ query: 'Kuriboh', setCode: '' })).toBe(true)
  })

  it('is true when a set was chosen', () => {
    expect(isNarrowed({ query: '', setCode: 'LOB' })).toBe(true)
  })
})
