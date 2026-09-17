import { apiUrl } from './apiUrl'

/**
 * The three shapes of request this API answers with: a body, a body from a
 * write, and `204 No Content`.
 *
 * A failed status throws rather than being returned, so a caller cannot read a
 * problem document as a page. Every hook turns that into its own error state —
 * none of them launders a failure into an empty binder or an empty market
 * (003-frontend.md §11).
 *
 * It lives in `lib/` rather than under one feature because two features now
 * call it: the binder's slots and the marketplace's listings speak the same
 * HTTP, and a second copy of it is a second place a header can be wrong
 * (000-principles.md §6).
 *
 * **No `Authorization` header.** Every endpoint but the browse feed takes an
 * actor and answers 401 without one, and this app has no sign-in: the spec's
 * mobile wave has no task for it (`specs/001-binder-mvp/tasks.md`, Wave 4).
 * These requests are correct apart from the credential, and the screens show
 * the error state until a session exists to attach. Adding a token store here
 * would be guessing at the shape of a wave that has not been designed.
 */

/**
 * A request that came back with a status the caller did not want.
 *
 * It carries the status because some of them mean something specific to one
 * caller: `POST /listings` answers 409 when the card is already for sale, which
 * the sheet says as a sentence rather than as a generic failure. Reading that
 * off the message text would be parsing English.
 */
export class ApiError extends Error {
  readonly status: number

  constructor(method: string, path: string, status: number) {
    super(`${method} ${path} answered ${status}`)
    this.name = 'ApiError'
    this.status = status
  }
}

export const readJson = async <T>(path: string): Promise<T> => {
  const response = await fetch(apiUrl(path))
  if (!response.ok) throw new ApiError('GET', path, response.status)

  const body: T = await response.json()
  return body
}

export const writeJson = async <T>(
  method: 'POST' | 'PATCH',
  path: string,
  body: object,
): Promise<T> => {
  const response = await fetch(apiUrl(path), {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!response.ok) throw new ApiError(method, path, response.status)

  const answered: T = await response.json()
  return answered
}

export const writeEmpty = async (
  method: 'PATCH' | 'DELETE',
  path: string,
  body?: object,
): Promise<void> => {
  const response = await fetch(apiUrl(path), {
    method,
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (!response.ok) throw new ApiError(method, path, response.status)
}
