import * as Clipboard from 'expo-clipboard'
import { useState } from 'react'
import { Linking } from 'react-native'

import type { ContactChannel, ContactMethod } from './contactMethods'

/** What the last action did, on which of the seller's details it did it. */
export interface ContactOutcome {
  kind: 'copied' | 'openFailed' | 'copyFailed'
  channel: ContactChannel
}

export interface ContactActions {
  /** Hands the detail to the phone's own mail app or dialler. */
  open: (method: ContactMethod) => void
  /** Puts the detail on the clipboard, which is the way out when nothing can open it. */
  copy: (method: ContactMethod) => void
  /** What to say about the last action, or nothing when it simply worked. */
  outcome: ContactOutcome | null
}

/**
 * The two things a buyer does with a seller's details, and what to say when one
 * of them does not work.
 *
 * Both exist on purpose. Opening is the fast path — a `mailto:` or a `tel:`
 * lands in the app that can act on it — but a phone with no mail account
 * configured cannot open a `mailto:` at all, and a number a buyer wants to save
 * is a number they would otherwise retype. A detail that can only be read off
 * the screen is a detail that gets retyped wrong.
 *
 * The clipboard gets the value the seller published, never the URL: nobody
 * wants `tel:` pasted into their contacts.
 */
export const useContactActions = (): ContactActions => {
  const [outcome, setOutcome] = useState<ContactOutcome | null>(null)

  return {
    open: (method: ContactMethod): void => {
      setOutcome(null)
      void Linking.openURL(method.url).then(
        () => undefined,
        () => setOutcome({ kind: 'openFailed', channel: method.channel }),
      )
    },

    copy: (method: ContactMethod): void => {
      setOutcome(null)
      void Clipboard.setStringAsync(method.value).then(
        (copied) =>
          setOutcome({ kind: copied ? 'copied' : 'copyFailed', channel: method.channel }),
        () => setOutcome({ kind: 'copyFailed', channel: method.channel }),
      )
    },

    outcome,
  }
}
