import { apiUrl } from '@/lib/apiUrl'

/**
 * The three shapes of request the binder endpoints answer with: a body, a body
 * from a write, and `204 No Content`.
 *
 * A failed status throws rather than being returned, so a caller cannot read a
 * problem document as a page. Every hook here turns that into its own error
 * state — none of them launders a failure into an empty binder
 * (003-frontend.md §11).
 *
 * **No `Authorization` header.** Every `/binders` endpoint takes an actor and
 * answers 401 without one, and this app has no sign-in: the spec's mobile wave
 * has no task for it (`specs/001-binder-mvp/tasks.md`, Wave 4). These requests
 * are correct apart from the credential, and the screens show the error state
 * until a session exists to attach. Adding a token store here would be guessing
 * at the shape of a wave that has not been designed.
 */
const failed = (method: string, path: string, status: number): Error =>
  new Error(`${method} ${path} answered ${status}`)

export const readJson = async <T>(path: string): Promise<T> => {
  const response = await fetch(apiUrl(path))
  if (!response.ok) throw failed('GET', path, response.status)

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
  if (!response.ok) throw failed(method, path, response.status)

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
  if (!response.ok) throw failed(method, path, response.status)
}
