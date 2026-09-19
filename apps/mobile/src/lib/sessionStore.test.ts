import { waitFor } from '@testing-library/react-native'
import * as SecureStore from 'expo-secure-store'

import {
  bearerToken,
  forgetSession,
  rememberSession,
  restoreSession,
  sessionState,
  subscribeToSession,
} from './sessionStore'
import { serializeSession, type Session } from './storedSession'

const session: Session = {
  token: 'header.payload.signature',
  expiresAt: '2026-09-18T10:00:00.000Z',
  userId: 'f1f0c0de-0000-4000-8000-000000000001',
}

/** Inside the 24h the backend issues for, read against the session above. */
const withinTheDay = new Date('2026-09-18T09:00:00.000Z')

describe('sessionStore', () => {
  beforeEach(async () => {
    jest.useFakeTimers({ doNotFake: ['nextTick', 'queueMicrotask'] })
    jest.setSystemTime(withinTheDay)
    await forgetSession()
  })

  afterEach(() => {
    jest.useRealTimers()
  })

  it('keeps a session in the keychain, so a later launch can read it back', async () => {
    await rememberSession(session)

    expect(sessionState()).toEqual({ status: 'signed-in', session })
    await expect(SecureStore.getItemAsync('binder.session')).resolves.toBe(
      serializeSession(session),
    )
  })

  it('attaches nothing while nobody is signed in — browse is public', () => {
    expect(bearerToken()).toBeNull()
  })

  it('offers the stored token once a session exists', async () => {
    await rememberSession(session)

    expect(bearerToken()).toBe(session.token)
  })

  it('restores a stored session on launch', async () => {
    await rememberSession(session)
    await forgetInMemoryOnly()

    restoreSession()

    expect(sessionState()).toEqual({ status: 'signed-in', session })
  })

  it('finds no session on a first launch', () => {
    restoreSession()

    expect(sessionState()).toEqual({ status: 'signed-out' })
  })

  it('drops a session the server would no longer accept, and clears the keychain', async () => {
    await rememberSession(session)
    jest.setSystemTime(new Date('2026-09-19T10:00:00.000Z'))

    restoreSession()

    expect(sessionState()).toEqual({ status: 'signed-out' })
    await waitFor(async () => {
      await expect(SecureStore.getItemAsync('binder.session')).resolves.toBeNull()
    })
  })

  // `devSession` has its own suite for what it accepts; these two pin what the
  // store does with one — it wins over the keychain, and it stays out of it.
  describe('when the build supplies a token', () => {
    const supplied = [
      'eyJhbGciOiJIUzI1NiJ9',
      btoa(JSON.stringify({ sub: session.userId, exp: Date.parse(session.expiresAt) / 1000 }))
        .replaceAll('+', '-')
        .replaceAll('/', '_')
        .replaceAll('=', ''),
      'signature',
    ].join('.')

    afterEach(() => {
      delete process.env.EXPO_PUBLIC_DEV_SESSION_TOKEN
    })

    it('signs in with it in preference to whatever the keychain holds', async () => {
      await rememberSession({ ...session, token: 'the-keychain-one' })
      process.env.EXPO_PUBLIC_DEV_SESSION_TOKEN = supplied

      restoreSession()

      expect(bearerToken()).toBe(supplied)
    })

    it('does not write it to the keychain, so removing the variable is enough to undo it', async () => {
      process.env.EXPO_PUBLIC_DEV_SESSION_TOKEN = supplied

      restoreSession()

      expect(bearerToken()).toBe(supplied)
      await expect(SecureStore.getItemAsync('binder.session')).resolves.toBeNull()
    })
  })

  it('clears the keychain on sign-out, so the token cannot be restored', async () => {
    await rememberSession(session)

    await forgetSession()

    expect(sessionState()).toEqual({ status: 'signed-out' })
    expect(bearerToken()).toBeNull()
    await expect(SecureStore.getItemAsync('binder.session')).resolves.toBeNull()
  })

  it('tells its subscribers when the session appears and when it goes', async () => {
    const listener = jest.fn()
    const unsubscribe = subscribeToSession(listener)

    await rememberSession(session)
    await forgetSession()
    unsubscribe()
    await rememberSession(session)

    expect(listener).toHaveBeenCalledTimes(2)
  })

  it('does not wake its subscribers when a restore finds the signed-out state again', () => {
    restoreSession()
    const listener = jest.fn()
    subscribeToSession(listener)

    restoreSession()

    expect(listener).not.toHaveBeenCalled()
  })

  it('stays signed in when the keychain refuses the write', async () => {
    jest
      .spyOn(SecureStore, 'setItemAsync')
      .mockRejectedValueOnce(new Error('the keychain is locked'))

    await rememberSession(session)

    expect(sessionState()).toEqual({ status: 'signed-in', session })
  })

  it('signs out even when the keychain refuses the delete', async () => {
    await rememberSession(session)
    jest
      .spyOn(SecureStore, 'deleteItemAsync')
      .mockRejectedValueOnce(new Error('the keychain is locked'))

    await forgetSession()

    expect(sessionState()).toEqual({ status: 'signed-out' })
  })

  it('finds no session when the keychain cannot be read', async () => {
    await rememberSession(session)
    jest.spyOn(SecureStore, 'getItem').mockImplementationOnce(() => {
      throw new Error('the keychain is locked')
    })

    restoreSession()

    expect(sessionState()).toEqual({ status: 'signed-out' })
  })
})

/**
 * A relaunch: the keychain keeps what it has and the in-memory state forgets it,
 * which is the only state a cold start can be in.
 */
const forgetInMemoryOnly = async (): Promise<void> => {
  const stored = await SecureStore.getItemAsync('binder.session')
  await forgetSession()
  if (stored !== null) await SecureStore.setItemAsync('binder.session', stored)
}
