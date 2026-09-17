import type { SellerContactBody } from '../types'

/** The kinds of contact detail a seller can publish. Nothing else is ever revealed. */
export type ContactChannel = 'email' | 'phone'

/** One way to reach a seller, ready to be shown, opened and copied. */
export interface ContactMethod {
  channel: ContactChannel
  /** Exactly what the seller published — what is shown, and what a copy puts on the clipboard. */
  value: string
  /** What the phone opens for it: `mailto:` for an address, `tel:` for a number. */
  url: string
}

/** A telephone URI has no spaces in it; the number is still shown the way it was published. */
const dialable = (phone: string): string => phone.replace(/\s+/g, '')

/**
 * Turns the seller's answer into the methods the screen offers.
 *
 * **An empty list is the normal answer**, not a failure: a seller who opted
 * into publishing neither field is a `200` with two nulls (D4), and the screen
 * says so in words. A blank string is treated the same as a null for the same
 * reason — there is nothing in it to write to or dial.
 *
 * Email comes before phone because that is the order the contract lists them
 * in, so the two cannot drift apart for no reason.
 *
 * Deliberately not carried across: nothing. `SellerContactBody` is those two
 * fields and the `$schema` URL huma adds to every body, which describes the
 * response rather than the seller.
 */
export const contactMethods = (contact: SellerContactBody): ContactMethod[] => {
  const methods: ContactMethod[] = []
  const email = contact.email?.trim() ?? ''
  const phone = contact.phone?.trim() ?? ''

  if (email !== '') methods.push({ channel: 'email', value: email, url: `mailto:${email}` })
  if (phone !== '') methods.push({ channel: 'phone', value: phone, url: `tel:${dialable(phone)}` })

  return methods
}
