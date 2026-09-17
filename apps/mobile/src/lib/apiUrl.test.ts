import { apiUrl } from './apiUrl'

describe('apiUrl', () => {
  afterEach(() => {
    delete process.env.EXPO_PUBLIC_API_URL
  })

  it('joins the path onto the configured base URL', () => {
    process.env.EXPO_PUBLIC_API_URL = 'https://api.binder.test'

    expect(apiUrl('/binders')).toBe('https://api.binder.test/binders')
  })

  it('refuses to build a URL when the base is unset', () => {
    expect(() => apiUrl('/binders')).toThrow('EXPO_PUBLIC_API_URL is not set')
  })

  it('refuses to build a URL when the base is empty', () => {
    process.env.EXPO_PUBLIC_API_URL = ''

    expect(() => apiUrl('/binders')).toThrow('EXPO_PUBLIC_API_URL is not set')
  })
})
