import { apiUrl } from './apiUrl'
import { bearerToken, forgetSession } from './sessionStore'

/**
 * The three shapes of request this API answers with: a body, a body from a
 * write, and `204 No Content`.
 *
 * A failed status throws rather than being returned, so a caller cannot read a
 * problem document as a page. Every hook turns that into its own error state —
 * none of them launders a failure into an empty binder or an empty market
 * (003-frontend.md §11).
 *
 * It lives in `lib/` rather than under one feature because the features share
 * one HTTP: the binder's slots, the marketplace's listings and the sign-in
 * exchange all speak it through here, and a second copy of it is a second place
 * a header can be wrong (000-principles.md §6). The scanner's two requests are
 * the one exception, and are not meant to be: they build their own `fetch` over
 * `apiUrl` and so send no credential at all — harmless only for as long as the
 * card domain takes no actor. The finding that proposes the fix is
 * `.ai/findings/open/2026-09-18-scan-api-calls-bypass-the-request-helper.md`.
 *
 * **The bearer token is attached here and nowhere else.** No hook passes one in
 * and no component holds one: they all go through these three functions, which
 * read the session from `sessionStore`. Signed out the header is simply absent,
 * which is a correct request for the endpoints that need no actor — `GET
 * /listings`, the browse feed, and the card domain's `GET /cards` and
 * `POST /scans/resolve`, which `cmd/binderd/routes.go` registers with no
 * `ActorFunc` — and a 401 from every one that does.
 */

/**
 * A request that came back with a status the caller did not want.
 *
 * It carries the status because some of them mean something specific to one
 * caller: `POST /listings` answers 409 when the card is already for sale, and a
 * 401 means the session is gone rather than the request being wrong. The screens
 * say those as different sentences, and reading them off the message text would
 * be parsing English.
 *
 * The message names the method, the path and the status — never a token, a body,
 * or anything else the request carried (004-security.md).
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
  const response = await fetch(apiUrl(path), { headers: headers(false) })
  if (!response.ok) await refuse('GET', path, response)

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
    headers: headers(true),
    body: JSON.stringify(body),
  })
  if (!response.ok) await refuse(method, path, response)

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
    headers: headers(body !== undefined),
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (!response.ok) await refuse(method, path, response)
}

/** The credential when there is one, and a content type when there is a body. */
const headers = (hasBody: boolean): Record<string, string> => {
  const token = bearerToken()

  return {
    ...(hasBody ? { 'Content-Type': 'application/json' } : {}),
    // Absent rather than empty: `Bearer ` with nothing after it is a malformed
    // credential, and the one endpoint that needs none would start refusing it.
    ...(token === null ? {} : { Authorization: `Bearer ${token}` }),
  }
}

/**
 * Turns a refused response into the error the caller sees, and takes the session
 * away first when the server says the credential is no good.
 *
 * A 401 is the only status this helper acts on. The session JWT lasts 24 hours
 * and cannot be refreshed, so a token the server has stopped accepting has to
 * go: keeping it would put a dead credential on every later request, and each
 * screen would show its own "could not be loaded" for what is really "sign in
 * again". Dropping it publishes `signed-out`, and the navigator renders the
 * sign-in screen from that state — no screen has to navigate anywhere (T082).
 */
const refuse = async (method: string, path: string, response: Response): Promise<never> => {
  if (response.status === 401) await forgetSession()

  throw new ApiError(method, path, response.status)
}
