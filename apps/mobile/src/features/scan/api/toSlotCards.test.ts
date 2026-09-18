import type { SlotCardBody } from '@/features/binder/types'

import type { RejectedScan, ResolvedScan, ScannedCard, ScannedPrinting } from '../types'
import type { ReviewDecision, ReviewRow } from '../useScanReview'

import { toSlotCards } from './toSlotCards'

const blueEyes: ScannedCard = {
  id: 'blue-eyes',
  name: 'Blue-Eyes White Dragon',
  imageObjectKey: 'cards/blue-eyes.jpg',
}

const darkMagician: ScannedCard = {
  id: 'dark-magician',
  name: 'Dark Magician',
  imageObjectKey: 'cards/dark-magician.jpg',
}

const lob: ScannedPrinting = {
  id: 'lob-001',
  cardId: 'blue-eyes',
  setCode: 'LOB-001',
  rarity: 'Ultra Rare',
}

const sdk: ScannedPrinting = {
  id: 'sdk-001',
  cardId: 'blue-eyes',
  setCode: 'SDK-001',
  rarity: 'Common',
}

/**
 * A scan the ladder settled, with **every** field of the answer carrying a
 * distinct value — the mapper has to read four of them and drop two, and a test
 * built from a half-filled scan would not notice it reading the wrong one.
 */
const settled = (over: Partial<ResolvedScan> = {}): ResolvedScan => ({
  code: 'LOB-001',
  resolution: 'exact',
  outcome: 'resolved',
  card: blueEyes,
  printing: lob,
  candidates: [{ card: blueEyes, printing: lob }],
  ...over,
})

/** A scan the ladder flagged: the review sheet is the only thing that can settle it. */
const flagged = (over: Partial<ResolvedScan> = {}): ResolvedScan => ({
  code: 'SMUDGE-1',
  resolution: 'unresolved',
  outcome: 'ambiguous',
  card: blueEyes,
  printing: null,
  candidates: [
    { card: blueEyes, printing: lob },
    { card: blueEyes, printing: sdk },
  ],
  ...over,
})

const row = (code: string, decision: ReviewDecision | null): ReviewRow => ({
  code,
  reason: 'ambiguous',
  copies: 1,
  card: blueEyes,
  candidates: [],
  decision,
})

const refused = (code: string): RejectedScan => ({ code, status: 422 })

