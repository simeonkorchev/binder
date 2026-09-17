import { contactMethods } from './contactMethods'

describe('contactMethods', () => {
  // The completeness test (000-principles.md §9): every field of the source set
  // to a distinct value, every field of every destination asserted. A field
  // dropped here is a seller nobody can reach.
  it('carries every published detail across, with the way to open it', () => {
    expect(contactMethods({ email: 'seller@binder.test', phone: '+359 88 123 4567' })).toEqual([
      {
        channel: 'email',
        value: 'seller@binder.test',
        url: 'mailto:seller@binder.test',
      },
      {
        channel: 'phone',
        value: '+359 88 123 4567',
        url: 'tel:+359881234567',
      },
    ])
  })

  // This is the case the backend went out of its way to answer with a 200: a
  // seller who shared nothing is ordinary, not an error and not a 404.
  it('is empty for a seller who opted into sharing nothing', () => {
    expect(contactMethods({ email: null, phone: null })).toEqual([])
  })

  it('reveals only the address when that is all the seller shared', () => {
    expect(contactMethods({ email: 'seller@binder.test', phone: null })).toEqual([
      { channel: 'email', value: 'seller@binder.test', url: 'mailto:seller@binder.test' },
    ])
  })

  it('reveals only the number when that is all the seller shared', () => {
    expect(contactMethods({ email: null, phone: '0888123456' })).toEqual([
      { channel: 'phone', value: '0888123456', url: 'tel:0888123456' },
    ])
  })

  it('treats a blank field as nothing shared, because there is nothing in it to reach', () => {
    expect(contactMethods({ email: '   ', phone: '' })).toEqual([])
  })

  it('shows what was published but trims what it was published with', () => {
    expect(contactMethods({ email: ' seller@binder.test ', phone: null })).toEqual([
      { channel: 'email', value: 'seller@binder.test', url: 'mailto:seller@binder.test' },
    ])
  })
})
