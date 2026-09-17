import * as Clipboard from 'expo-clipboard'
import { act, renderHook } from '@testing-library/react-native'
import { Linking } from 'react-native'

import type { ContactMethod } from './contactMethods'
import { useContactActions, type ContactActions } from './useContactActions'

const email: ContactMethod = {
  channel: 'email',
  value: 'seller@binder.test',
  url: 'mailto:seller@binder.test',
}

const phone: ContactMethod = {
  channel: 'phone',
  value: '+359 88 123 4567',
  url: 'tel:+359881234567',
}

const renderActions = async (): Promise<{ current: ContactActions }> => {
  const { result } = await renderHook(() => useContactActions())
  return result
}

describe('useContactActions', () => {
  beforeEach(() => {
    jest.spyOn(Linking, 'openURL').mockResolvedValue(true)
    jest.spyOn(Clipboard, 'setStringAsync').mockResolvedValue(true)
  })

  afterEach(() => {
    jest.restoreAllMocks()
  })

  it('has nothing to say before the buyer does anything', async () => {
    const actions = await renderActions()

    expect(actions.current.outcome).toBeNull()
  })

  it('opens an address in whatever app writes mail', async () => {
    const actions = await renderActions()

    await act(async () => {
      actions.current.open(email)
    })

    expect(Linking.openURL).toHaveBeenCalledWith('mailto:seller@binder.test')
    expect(actions.current.outcome).toBeNull()
  })

  it('opens a number in the dialler, without the spaces it is written with', async () => {
    const actions = await renderActions()

    await act(async () => {
      actions.current.open(phone)
    })

    expect(Linking.openURL).toHaveBeenCalledWith('tel:+359881234567')
  })

  it('says so when the phone has no app that can open the detail', async () => {
    jest.spyOn(Linking, 'openURL').mockRejectedValue(new Error('no handler'))
    const actions = await renderActions()

    await act(async () => {
      actions.current.open(email)
    })

    expect(actions.current.outcome).toEqual({ kind: 'openFailed', channel: 'email' })
  })

  // The number, not the URL: `tel:+359881234567` pasted into a contact card is
  // not a phone number anybody can call.
  it('copies what the seller published, not the URL it is opened with', async () => {
    const actions = await renderActions()

    await act(async () => {
      actions.current.copy(phone)
    })

    expect(Clipboard.setStringAsync).toHaveBeenCalledWith('+359 88 123 4567')
    expect(actions.current.outcome).toEqual({ kind: 'copied', channel: 'phone' })
  })

  it('says the copy did not land when the clipboard refuses it', async () => {
    jest.spyOn(Clipboard, 'setStringAsync').mockResolvedValue(false)
    const actions = await renderActions()

    await act(async () => {
      actions.current.copy(email)
    })

    expect(actions.current.outcome).toEqual({ kind: 'copyFailed', channel: 'email' })
  })

  it('says the copy did not land when the clipboard is not there at all', async () => {
    jest.spyOn(Clipboard, 'setStringAsync').mockRejectedValue(new Error('no clipboard'))
    const actions = await renderActions()

    await act(async () => {
      actions.current.copy(email)
    })

    expect(actions.current.outcome).toEqual({ kind: 'copyFailed', channel: 'email' })
  })

  it('drops what it said about the last action when the buyer tries another', async () => {
    const actions = await renderActions()
    await act(async () => {
      actions.current.copy(email)
    })

    await act(async () => {
      actions.current.open(phone)
    })

    expect(actions.current.outcome).toBeNull()
  })
})