describe('toSlotCards', () => {
  it('files a card the ladder settled on the rung the ladder reached it on', () => {
    const cards = toSlotCards({ resolved: [settled()], rejected: [], reviewed: [] })

    expect(cards).toEqual([
      { cardId: 'blue-eyes', cardPrintingId: 'lob-001', setResolution: 'exact' },
    ] satisfies SlotCardBody[])
  })

  it.each([
    { resolution: 'exact' },
    { resolution: 'by_prefix_and_number' },
    { resolution: 'by_number' },
  ] as const)('keeps the $resolution rung as it was answered', ({ resolution }) => {
    const cards = toSlotCards({
      resolved: [settled({ resolution })],
      rejected: [],
      reviewed: [],
    })

    expect(cards).toEqual([
      { cardId: 'blue-eyes', cardPrintingId: 'lob-001', setResolution: resolution },
    ] satisfies SlotCardBody[])
  })

  it('files a printing the user picked as manual, never as the rung the scan came in on', () => {
    const cards = toSlotCards({
      resolved: [flagged()],
      rejected: [],
      reviewed: [row('SMUDGE-1', { kind: 'kept', card: blueEyes, printing: sdk })],
    })

    expect(cards).toEqual([
      { cardId: 'blue-eyes', cardPrintingId: 'sdk-001', setResolution: 'manual' },
    ] satisfies SlotCardBody[])
  })

  it('files a card kept with its set left open as by_name, with no printing', () => {
    const cards = toSlotCards({
      resolved: [flagged()],
      rejected: [],
      reviewed: [row('SMUDGE-1', { kind: 'kept', card: blueEyes, printing: null })],
    })

    expect(cards).toEqual([
      { cardId: 'blue-eyes', cardPrintingId: null, setResolution: 'by_name' },
    ] satisfies SlotCardBody[])
  })

  it('files the card a name search found, not the card the scan guessed at', () => {
    const cards = toSlotCards({
      resolved: [flagged()],
      rejected: [],
      reviewed: [row('SMUDGE-1', { kind: 'kept', card: darkMagician, printing: null })],
    })

    expect(cards).toEqual([
      { cardId: 'dark-magician', cardPrintingId: null, setResolution: 'by_name' },
    ] satisfies SlotCardBody[])
  })

  it('sends nothing for a card the user left out', () => {
    const cards = toSlotCards({
      resolved: [flagged()],
      rejected: [],
      reviewed: [row('SMUDGE-1', { kind: 'discarded' })],
    })

    expect(cards).toEqual([])
  })

  it('sends nothing for a flagged card the user never answered', () => {
    const cards = toSlotCards({
      resolved: [flagged()],
      rejected: [],
      reviewed: [row('SMUDGE-1', null)],
    })

    expect(cards).toEqual([])
  })

  it('is an empty commit when every card in the sweep was left out', () => {
    const cards = toSlotCards({
      resolved: [flagged(), flagged({ code: 'SMUDGE-2' })],
      rejected: [refused('!!!')],
      reviewed: [
        row('SMUDGE-1', { kind: 'discarded' }),
        row('SMUDGE-2', { kind: 'discarded' }),
        row('!!!', { kind: 'discarded' }),
      ],
    })

    expect(cards).toEqual([])
  })

  it('files a refused scan the user identified by name', () => {
    const cards = toSlotCards({
      resolved: [],
      rejected: [refused('!!!')],
      reviewed: [row('!!!', { kind: 'kept', card: darkMagician, printing: null })],
    })

    expect(cards).toEqual([
      { cardId: 'dark-magician', cardPrintingId: null, setResolution: 'by_name' },
    ] satisfies SlotCardBody[])
  })

  it('does not file a refusal the resolver later answered about, which is one card', () => {
    const cards = toSlotCards({
      resolved: [settled()],
      rejected: [refused('LOB-001')],
      reviewed: [],
    })

    expect(cards).toEqual([
      { cardId: 'blue-eyes', cardPrintingId: 'lob-001', setResolution: 'exact' },
    ] satisfies SlotCardBody[])
  })

  it('expands one decision back out to every copy the sweep captured', () => {
    const cards = toSlotCards({
      resolved: [flagged(), flagged()],
      rejected: [],
      reviewed: [row('SMUDGE-1', { kind: 'kept', card: blueEyes, printing: sdk })],
    })

    expect(cards).toEqual([
      { cardId: 'blue-eyes', cardPrintingId: 'sdk-001', setResolution: 'manual' },
      { cardId: 'blue-eyes', cardPrintingId: 'sdk-001', setResolution: 'manual' },
    ] satisfies SlotCardBody[])
  })

  it('keeps the sweep in the order the cards were swept, decided and settled alike', () => {
    const cards = toSlotCards({
      resolved: [settled(), flagged(), settled({ code: 'SDK-001', printing: sdk })],
      rejected: [refused('!!!')],
      reviewed: [
        row('SMUDGE-1', { kind: 'kept', card: blueEyes, printing: lob }),
        row('!!!', { kind: 'kept', card: darkMagician, printing: null }),
      ],
    })

    expect(cards).toEqual([
      { cardId: 'blue-eyes', cardPrintingId: 'lob-001', setResolution: 'exact' },
      { cardId: 'blue-eyes', cardPrintingId: 'lob-001', setResolution: 'manual' },
      { cardId: 'blue-eyes', cardPrintingId: 'sdk-001', setResolution: 'exact' },
      { cardId: 'dark-magician', cardPrintingId: null, setResolution: 'by_name' },
    ] satisfies SlotCardBody[])
  })

  describe('an answer whose shape the contract rules out', () => {
    it('leaves out a settled scan that names no card, having nothing to file', () => {
      const cards = toSlotCards({
        resolved: [settled({ card: null })],
        rejected: [],
        reviewed: [],
      })

      expect(cards).toEqual([])
    })

    // A printing-bearing rung with no printing is the pair the slot's CHECK
    // rejects, and one rejected row would roll the whole sweep back.
    it('leaves out a rung that promises a printing and carries none', () => {
      const cards = toSlotCards({
        resolved: [settled({ printing: null })],
        rejected: [],
        reviewed: [],
      })

      expect(cards).toEqual([])
    })

    it('files an unflagged scan that determined no set as the card it named', () => {
      const cards = toSlotCards({
        resolved: [settled({ resolution: 'by_name', printing: null })],
        rejected: [],
        reviewed: [],
      })

      expect(cards).toEqual([
        { cardId: 'blue-eyes', cardPrintingId: null, setResolution: 'by_name' },
      ] satisfies SlotCardBody[])
    })

    it('files an unflagged unresolved scan as the card it named, dropping the printing', () => {
      const cards = toSlotCards({
        resolved: [settled({ resolution: 'unresolved' })],
        rejected: [],
        reviewed: [],
      })

      expect(cards).toEqual([
        { cardId: 'blue-eyes', cardPrintingId: null, setResolution: 'by_name' },
      ] satisfies SlotCardBody[])
    })
  })
})
